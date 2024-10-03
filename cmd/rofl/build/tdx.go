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
	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

// TODO: Replace these URIs with a better mechanism for managing releases.
const (
	artifactFirmware = "firmware"
	artifactKernel   = "kernel"
	artifactStage2   = "stage 2 template"

	defaultFirmwareURI       = "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.2.0/ovmf.tdx.fd"
	defaultKernelURI         = "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.2.0/stage1.bin"
	defaultStage2TemplateURI = "https://github.com/oasisprotocol/oasis-boot/releases/download/v0.2.0/stage2-basic.tar.bz2"
)

var knownHashes = map[string]string{
	defaultFirmwareURI:       "db47100a7d6a0c1f6983be224137c3f8d7cb09b63bb1c7a5ee7829d8e994a42f",
	defaultKernelURI:         "0c4a74af5e3860e1b9c79b38aff9de8c59aa92f14da715fbfd04a9362ee4cd59",
	defaultStage2TemplateURI: "8cbc67e4a05b01e6fc257a3ef378db50ec230bc4c7aacbfb9abf0f5b17dcb8fd",
}

var (
	tdxFirmwareURI        string
	tdxFirmwareHash       string
	tdxKernelURI          string
	tdxKernelHash         string
	tdxStage2TemplateURI  string
	tdxStage2TemplateHash string

	tdxResourcesMemory   uint64
	tdxResourcesCPUCount uint8

	tdxCmd = &cobra.Command{
		Use:   "tdx",
		Short: "Build a TDX-based ROFL application",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)

			if npa.ParaTime == nil {
				cobra.CheckErr("no ParaTime selected")
			}

			// Obtain required artifacts.
			artifacts := make(map[string]string)
			for _, ar := range []struct {
				kind      string
				uri       string
				knownHash string
			}{
				{artifactFirmware, tdxFirmwareURI, tdxFirmwareHash},
				{artifactKernel, tdxKernelURI, tdxKernelHash},
				{artifactStage2, tdxStage2TemplateURI, tdxStage2TemplateHash},
			} {
				// Automatically populate known hashes for known URIs.
				if ar.knownHash == "" {
					ar.knownHash = knownHashes[ar.uri]
				}

				artifacts[ar.kind] = maybeDownloadArtifact(ar.kind, ar.uri, ar.knownHash)
			}

			fmt.Println("Building a TDX-based Rust ROFL application...")

			detectBuildMode(npa)
			tdxSetupBuildEnv()

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
					Name: pkgMeta.Name,
					ID:   npa.ParaTime.Namespace(),
				},
			}
			bnd.Manifest.Version, err = version.FromString(pkgMeta.Version)
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

			// Create temporary directory and unpack stage 2 template into it.
			fmt.Println("Preparing stage 2 root filesystem...")
			tmpDir, err := os.MkdirTemp("", "oasis-build-stage2")
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to create temporary stage 2 build directory: %w", err))
			}
			defer os.RemoveAll(tmpDir) // TODO: This doesn't work because of cobra.CheckErr

			rootfsDir := filepath.Join(tmpDir, "rootfs")
			if err = os.Mkdir(rootfsDir, 0o755); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to create temporary rootfs directory: %w", err))
			}

			// Unpack template into temporary directory.
			fmt.Println("Unpacking template...")
			if err = extractArchive(artifacts[artifactStage2], rootfsDir); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to extract stage 2 template: %w", err))
			}

			// Add runtime as init.
			fmt.Println("Adding runtime as init...")
			err = copyFile(initPath, filepath.Join(rootfsDir, "init"), 0o755)
			cobra.CheckErr(err)

			// Create an ext4 filesystem.
			fmt.Println("Creating ext4 filesystem...")
			rootfsImage := filepath.Join(tmpDir, "rootfs.ext4")
			rootfsSize, err := createExt4Fs(rootfsImage, rootfsDir)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to create rootfs image: %w", err))
			}

			// Create dm-verity hash tree.
			fmt.Println("Creating dm-verity hash tree...")
			hashFile := filepath.Join(tmpDir, "rootfs.hash")
			rootHash, err := createVerityHashTree(rootfsImage, hashFile)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to create verity hash tree: %w", err))
			}

			// Concatenate filesystem and hash tree into one image.
			if err = concatFiles(rootfsImage, hashFile); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to concatenate rootfs and hash tree files: %w", err))
			}

			fmt.Println("Creating ORC bundle...")

			// Add the ROFL component.
			firmwareName := "firmware.fd"
			kernelName := "kernel.bin"
			stage2Name := "stage2.img"

			comp := bundle.Component{
				Kind: component.ROFL,
				Name: pkgMeta.Name,
				TDX: &bundle.TDXMetadata{
					Firmware:    firmwareName,
					Kernel:      kernelName,
					Stage2Image: stage2Name,
					ExtraKernelOptions: []string{
						"console=ttyS0",
						fmt.Sprintf("oasis.stage2.roothash=%s", rootHash),
						fmt.Sprintf("oasis.stage2.hash_offset=%d", rootfsSize),
					},
					Resources: bundle.TDXResources{
						Memory:   tdxResourcesMemory,
						CPUCount: tdxResourcesCPUCount,
					},
				},
			}
			bnd.Manifest.Components = append(bnd.Manifest.Components, &comp)

			if err = bnd.Manifest.Validate(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to validate manifest: %w", err))
			}

			// Add all files.
			fileMap := map[string]string{
				firmwareName: artifacts[artifactFirmware],
				kernelName:   artifacts[artifactKernel],
				stage2Name:   rootfsImage,
			}
			for dst, src := range fileMap {
				_ = bnd.Add(dst, bundle.NewFileData(src))
			}

			// Write the bundle out.
			outFn := fmt.Sprintf("%s.orc", bnd.Manifest.Name)
			if outputFn != "" {
				outFn = outputFn
			}
			if err = bnd.Write(outFn); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to write output bundle: %w", err))
			}

			fmt.Printf("ROFL app built and bundle written to '%s'.\n", outFn)
		},
	}
)

// tdxSetupBuildEnv sets up the TDX build environment.
func tdxSetupBuildEnv() {
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
	tdxFlags.StringVar(&tdxFirmwareHash, "firmware-hash", "", "optional SHA256 hash of firmware image")
	tdxFlags.StringVar(&tdxKernelURI, "kernel", defaultKernelURI, "URL or path to kernel image")
	tdxFlags.StringVar(&tdxKernelHash, "kernel-hash", "", "optional SHA256 hash of kernel image")
	tdxFlags.StringVar(&tdxStage2TemplateURI, "template", defaultStage2TemplateURI, "URL or path to stage 2 template")
	tdxFlags.StringVar(&tdxStage2TemplateHash, "template-hash", "", "optional SHA256 hash of stage 2 template")

	tdxFlags.Uint64Var(&tdxResourcesMemory, "memory", 512, "required amount of VM memory in megabytes")
	tdxFlags.Uint8Var(&tdxResourcesCPUCount, "cpus", 1, "required number of vCPUs")

	tdxCmd.Flags().AddFlagSet(common.SelectorNPFlags)
	tdxCmd.Flags().AddFlagSet(tdxFlags)
}
