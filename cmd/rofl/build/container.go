package build

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/net/idna"

	compose "github.com/compose-spec/compose-go/v2/cli"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/registry/remote"

	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"

	"github.com/oasisprotocol/cli/build/env"
	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
)

// tdxBuildContainer builds a TDX-based container ROFL app.
func tdxBuildContainer(
	buildEnv env.ExecEnv,
	tmpDir string,
	npa *common.NPASelection,
	manifest *buildRofl.Manifest,
	deployment *buildRofl.Deployment,
	bnd *bundle.Bundle,
) error {
	fmt.Println("Building a container-based TDX ROFL application...")

	wantedArtifacts := tdxWantedArtifacts(manifest, buildRofl.LatestContainerArtifacts)
	artifacts := tdxFetchArtifacts(wantedArtifacts)

	// Validate compose file.
	fmt.Println("Validating compose file...")
	options, err := compose.NewProjectOptions([]string{artifacts[artifactContainerCompose]})
	if err != nil {
		return fmt.Errorf("failed to set up compose options: %w", err)
	}
	proj, err := options.LoadProject(context.Background())
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("compose file validation failed")
	}

	// Keep track of all images encountered, as we will need them in later steps.
	images := []string{}

	// Make sure that the image fields for all services contain a FQDN or it will cause
	// Podman errors when trying to run it.
	for serviceName, service := range proj.Services {
		image := service.Image
		images = append(images, image)

		validationFailedErr := fmt.Errorf("compose file validation failed: image '%s' of service '%s' is not a fully-qualified domain name", image, serviceName)

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
	}

	// Make sure that we have images.
	if len(images) == 0 {
		return fmt.Errorf("compose file validation failed: no images defined")
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
			return fmt.Errorf("compose file validation failed: %w", err)
		}
		mainDescriptor, mfRaw, err := oras.FetchBytes(context.Background(), repo, tag, oras.DefaultFetchBytesOptions)
		if err != nil {
			return fmt.Errorf("compose file validation failed: unable to fetch manifest for image '%s': %w", imageFull, err)
		}
		var mf ocispec.Manifest
		if err = json.Unmarshal(mfRaw, &mf); err != nil {
			return fmt.Errorf("compose file validation failed: unable to parse manifest for image '%s': %w", imageFull, err)
		}

		// Validate platform if given.
		for _, platform := range []*ocispec.Platform{mainDescriptor.Platform, mf.Config.Platform} {
			if platform != nil {
				if platform.Architecture != "amd64" || platform.OS != "linux" {
					return fmt.Errorf("compose file validation failed: image '%s' has incorrect platform (expected linux/amd64, got %s/%s)", imageFull, platform.OS, platform.Architecture)
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
		return fmt.Errorf("compose file validation failed: estimated total size of images (%d MB) exceeds storage size set in ROFL manifest (%d MB)", totalSize/1024/1024, manifest.Resources.Storage.Size)
	}

	// Use the pre-built container runtime.
	initPath := artifacts[artifactContainerRuntime]

	stage2, err := tdxPrepareStage2(buildEnv, tmpDir, artifacts, initPath, map[string]string{
		artifacts[artifactContainerCompose]: "etc/oasis/containers/compose.yaml",
	})
	if err != nil {
		return err
	}

	// Configure app ID.
	var extraKernelOpts []string
	extraKernelOpts = append(extraKernelOpts,
		fmt.Sprintf("ROFL_APP_ID=%s", deployment.AppID),
	)

	// Obtain and configure trust root.
	trustRoot, err := fetchTrustRoot(npa, deployment.TrustRoot)
	if err != nil {
		return err
	}
	extraKernelOpts = append(extraKernelOpts,
		fmt.Sprintf("ROFL_CONSENSUS_TRUST_ROOT=%s", trustRoot),
	)

	fmt.Println("Creating ORC bundle...")

	return tdxBundleComponent(buildEnv, manifest, artifacts, bnd, stage2, extraKernelOpts)
}
