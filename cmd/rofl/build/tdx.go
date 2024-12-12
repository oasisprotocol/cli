package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/common/version"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle/component"

	"github.com/oasisprotocol/cli/build/cargo"
	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

// TODO: Replace these URIs with a better mechanism for managing releases.
const (
	artifactFirmware = "firmware"
	artifactKernel   = "kernel"
	artifactStage2   = "stage 2 template"

	defaultFirmwareURI       = "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.2.0/ovmf.tdx.fd#db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f"
	defaultKernelURI         = "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.2.0/stage1.bin#0c4a74af5e3860e1b9c79b38aff9de8c59aa92f14da715fbfd04a9362ee4cd59"
	defaultStage2TemplateURI = "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.2.0/stage2-basic.tar.bz2#8cbc67e4a05b01e6fc257a3ef378db50ec230bc4c7aacbfb9abf0f5b17dcb8fd"
)

var (
	tdxFirmwareURI       string
	tdxKernelURI         string
	tdxStage2TemplateURI string

	tdxResourcesMemory   uint64
	tdxResourcesCPUCount uint8

	tdxTmpStorageMode string
	tdxTmpStorageSize uint64

	tdxCmd = &cobra.Command{
		Use:   "tdx",
		Short: "Build a TDX-based ROFL application",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			manifest, _ := roflCommon.MaybeLoadManifestAndSetNPA(cfg, npa)

			if npa.ParaTime == nil {
				cobra.CheckErr("no ParaTime selected")
			}

			wantedArtifacts := tdxGetDefaultArtifacts()
			tdxOverrideArtifacts(manifest, wantedArtifacts)
			artifacts := tdxFetchArtifacts(wantedArtifacts)

			fmt.Println("Building a TDX-based Rust ROFL application...")

			detectBuildMode(npa)
			tdxSetupBuildEnv(manifest, npa)

			// Obtain package metadata.
			pkgMeta, err := cargo.GetMetadata()
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to obtain package metadata: %w", err))
			}
			// Quickly validate whether the SDK has been built for TDX.
			dep := pkgMeta.FindDependency("oasis-runtime-sdk")
			switch dep {
			case nil:
				fmt.Println("WARNING: No oasis-runtime-sdk dependency found. Skipping validation of TDX binary.")
			default:
				// Check for presence of TDX feature.
				if !dep.HasFeature("tdx") {
					cobra.CheckErr(fmt.Errorf("runtime does not use the 'tdx' feature for oasis-runtime-sdk"))
				}
			}

			// Start creating the bundle early so we can fail before building anything.
			bnd := &bundle.Bundle{
				Manifest: &bundle.Manifest{
					ID: npa.ParaTime.Namespace(),
				},
			}
			var rawVersion string
			switch manifest {
			case nil:
				// No ROFL app manifest, use Cargo manifest.
				bnd.Manifest.Name = pkgMeta.Name
				rawVersion = pkgMeta.Version
			default:
				// Use ROFL app manifest.
				bnd.Manifest.Name = manifest.Name
				rawVersion = manifest.Version
			}
			bnd.Manifest.Version, err = version.FromString(rawVersion)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("unsupported package version format: %w", err))
			}

			fmt.Printf("Name:    %s\n", bnd.Manifest.Name)
			fmt.Printf("Version: %s\n", bnd.Manifest.Version)

			fmt.Println("Building runtime binary...")
			initPath, err := cargo.Build(true, "", nil)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to build runtime binary: %w", err))
			}

			stage2, err := tdxPrepareStage2(artifacts, initPath, nil)
			if err != nil {
				cobra.CheckErr(err)
			}
			defer os.RemoveAll(stage2.tmpDir)

			fmt.Println("Creating ORC bundle...")

			outFn, err := tdxBundleComponent(manifest, artifacts, bnd, stage2, nil)
			if err != nil {
				_ = os.RemoveAll(stage2.tmpDir)
				cobra.CheckErr(err)
			}

			fmt.Printf("ROFL app built and bundle written to '%s'.\n", outFn)
		},
	}
)

type artifact struct {
	kind string
	uri  string
}

// tdxGetDefaultArtifacts returns the list of default TDX artifacts.
func tdxGetDefaultArtifacts() []*artifact {
	return []*artifact{
		{artifactFirmware, tdxFirmwareURI},
		{artifactKernel, tdxKernelURI},
		{artifactStage2, tdxStage2TemplateURI},
	}
}

// tdxFetchArtifacts obtains all of the required artifacts for a TDX image.
func tdxFetchArtifacts(artifacts []*artifact) map[string]string {
	result := make(map[string]string)
	for _, ar := range artifacts {
		result[ar.kind] = maybeDownloadArtifact(ar.kind, ar.uri)
	}
	return result
}

// tdxOverrideArtifacts overrides artifacts based on the manifest.
func tdxOverrideArtifacts(manifest *buildRofl.Manifest, artifacts []*artifact) {
	if manifest == nil || manifest.Artifacts == nil {
		return
	}
	overrides := manifest.Artifacts

	for _, artifact := range artifacts {
		var overrideURI string
		switch artifact.kind {
		case artifactFirmware:
			overrideURI = overrides.Firmware
		case artifactKernel:
			overrideURI = overrides.Kernel
		case artifactStage2:
			overrideURI = overrides.Stage2
		case artifactContainerRuntime:
			overrideURI = overrides.Container.Runtime
		case artifactContainerCompose:
			overrideURI = overrides.Container.Compose
		default:
		}

		if overrideURI == "" {
			continue
		}
		artifact.uri = overrideURI
	}
}

type tdxStage2 struct {
	tmpDir   string
	fn       string
	rootHash string
	fsSize   int64
}

// tdxPrepareStage2 prepares the stage 2 rootfs.
func tdxPrepareStage2(artifacts map[string]string, initPath string, extraFiles map[string]string) (*tdxStage2, error) {
	var ok bool

	// Create temporary directory and unpack stage 2 template into it.
	fmt.Println("Preparing stage 2 root filesystem...")
	tmpDir, err := os.MkdirTemp("", "oasis-build-stage2")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary stage 2 build directory: %w", err)
	}
	defer func() {
		// Ensure temporary directory is removed on errors.
		if !ok {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	rootfsDir := filepath.Join(tmpDir, "rootfs")
	if err = os.Mkdir(rootfsDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create temporary rootfs directory: %w", err)
	}

	// Unpack template into temporary directory.
	fmt.Println("Unpacking template...")
	if err = extractArchive(artifacts[artifactStage2], rootfsDir); err != nil {
		return nil, fmt.Errorf("failed to extract stage 2 template: %w", err)
	}

	// Add runtime as init.
	fmt.Println("Adding runtime as init...")
	if err = copyFile(initPath, filepath.Join(rootfsDir, "init"), 0o755); err != nil {
		return nil, err
	}

	// Copy any extra files.
	fmt.Println("Adding extra files...")
	for src, dst := range extraFiles {
		if err = copyFile(src, filepath.Join(rootfsDir, dst), 0o644); err != nil {
			return nil, err
		}
	}

	// Create the root filesystem.
	fmt.Println("Creating squashfs filesystem...")
	rootfsImage := filepath.Join(tmpDir, "rootfs.squashfs")
	rootfsSize, err := createSquashFs(rootfsImage, rootfsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create rootfs image: %w", err)
	}

	// Create dm-verity hash tree.
	fmt.Println("Creating dm-verity hash tree...")
	hashFile := filepath.Join(tmpDir, "rootfs.hash")
	rootHash, err := createVerityHashTree(rootfsImage, hashFile)
	if err != nil {
		return nil, fmt.Errorf("failed to create verity hash tree: %w", err)
	}

	// Concatenate filesystem and hash tree into one image.
	if err = concatFiles(rootfsImage, hashFile); err != nil {
		return nil, fmt.Errorf("failed to concatenate rootfs and hash tree files: %w", err)
	}

	ok = true

	return &tdxStage2{
		tmpDir:   tmpDir,
		fn:       rootfsImage,
		rootHash: rootHash,
		fsSize:   rootfsSize,
	}, nil
}

// tdxBundleComponent adds the ROFL component to the given bundle.
func tdxBundleComponent(
	manifest *buildRofl.Manifest,
	artifacts map[string]string,
	bnd *bundle.Bundle,
	stage2 *tdxStage2,
	extraKernelOpts []string,
) (string, error) {
	// Add the ROFL component.
	firmwareName := "firmware.fd"
	kernelName := "kernel.bin"
	stage2Name := "stage2.img"

	// When manifest is available, override values from manifest.
	if manifest != nil {
		tdxResourcesMemory = manifest.Resources.Memory
		tdxResourcesCPUCount = manifest.Resources.CPUCount

		switch {
		case manifest.Resources.EphemeralStorage == nil:
			tdxTmpStorageMode = buildRofl.EphemeralStorageKindNone
		default:
			tdxTmpStorageMode = manifest.Resources.EphemeralStorage.Kind
			tdxTmpStorageSize = manifest.Resources.EphemeralStorage.Size
		}
	}

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
				Memory:   tdxResourcesMemory,
				CPUCount: tdxResourcesCPUCount,
			},
		},
	}

	switch tdxTmpStorageMode {
	case buildRofl.EphemeralStorageKindNone:
	case buildRofl.EphemeralStorageKindRAM:
		comp.TDX.ExtraKernelOptions = append(comp.TDX.ExtraKernelOptions,
			"oasis.stage2.storage_mode=ram",
			fmt.Sprintf("oasis.stage2.storage_size=%d", tdxTmpStorageSize*1024*1024),
		)
	case buildRofl.EphemeralStorageKindDisk:
		// Allocate some space after regular stage2.
		const sectorSize = 512
		storageSize := tdxTmpStorageSize * 1024 * 1024
		storageOffset, err := appendEmptySpace(stage2.fn, storageSize, sectorSize)
		if err != nil {
			return "", err
		}

		comp.TDX.ExtraKernelOptions = append(comp.TDX.ExtraKernelOptions,
			"oasis.stage2.storage_mode=disk",
			fmt.Sprintf("oasis.stage2.storage_size=%d", storageSize/sectorSize),
			fmt.Sprintf("oasis.stage2.storage_offset=%d", storageOffset/sectorSize),
		)
	default:
		return "", fmt.Errorf("unsupported ephemeral storage mode: %s", tdxTmpStorageMode)
	}

	// TODO: (Oasis Core 25.0+) Use qcow2 image format to support sparse files.

	// Add extra kernel options.
	comp.TDX.ExtraKernelOptions = append(comp.TDX.ExtraKernelOptions, extraKernelOpts...)

	bnd.Manifest.Components = append(bnd.Manifest.Components, &comp)

	if err := bnd.Manifest.Validate(); err != nil {
		return "", fmt.Errorf("failed to validate manifest: %w", err)
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

	// Write the bundle out.
	outFn := fmt.Sprintf("%s.orc", bnd.Manifest.Name)
	if outputFn != "" {
		outFn = outputFn
	}
	if err := bnd.Write(outFn); err != nil {
		return "", fmt.Errorf("failed to write output bundle: %w", err)
	}
	return outFn, nil
}

// tdxSetupBuildEnv sets up the TDX build environment.
func tdxSetupBuildEnv(manifest *buildRofl.Manifest, npa *common.NPASelection) {
	setupBuildEnv(manifest, npa)

	switch buildMode {
	case buildModeProduction, buildModeAuto:
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
		fmt.Println("WARNING: Building in UNSAFE DEBUG mode with MOCK SGX.")
		fmt.Println("WARNING: This build will NOT BE DEPLOYABLE outside local test environments.")

		os.Setenv("OASIS_UNSAFE_SKIP_AVR_VERIFY", "1")
		os.Setenv("OASIS_UNSAFE_ALLOW_DEBUG_ENCLAVES", "1")
		os.Unsetenv("OASIS_UNSAFE_MOCK_SGX")
		os.Unsetenv("OASIS_UNSAFE_SKIP_KM_POLICY")
	default:
		cobra.CheckErr(fmt.Errorf("unsupported build mode: %s", buildMode))
	}
}

func init() {
	tdxFlags := flag.NewFlagSet("", flag.ContinueOnError)
	tdxFlags.StringVar(&tdxFirmwareURI, "firmware", defaultFirmwareURI, "URL or path to firmware image")
	tdxFlags.StringVar(&tdxKernelURI, "kernel", defaultKernelURI, "URL or path to kernel image")
	tdxFlags.StringVar(&tdxStage2TemplateURI, "template", defaultStage2TemplateURI, "URL or path to stage 2 template")

	tdxFlags.Uint64Var(&tdxResourcesMemory, "memory", 512, "required amount of VM memory in megabytes")
	tdxFlags.Uint8Var(&tdxResourcesCPUCount, "cpus", 1, "required number of vCPUs")

	tdxFlags.StringVar(&tdxTmpStorageMode, "ephemeral-storage-mode", "none", "ephemeral storage mode")
	tdxFlags.Uint64Var(&tdxTmpStorageSize, "ephemeral-storage-size", 64, "ephemeral storage size in megabytes")

	tdxCmd.Flags().AddFlagSet(common.SelectorNPFlags)
	tdxCmd.Flags().AddFlagSet(tdxFlags)

	// XXX: We need to define the flags here due to init order (container gets called before tdx).
	tdxContainerCmd.Flags().AddFlagSet(common.SelectorNPFlags)
	tdxContainerCmd.Flags().AddFlagSet(tdxFlags)

	// Override some flag defaults.
	flags := tdxContainerCmd.Flags()
	flags.Lookup("template").DefValue = defaultContainerStage2TemplateURI
	_ = flags.Lookup("template").Value.Set(defaultContainerStage2TemplateURI)
	flags.Lookup("ephemeral-storage-mode").DefValue = "ram"
	_ = flags.Lookup("ephemeral-storage-mode").Value.Set("ram")

	tdxCmd.AddCommand(tdxContainerCmd)
}
