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

// validateApp validates the ROFL app manifest.
func validateApp(manifest *buildRofl.Manifest) error {
	switch manifest.TEE {
	case buildRofl.TEETypeSGX:
		if manifest.Kind != buildRofl.AppKindRaw {
			return fmt.Errorf("unsupported app kind for SGX TEE: %s", manifest.Kind)
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
				return fmt.Errorf("missing compose.yaml artifact")
			}

			// Only fetch the compose.yaml artifact.
			artifacts := tdxFetchArtifacts([]*artifact{composeAf})

			// Validate compose.yaml.
			err := validateComposeFile(artifacts[artifactContainerCompose], manifest)
			if err != nil {
				return fmt.Errorf("compose file validation failed: %w", err)
			}
		default:
			return fmt.Errorf("unsupported app kind for TDX TEE: %s", manifest.Kind)
		}
	default:
		return fmt.Errorf("unsupported TEE kind: %s", manifest.TEE)
	}

	return nil
}

// validateComposeFile validates the Docker compose file.
func validateComposeFile(composeFile string, manifest *buildRofl.Manifest) error { //nolint: gocyclo
	// Parse the compose file.
	options, err := compose.NewProjectOptions([]string{composeFile}, compose.WithInterpolation(false))
	if err != nil {
		return fmt.Errorf("failed to set-up compose options: %w", err)
	}
	proj, err := options.LoadProject(context.Background())
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("parsing failed")
	}

	// Keep track of all images encountered, as we will need them in later steps.
	images := []string{}

	for serviceName, service := range proj.Services {
		image := service.Image
		images = append(images, image)

		// Make sure that the image fields for all services contain a FQDN
		// or it will cause Podman errors when trying to run it.
		validationFailedErr := fmt.Errorf("image '%s' of service '%s' is not a fully-qualified domain name", image, serviceName)

		if !strings.Contains(image, "/") {
			return validationFailedErr
		}
		s := strings.Split(image, "/")
		if len(s[0]) == 0 || len(s[1]) == 0 {
			return validationFailedErr
		}

		domain := s[0]
		if !strings.Contains(domain, ".") {
			return validationFailedErr
		}

		_, err := idna.Lookup.ToASCII(domain)
		if err != nil {
			return validationFailedErr
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

			return fmt.Errorf("volume '%s:%s' of service '%s' has an invalid external source (should be '/run/rofl-appd.sock', '/run/podman.sock' or reside inside '/storage/')", vol.Source, vol.Target, serviceName)
		}

		// Validate ports.
		publishedPorts := make(map[uint64]struct{})
		for _, port := range service.Ports {
			if port.Target == 0 {
				return fmt.Errorf("service '%s' has an invalid zero port defined", serviceName)
			}
			if port.Published == "" || port.Published == "0" {
				return fmt.Errorf("port '%d' of service '%s' does not have an explicit published port defined", port.Target, serviceName)
			}
			publishedPort, err := strconv.ParseUint(port.Published, 10, 16)
			if err != nil {
				return fmt.Errorf("published port '%s' of service '%s' is invalid: %w", port.Published, serviceName, err)
			}
			publishedPorts[publishedPort] = struct{}{}
		}

		// Validate annotations.
		for key, value := range service.Annotations {
			keyAtoms := strings.Split(key, ".")
			switch {
			case strings.HasPrefix(key, "net.oasis.proxy.ports"):
				port, err := strconv.ParseUint(keyAtoms[4], 10, 16)
				if err != nil {
					return fmt.Errorf("proxy port annotation of service '%s' has an invalid port: %s", serviceName, keyAtoms[4])
				}
				if _, ok := publishedPorts[port]; !ok {
					return fmt.Errorf("proxy port annotation of service '%s' references an unpublished port '%d'", serviceName, port)
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
						return fmt.Errorf("proxy port annotation of service '%s', port '%d' has an invalid mode: %s", serviceName, port, value)
					}
				default:
				}
			default:
			}
		}
	}

	// Make sure that we have images.
	if len(images) == 0 {
		return fmt.Errorf("no images defined")
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

		// Fetch manifest from OCI repository.
		repo, err := remote.NewRepository(image)
		if err != nil {
			return err
		}
		mainDescriptor, mfRaw, err := oras.FetchBytes(context.Background(), repo, tag, oras.DefaultFetchBytesOptions)
		if err != nil {
			return fmt.Errorf("unable to fetch manifest for image '%s': %w", imageFull, err)
		}
		var mf ocispec.Manifest
		if err = json.Unmarshal(mfRaw, &mf); err != nil {
			return fmt.Errorf("unable to parse manifest for image '%s': %w", imageFull, err)
		}

		// Validate platform if given.
		for _, platform := range []*ocispec.Platform{mainDescriptor.Platform, mf.Config.Platform} {
			if platform != nil {
				if platform.Architecture != "amd64" || platform.OS != "linux" {
					return fmt.Errorf("image '%s' has incorrect platform (expected linux/amd64, got %s/%s)", imageFull, platform.OS, platform.Architecture)
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
		return fmt.Errorf("estimated total size of images (%d MB) exceeds storage size set in ROFL manifest (%d MB)", totalSize/1024/1024, manifest.Resources.Storage.Size)
	}

	return nil
}
