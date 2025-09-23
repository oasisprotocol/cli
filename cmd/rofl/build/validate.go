package build

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/net/idna"

	compose "github.com/compose-spec/compose-go/v2/cli"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
)

// ValidationOpts represents options for the validation process.
type ValidationOpts struct {
	// Offline indicates whether the validation should be performed without network access.
	Offline bool
}

// AppExtraConfig represents extra configuration for the ROFL app.
type AppExtraConfig struct {
	// Ports are the port mappings exposed by the app.
	Ports []*PortMapping
}

// PortMapping represents a port mapping.
type PortMapping struct {
	// ServiceName is the name of the service.
	ServiceName string
	// Port is the port number.
	Port string
	// ProxyMode is the proxy mode for the port.
	ProxyMode string
	// GenericDomain is the generic domain name.
	GenericDomain string
	// CustomDomain is the custom domain name (if any).
	CustomDomain string
}

// ValidateApp validates the ROFL app manifest.
func ValidateApp(manifest *buildRofl.Manifest, opts ValidationOpts) (*AppExtraConfig, error) {
	switch manifest.TEE {
	case buildRofl.TEETypeSGX:
		if manifest.Kind != buildRofl.AppKindRaw {
			return nil, fmt.Errorf("unsupported app kind for SGX TEE: %s", manifest.Kind)
		}
	case buildRofl.TEETypeTDX:
		switch manifest.Kind {
		case buildRofl.AppKindRaw:
		case buildRofl.AppKindContainer:
			wantedArtifacts := tdxWantedArtifacts(manifest, buildRofl.LatestContainerArtifacts)

			var composeAf *artifact
			for _, a := range wantedArtifacts {
				if a.kind == artifactContainerCompose {
					composeAf = a
					break
				}
			}
			if composeAf == nil {
				return nil, fmt.Errorf("missing compose.yaml artifact")
			}

			// Only fetch the compose.yaml artifact.
			artifacts := tdxFetchArtifacts([]*artifact{composeAf})

			// Validate compose.yaml.
			appCfg, err := validateComposeFile(artifacts[artifactContainerCompose], manifest, opts)
			if err != nil {
				return nil, fmt.Errorf("compose file validation failed: %w", err)
			}
			return appCfg, nil
		default:
			return nil, fmt.Errorf("unsupported app kind for TDX TEE: %s", manifest.Kind)
		}
	default:
		return nil, fmt.Errorf("unsupported TEE kind: %s", manifest.TEE)
	}

	return &AppExtraConfig{}, nil
}

// validateComposeFile validates the Docker compose file.
func validateComposeFile(composeFile string, manifest *buildRofl.Manifest, opts ValidationOpts) (*AppExtraConfig, error) { //nolint: gocyclo
	// Parse the compose file.
	options, err := compose.NewProjectOptions([]string{composeFile}, compose.WithInterpolation(false))
	if err != nil {
		return nil, fmt.Errorf("failed to set-up compose options: %w", err)
	}
	proj, err := options.LoadProject(context.Background())
	if err != nil {
		return nil, fmt.Errorf("parsing: %w", err)
	}

	// Keep track of all images encountered, as we will need them in later steps.
	images := []string{}
	customDomains := make(map[string]struct{})

	var appCfg AppExtraConfig
	for serviceName, service := range proj.Services {
		image := service.Image
		images = append(images, image)

		// Make sure that the image fields for all services contain a FQDN
		// or it will cause Podman errors when trying to run it.
		validationFailedErr := fmt.Errorf("image '%s' of service '%s' is not a fully-qualified domain name", image, serviceName)

		if !strings.Contains(image, "/") {
			return nil, validationFailedErr
		}
		s := strings.Split(image, "/")
		if len(s[0]) == 0 || len(s[1]) == 0 {
			return nil, validationFailedErr
		}

		domain := s[0]
		if !strings.Contains(domain, ".") {
			return nil, validationFailedErr
		}

		_, err := idna.Lookup.ToASCII(domain)
		if err != nil {
			return nil, validationFailedErr
		}

		// Also, if any volumes are set, make sure that the source of each volume
		// is either /run/rofl-appd.sock or starts with /storage/ (unless defined in the
		// top-level volumes config).
		volumeSourceIsValid := func(src string) bool {
			if src == "/run/rofl-appd.sock" {
				return true
			}
			if src == "/run/podman.sock" {
				return true
			}

			if strings.HasPrefix(src, "/storage/") {
				return true
			}

			return false
		}

		for _, vol := range service.Volumes {
			// Short compose syntax for volumes.
			if volumeSourceIsValid(vol.Source) {
				continue
			}

			// Implicit declaration of volumes.
			if v, ok := proj.Volumes[vol.Source]; ok {
				// If the volume is external, it should be one of the valid locations.
				if bool(!v.External) || volumeSourceIsValid(v.Name) {
					continue
				}
			}

			return nil, fmt.Errorf("volume '%s:%s' of service '%s' has an invalid external source (should be '/run/rofl-appd.sock', '/run/podman.sock' or reside inside '/storage/')", vol.Source, vol.Target, serviceName)
		}

		// Validate ports.
		publishedPorts := make(map[uint64]struct{})
		for _, port := range service.Ports {
			if port.Target == 0 {
				return nil, fmt.Errorf("service '%s' has an invalid zero port defined", serviceName)
			}
			if port.Published == "" || port.Published == "0" {
				return nil, fmt.Errorf("port '%d' of service '%s' does not have an explicit published port defined", port.Target, serviceName)
			}
			publishedPort, err := strconv.ParseUint(port.Published, 10, 16)
			if err != nil {
				return nil, fmt.Errorf("published port '%s' of service '%s' is invalid: %w", port.Published, serviceName, err)
			}
			publishedPorts[publishedPort] = struct{}{}

			if port.Protocol != "tcp" {
				continue
			}
			proxyMode := service.Annotations[fmt.Sprintf("net.oasis.proxy.ports.%s.mode", port.Published)]
			if proxyMode == "ignore" {
				continue
			}
			genericDomain := fmt.Sprintf("p%s", port.Published)
			customDomain := service.Annotations[fmt.Sprintf("net.oasis.proxy.ports.%s.custom_domain", port.Published)]
			if customDomain != "" {
				if _, ok := customDomains[customDomain]; ok {
					return nil, fmt.Errorf("custom domain '%s' of service '%s' is already in use", customDomain, serviceName)
				}
				customDomains[customDomain] = struct{}{}
			}
			appCfg.Ports = append(appCfg.Ports, &PortMapping{
				ServiceName:   serviceName,
				Port:          port.Published,
				ProxyMode:     proxyMode,
				GenericDomain: genericDomain,
				CustomDomain:  customDomain,
			})
		}

		// Validate annotations.
		for key, value := range service.Annotations {
			keyAtoms := strings.Split(key, ".")
			switch {
			case strings.HasPrefix(key, "net.oasis.proxy.ports"):
				port, err := strconv.ParseUint(keyAtoms[4], 10, 16)
				if err != nil {
					return nil, fmt.Errorf("proxy port annotation of service '%s' has an invalid port: %s", serviceName, keyAtoms[4])
				}
				if _, ok := publishedPorts[port]; !ok {
					return nil, fmt.Errorf("proxy port annotation of service '%s' references an unpublished port '%d'", serviceName, port)
				}

				// Validate supported annotations.
				switch keyAtoms[5] {
				case "mode":
					// Validate supported modes.
					switch value {
					case "ignore":
					case "passthrough":
					case "terminate-tls":
					default:
						return nil, fmt.Errorf("proxy port annotation of service '%s', port '%d' has an invalid mode: %s", serviceName, port, value)
					}
				case "custom_domain":
				default:
				}
			default:
			}
		}
	}

	// Make sure that we have images.
	if len(images) == 0 {
		return nil, fmt.Errorf("no images defined")
	}

	// Make sure that the total size of images fits into the storage size
	// and perform other minor validations.
	//
	// Note: This checks the compressed image size only, because that is the only thing
	// available in the OCI manifest.  To get the uncompressed size, we would need to
	// download all the layers and decompress them locally.
	maxSize := manifest.Resources.Storage.Size * 1024 * 1024
	var totalSize uint64
	for _, imageFull := range images {
		image := imageFull
		tag := "latest"

		// Extract tag if present.
		digestBits := strings.Split(imageFull, "@")
		if len(digestBits) == 2 {
			// image@sha256:...
			image = digestBits[0]
			tag = digestBits[1]
		} else {
			// image:tag
			bits := strings.Split(imageFull, ":")
			if len(bits) > 1 {
				image = strings.Join(bits[:len(bits)-1], ":")
				tag = bits[len(bits)-1]
			}
		}

		if opts.Offline {
			continue
		}

		// Fetch manifest from OCI repository.
		repo, err := remote.NewRepository(image)
		if err != nil {
			return nil, err
		}
		mainDescriptor, mfRaw, err := oras.FetchBytes(context.Background(), repo, tag, oras.DefaultFetchBytesOptions)
		if err != nil {
			return nil, fmt.Errorf("unable to fetch manifest for image '%s': %w", imageFull, err)
		}
		var mf ocispec.Manifest
		if err = json.Unmarshal(mfRaw, &mf); err != nil {
			return nil, fmt.Errorf("unable to parse manifest for image '%s': %w", imageFull, err)
		}

		// Validate platform if given.
		for _, platform := range []*ocispec.Platform{mainDescriptor.Platform, mf.Config.Platform} {
			if platform != nil {
				if platform.Architecture != "amd64" || platform.OS != "linux" {
					return nil, fmt.Errorf("image '%s' has incorrect platform (expected linux/amd64, got %s/%s)", imageFull, platform.OS, platform.Architecture)
				}
			}
		}

		// Add sizes for all layers.
		var imageSize uint64
		for _, layer := range mf.Layers {
			if layer.Size > 0 {
				imageSize += uint64(layer.Size)
			}
		}
		totalSize += imageSize
	}

	// Since we calculate the compressed size only, multiply by a fudge factor to bring
	// it closer to the actual size.
	totalSize *= 2

	// We could terminate early above, but it's more useful to give the user an estimate
	// of how big the storage should be.
	if totalSize > maxSize {
		return nil, fmt.Errorf("estimated total size of images (%d MB) exceeds storage size set in ROFL manifest (%d MB)", totalSize/1024/1024, manifest.Resources.Storage.Size)
	}

	return &appCfg, nil
}
