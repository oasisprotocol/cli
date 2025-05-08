package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/version"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle/component"

	"github.com/oasisprotocol/cli/build/cargo"
	"github.com/oasisprotocol/cli/build/env"
	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
)

// Artifact kinds.
const (
	artifactFirmware         = "firmware"
	artifactKernel           = "kernel"
	artifactStage2           = "stage 2 template"
	artifactContainerRuntime = "rofl-container runtime"
	artifactContainerCompose = "compose.yaml"
)

// tdxBuildRaw builds a TDX-based "raw" ROFL app.
func tdxBuildRaw(
	buildEnv env.ExecEnv,
	tmpDir string,
	npa *common.NPASelection,
	manifest *buildRofl.Manifest,
	deployment *buildRofl.Deployment,
	bnd *bundle.Bundle,
) error {
	wantedArtifacts := tdxWantedArtifacts(manifest, buildRofl.LatestBasicArtifacts)
	artifacts := tdxFetchArtifacts(wantedArtifacts)

	fmt.Println("Building a TDX-based Rust ROFL application...")

	tdxSetupBuildEnv(deployment, npa)

	// Obtain package metadata.
	pkgMeta, err := cargo.GetMetadata(buildEnv)
	if err != nil {
		return fmt.Errorf("failed to obtain package metadata: %w", err)
	}
	// Quickly validate whether the SDK has been built for TDX.
	dep := pkgMeta.FindDependency("oasis-runtime-sdk")
	switch dep {
	case nil:
		fmt.Println("WARNING: No oasis-runtime-sdk dependency found. Skipping validation of TDX binary.")
	default:
		// Check for presence of TDX feature.
		if !dep.HasFeature("tdx") {
			return fmt.Errorf("runtime does not use the 'tdx' feature for oasis-runtime-sdk")
		}
	}

	fmt.Println("Building runtime binary...")
	initPath, err := cargo.Build(buildEnv, true, "", nil)
	if err != nil {
		return fmt.Errorf("failed to build runtime binary: %w", err)
	}

	stage2, err := tdxPrepareStage2(buildEnv, tmpDir, artifacts, initPath, nil)
	if err != nil {
		return err
	}

	fmt.Println("Creating ORC bundle...")

	return tdxBundleComponent(buildEnv, manifest, artifacts, bnd, stage2, nil)
}

type artifact struct {
	kind string
	uri  string
}

// tdxWantedArtifacts returns the list of wanted artifacts based on the passed manifest and a set of
// defaults. In case an artifact is not defined in the manifest, it is taken from defaults.
func tdxWantedArtifacts(manifest *buildRofl.Manifest, defaults buildRofl.ArtifactsConfig) []*artifact {
	var artifacts []*artifact
	for _, a := range []struct {
		kind   string
		getter func(*buildRofl.ArtifactsConfig) string
	}{
		{artifactFirmware, func(ac *buildRofl.ArtifactsConfig) string { return ac.Firmware }},
		{artifactKernel, func(ac *buildRofl.ArtifactsConfig) string { return ac.Kernel }},
		{artifactStage2, func(ac *buildRofl.ArtifactsConfig) string { return ac.Stage2 }},
		{artifactContainerRuntime, func(ac *buildRofl.ArtifactsConfig) string { return ac.Container.Runtime }},
		{artifactContainerCompose, func(ac *buildRofl.ArtifactsConfig) string { return ac.Container.Compose }},
	} {
		var uri string
		if manifest.Artifacts != nil {
			uri = a.getter(manifest.Artifacts)
		}
		if uri == "" {
			uri = a.getter(&defaults)
		}
		if uri != "" {
			artifacts = append(artifacts, &artifact{a.kind, uri})
		}
	}
	return artifacts
}

// tdxFetchArtifacts obtains all of the required artifacts for a TDX image.
func tdxFetchArtifacts(artifacts []*artifact) map[string]string {
	result := make(map[string]string)
	for _, ar := range artifacts {
		result[ar.kind] = maybeDownloadArtifact(ar.kind, ar.uri)
	}
	return result
}

type tdxStage2 struct {
	fn       string
	rootHash string
	fsSize   int64
}

// tdxPrepareStage2 prepares the stage 2 rootfs.
func tdxPrepareStage2(
	buildEnv env.ExecEnv,
	tmpDir string,
	artifacts map[string]string,
	initPath string,
	extraFiles map[string]string,
) (*tdxStage2, error) {
	// Create temporary directory and unpack stage 2 template into it.
	fmt.Println("Preparing stage 2 root filesystem...")
	rootfsDir := filepath.Join(tmpDir, "rootfs")
	if err := os.Mkdir(rootfsDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create temporary rootfs directory: %w", err)
	}

	// Unpack template into temporary directory.
	fmt.Println("Unpacking template...")
	if err := extractArchive(artifacts[artifactStage2], rootfsDir); err != nil {
		return nil, fmt.Errorf("failed to extract stage 2 template: %w", err)
	}

	// Add runtime as init.
	fmt.Println("Adding runtime as init...")

	initHash, err := sha256File(initPath)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Runtime hash: %s\n", initHash)

	if err := copyFile(initPath, filepath.Join(rootfsDir, "init"), 0o755); err != nil {
		return nil, err
	}

	// Copy any extra files.
	fmt.Println("Adding extra files...")
	for src, dst := range extraFiles {
		if err := copyFile(src, filepath.Join(rootfsDir, dst), 0o644); err != nil {
			return nil, err
		}
	}

	// Create the root filesystem.
	fmt.Println("Creating squashfs filesystem...")
	rootfsImage := filepath.Join(tmpDir, "rootfs.squashfs")
	rootfsSize, err := createSquashFs(buildEnv, rootfsImage, rootfsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create rootfs image: %w", err)
	}

	// Create dm-verity hash tree.
	fmt.Println("Creating dm-verity hash tree...")
	hashFile := filepath.Join(tmpDir, "rootfs.hash")
	rootHash, err := createVerityHashTree(buildEnv, rootfsImage, hashFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create verity hash tree: %w", err)
	}

	// Concatenate filesystem and hash tree into one image.
	if err = concatFiles(rootfsImage, hashFile); err != nil {
		return nil, fmt.Errorf("failed to concatenate rootfs and hash tree files: %w", err)
	}

	return &tdxStage2{
		fn:       rootfsImage,
		rootHash: rootHash,
		fsSize:   rootfsSize,
	}, nil
}

// tdxBundleComponent adds the ROFL component to the given bundle.
func tdxBundleComponent(
	buildEnv env.ExecEnv,
	manifest *buildRofl.Manifest,
	artifacts map[string]string,
	bnd *bundle.Bundle,
	stage2 *tdxStage2,
	extraKernelOpts []string,
) error {
	// Add the ROFL component.
	firmwareName := "firmware.fd"
	kernelName := "kernel.bin"
	stage2Name := "stage2.img"

	comp := bundle.Component{
		Kind: component.ROFL,
		Name: bnd.Manifest.Name,
		TDX: &bundle.TDXMetadata{
			Firmware:    firmwareName,
			Kernel:      kernelName,
			Stage2Image: stage2Name,
			ExtraKernelOptions: []string{
				"console=ttyS0",
				fmt.Sprintf("oasis.stage2.roothash=%s", stage2.rootHash),
				fmt.Sprintf("oasis.stage2.hash_offset=%d", stage2.fsSize),
			},
			Resources: bundle.TDXResources{
				Memory:   manifest.Resources.Memory,
				CPUCount: manifest.Resources.CPUCount,
			},
		},
	}

	var err error
	comp.Version, err = version.FromString(manifest.Version)
	if err != nil {
		return fmt.Errorf("unsupported package version format: %w", err)
	}

	storageKind := buildRofl.StorageKindNone
	if manifest.Resources.Storage != nil {
		storageKind = manifest.Resources.Storage.Kind
	}

	switch storageKind {
	case buildRofl.StorageKindNone:
	case buildRofl.StorageKindRAM:
		comp.TDX.ExtraKernelOptions = append(comp.TDX.ExtraKernelOptions,
			"oasis.stage2.storage_mode=ram",
			fmt.Sprintf("oasis.stage2.storage_size=%d", manifest.Resources.Storage.Size*1024*1024),
		)
	case buildRofl.StorageKindDiskEphemeral, buildRofl.StorageKindDiskPersistent:
		var storageMode string
		switch storageKind {
		case buildRofl.StorageKindDiskPersistent:
			// Persistent storage needs to be set up by stage 2.
			storageMode = "custom"

			// Add some sparse padding to allow for growth of the root partition during upgrades.
			// Note that this will not actually take any space so it could be arbitrarily large.
			if err = padWithEmptySpace(stage2.fn, 2*1024*1024*1024); err != nil {
				return err
			}

			comp.TDX.Stage2Persist = true
		case buildRofl.StorageKindDiskEphemeral:
			// Ephemeral storage can be set up by stage 1 directly.
			storageMode = "disk"
		}

		// Allocate some space after regular stage2.
		const sectorSize = 512
		storageSize := manifest.Resources.Storage.Size * 1024 * 1024
		var storageOffset uint64
		storageOffset, err = appendEmptySpace(stage2.fn, storageSize, sectorSize)
		if err != nil {
			return err
		}

		comp.TDX.ExtraKernelOptions = append(comp.TDX.ExtraKernelOptions,
			fmt.Sprintf("oasis.stage2.storage_mode=%s", storageMode),
			fmt.Sprintf("oasis.stage2.storage_size=%d", storageSize/sectorSize),
			fmt.Sprintf("oasis.stage2.storage_offset=%d", storageOffset/sectorSize),
		)
	default:
		return fmt.Errorf("unsupported storage mode: %s", storageKind)
	}

	// Use qcow2 image format to support sparse files.
	if err = convertToQcow2(buildEnv, stage2.fn); err != nil {
		return fmt.Errorf("failed to convert to qcow2 image: %w", err)
	}
	comp.TDX.Stage2Format = "qcow2"

	// Add extra kernel options.
	comp.TDX.ExtraKernelOptions = append(comp.TDX.ExtraKernelOptions, extraKernelOpts...)

	bnd.Manifest.Components = append(bnd.Manifest.Components, &comp)

	if err = bnd.Manifest.Validate(); err != nil {
		return fmt.Errorf("failed to validate manifest: %w", err)
	}

	// Add all files.
	fileMap := map[string]string{
		firmwareName: artifacts[artifactFirmware],
		kernelName:   artifacts[artifactKernel],
		stage2Name:   stage2.fn,
	}
	for dst, src := range fileMap {
		_ = bnd.Add(dst, bundle.NewFileData(src))
	}

	// Compute expected component identity and include it in the manifest.
	ids, err := roflCommon.ComputeComponentIdentity(bnd, &comp)
	if err != nil {
		return fmt.Errorf("failed to compute component identity: %w", err)
	}
	comp.Identities = ids

	return nil
}

// tdxSetupBuildEnv sets up the TDX build environment.
func tdxSetupBuildEnv(deployment *buildRofl.Deployment, npa *common.NPASelection) {
	setupBuildEnv(deployment, npa)

	switch buildMode {
	case buildModeProduction:
		// Production builds.
		fmt.Println("Building in production mode.")

		for _, kv := range os.Environ() {
			key, _, _ := strings.Cut(kv, "=")
			if strings.HasPrefix(key, "OASIS_UNSAFE_") {
				os.Unsetenv(key)
			}
		}
	case buildModeUnsafe:
		// Unsafe debug builds.
		fmt.Println("WARNING: Building in UNSAFE DEBUG mode with MOCK TDX.")
		fmt.Println("WARNING: This build will NOT BE DEPLOYABLE outside local test environments.")

		os.Setenv("OASIS_UNSAFE_SKIP_AVR_VERIFY", "1")
		os.Setenv("OASIS_UNSAFE_ALLOW_DEBUG_ENCLAVES", "1")
		os.Unsetenv("OASIS_UNSAFE_MOCK_SGX")
		os.Unsetenv("OASIS_UNSAFE_MOCK_TEE")
		os.Unsetenv("OASIS_UNSAFE_SKIP_KM_POLICY")
	default:
		cobra.CheckErr(fmt.Errorf("unsupported build mode: %s", buildMode))
	}
}
