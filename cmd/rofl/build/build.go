package build

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/sgx"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/build/env"
	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
)

// Build modes.
const (
	buildModeProduction = "production"
	buildModeUnsafe     = "unsafe"
)

var (
	outputFn     string
	buildMode    string
	offline      bool
	doVerify     bool
	noContainer  bool
	onlyValidate bool

	Cmd = &cobra.Command{
		Use:   "build",
		Short: "Build a ROFL application",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cmd.SilenceUsage = true
			manifest, deployment, npa := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: false,
			})

			if onlyValidate {
				fmt.Println("Validating app...")
				_, err := ValidateApp(manifest, ValidationOpts{})
				if err == nil {
					fmt.Println("App validation passed.")
					return nil
				}
				return err
			}

			fmt.Println("Building a ROFL application...")
			fmt.Printf("Deployment: %s\n", roflCommon.DeploymentName)
			fmt.Printf("Network:    %s\n", deployment.Network)
			fmt.Printf("ParaTime:   %s\n", deployment.ParaTime)
			fmt.Printf("Debug:      %v\n", deployment.Debug)
			fmt.Printf("App ID:     %s\n", deployment.AppID)
			fmt.Printf("Name:       %s\n", manifest.Name)
			fmt.Printf("Version:    %s\n", manifest.Version)
			fmt.Printf("TEE:        %s\n", manifest.TEE)
			fmt.Printf("Kind:       %s\n", manifest.Kind)

			switch deployment.Debug {
			case true:
				buildMode = buildModeUnsafe
			case false:
				buildMode = buildModeProduction
			}

			// Prepare temporary build directory.
			tmpDir, err := os.MkdirTemp("", "oasis-build")
			if err != nil {
				return fmt.Errorf("failed to create temporary build directory: %w", err)
			}
			defer os.RemoveAll(tmpDir)

			// Ensure deterministic umask for builds.
			setUmask(0o002)

			// Set LC_ALL=C for the entire build process to ensure deterministic file ordering
			// and consistent behavior across different locale environments.
			os.Setenv("LC_ALL", "C")
			os.Unsetenv("LANG")
			for _, env := range os.Environ() {
				if strings.HasPrefix(env, "LC_") && env != "LC_ALL=C" {
					parts := strings.SplitN(env, "=", 2)
					if len(parts) == 2 {
						os.Unsetenv(parts[0])
					}
				}
			}

			var buildEnv env.ExecEnv
			switch {
			case manifest.Artifacts == nil || manifest.Artifacts.Builder == "" || noContainer:
				buildEnv = env.NewNativeEnv()
			default:
				var baseDir string
				baseDir, err = env.GetBasedir()
				if err != nil {
					return fmt.Errorf("failed to determine base directory: %w", err)
				}

				containerEnv := env.NewContainerEnv(
					manifest.Artifacts.Builder,
					baseDir,
					"/src",
				)
				containerEnv.AddDirectory(tmpDir)
				buildEnv = containerEnv
			}

			if !buildEnv.IsAvailable() {
				return fmt.Errorf("build environment '%s' is not available", buildEnv)
			}

			bnd := &bundle.Bundle{
				Manifest: &bundle.Manifest{
					Name: deployment.AppID,
					ID:   npa.ParaTime.Namespace(),
				},
			}

			// Setup some build environment variables.
			os.Setenv("ROFL_MANIFEST", manifest.SourceFileName())
			os.Setenv("ROFL_DEPLOYMENT_NAME", roflCommon.DeploymentName)
			os.Setenv("ROFL_DEPLOYMENT_NETWORK", deployment.Network)
			os.Setenv("ROFL_DEPLOYMENT_PARATIME", deployment.ParaTime)
			os.Setenv("ROFL_TMPDIR", tmpDir)

			runScript(manifest, buildRofl.ScriptBuildPre)

			switch manifest.TEE {
			case buildRofl.TEETypeSGX:
				// SGX.
				if manifest.Kind != buildRofl.AppKindRaw {
					return fmt.Errorf("unsupported app kind for SGX TEE: %s", manifest.Kind)
				}

				sgxBuild(buildEnv, npa, manifest, deployment, bnd, doVerify)
			case buildRofl.TEETypeTDX:
				// TDX.
				switch manifest.Kind {
				case buildRofl.AppKindRaw:
					err = tdxBuildRaw(buildEnv, tmpDir, npa, manifest, deployment, bnd, doVerify)
				case buildRofl.AppKindContainer:
					err = tdxBuildContainer(buildEnv, tmpDir, npa, manifest, deployment, bnd)
				}
			default:
				return fmt.Errorf("unsupported TEE kind: %s", manifest.TEE)
			}
			if err != nil {
				return err
			}

			runScript(manifest, buildRofl.ScriptBuildPost)

			// Write the bundle out.
			outFn := roflCommon.GetOrcFilename(manifest, roflCommon.DeploymentName)
			if outputFn != "" {
				outFn = outputFn
			}
			if err = bnd.Write(outFn); err != nil {
				return fmt.Errorf("failed to write output bundle: %s", err)
			}

			fmt.Printf("ROFL app built and bundle written to '%s'.\n", outFn)

			fmt.Println("Computing enclave identity...")

			ids, err := roflCommon.ComputeEnclaveIdentity(bnd, "")
			if err != nil {
				return err
			}

			// Setup some post-bundle environment variables.
			os.Setenv("ROFL_BUNDLE", outFn)
			for idx, id := range ids {
				data, _ := id.Enclave.MarshalText()
				os.Setenv(fmt.Sprintf("ROFL_ENCLAVE_ID_%d", idx), string(data))
			}

			runScript(manifest, buildRofl.ScriptBundlePost)

			buildEnclaves := make(map[sgx.EnclaveIdentity]struct{})
			for _, id := range ids {
				buildEnclaves[id.Enclave] = struct{}{}
			}

			allManifestEnclaves := make(map[sgx.EnclaveIdentity]struct{})
			latestManifestEnclaves := make(map[sgx.EnclaveIdentity]struct{})
			for _, eid := range deployment.Policy.Enclaves {
				if eid.IsLatest() {
					latestManifestEnclaves[eid.ID] = struct{}{}
				}
				allManifestEnclaves[eid.ID] = struct{}{}
			}

			// Perform verification when requested.
			if doVerify {
				showIdentityDiff := func(this, other map[sgx.EnclaveIdentity]struct{}, thisName, otherName string) {
					fmt.Printf("%s enclave identities:\n", thisName)
					for enclaveID := range this {
						data, _ := enclaveID.MarshalText()
						fmt.Printf("  - %s\n", string(data))
					}

					fmt.Printf("%s enclave identities:\n", otherName)
					for enclaveID := range other {
						data, _ := enclaveID.MarshalText()
						fmt.Printf("  - %s\n", string(data))
					}
				}

				if !maps.Equal(buildEnclaves, latestManifestEnclaves) {
					fmt.Println("Built enclave identities DIFFER from latest manifest enclave identities!")
					showIdentityDiff(buildEnclaves, latestManifestEnclaves, "Built", "Manifest")
					return fmt.Errorf("enclave identity verification failed")
				}

				fmt.Println("Built enclave identities MATCH latest manifest enclave identities.")
				if len(latestManifestEnclaves) != len(allManifestEnclaves) {
					fmt.Println("NOTE: Non-latest enclave identities present in manifest!")
				}

				// When not in offline mode, also verify on-chain enclave identities.
				if !offline {
					ctx := context.Background()
					var cfgEnclaves map[sgx.EnclaveIdentity]struct{}
					cfgEnclaves, err = roflCommon.GetRegisteredEnclaves(ctx, deployment.AppID, npa)
					if err != nil {
						return err
					}

					if !maps.Equal(allManifestEnclaves, cfgEnclaves) {
						fmt.Println("Manifest enclave identities DIFFER from on-chain enclave identities!")
						showIdentityDiff(allManifestEnclaves, cfgEnclaves, "Manifest", "On-chain")
						return fmt.Errorf("enclave identity verification failed")
					}

					fmt.Println("Manifest enclave identities MATCH on-chain enclave identities.")
				}
				return nil
			}

			// Override the update manifest flag in case the policy does not exist.
			if deployment.Policy == nil {
				roflCommon.NoUpdate = true
			}

			switch roflCommon.NoUpdate {
			case true:
				// Ask the user to update the manifest manually (if the manifest has changed).
				if maps.Equal(buildEnclaves, latestManifestEnclaves) {
					fmt.Println("Built enclave identities already match manifest enclave identities.")
					break
				}

				fmt.Println("Update the manifest with the following identities to use the new app:")
				fmt.Println()
				fmt.Printf("deployments:\n")
				fmt.Printf("  %s:\n", roflCommon.DeploymentName)
				fmt.Printf("    policy:\n")
				fmt.Printf("      enclaves:\n")
				for _, id := range ids {
					data, _ := id.Enclave.MarshalText()
					fmt.Printf("        - \"%s\"\n", string(data))
				}
			case false:
				// Update the manifest with the given enclave identities, overwriting existing ones.
				deployment.Policy.Enclaves = slices.DeleteFunc(deployment.Policy.Enclaves, func(ei *buildRofl.EnclaveIdentity) bool {
					return ei.IsLatest()
				})
				for _, id := range ids {
					deployment.Policy.Enclaves = append(deployment.Policy.Enclaves, &buildRofl.EnclaveIdentity{
						ID: id.Enclave,
					})
				}

				if err = manifest.Save(); err != nil {
					return fmt.Errorf("failed to update manifest: %w", err)
				}

				fmt.Printf("Run `oasis rofl update` to update your ROFL app's on-chain configuration.\n")
			}
			return nil
		},
	}
)

func setupBuildEnv(deployment *buildRofl.Deployment, npa *common.NPASelection) {
	// Configure app ID.
	os.Setenv("ROFL_APP_ID", deployment.AppID)

	// Obtain and configure trust root.
	trustRoot, err := fetchTrustRoot(npa, deployment.TrustRoot)
	if deployment.Debug && err != nil {
		// Trust root is not mandatory for debug builds.
		fmt.Printf("WARNING: no trust root will be provided during compilation: %v\n", err)
		return
	}
	cobra.CheckErr(err)
	os.Setenv("ROFL_CONSENSUS_TRUST_ROOT", trustRoot)
}

// fetchTrustRoot fetches the trust root based on configuration and returns a serialized version
// suitable for inclusion as an environment variable.
func fetchTrustRoot(npa *common.NPASelection, cfg *buildRofl.TrustRootConfig) (string, error) {
	var (
		height int64
		hash   string
	)
	switch {
	case cfg == nil || cfg.Hash == "":
		// Hash is not known, we need to fetch it if not in offline mode.
		if offline {
			return "", fmt.Errorf("trust root hash not available in manifest while in offline mode")
		}

		// Establish connection with the target network.
		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		if err != nil {
			return "", err
		}

		switch cfg {
		case nil:
			// Use latest height.
			height, err = common.GetActualHeight(ctx, conn.Consensus().Core())
			if err != nil {
				return "", err
			}
		default:
			// Use configured height.
			height = int64(cfg.Height) //nolint: gosec
		}

		blk, err := conn.Consensus().Core().GetBlock(ctx, height)
		if err != nil {
			return "", err
		}
		hash = blk.Hash.Hex()
	default:
		// Hash is known, just use it.
		height = int64(cfg.Height) //nolint: gosec
		hash = cfg.Hash
	}

	// TODO: Move this structure to Core.
	type trustRoot struct {
		Height       uint64               `json:"height"`
		Hash         string               `json:"hash"`
		RuntimeID    coreCommon.Namespace `json:"runtime_id"`
		ChainContext string               `json:"chain_context"`
	}
	root := trustRoot{
		Height:       uint64(height), //nolint: gosec
		Hash:         hash,
		RuntimeID:    npa.ParaTime.Namespace(),
		ChainContext: npa.Network.ChainContext,
	}
	encRoot := cbor.Marshal(root)
	return base64.StdEncoding.EncodeToString(encRoot), nil
}

func init() {
	buildFlags := flag.NewFlagSet("", flag.ContinueOnError)
	buildFlags.BoolVar(&offline, "offline", false, "do not perform any operations requiring network access")
	buildFlags.StringVar(&outputFn, "output", "", "output bundle filename")
	buildFlags.BoolVar(&doVerify, "verify", false, "verify build against manifest and on-chain state")
	buildFlags.BoolVar(&noContainer, "no-container", false, "do not use the containerized builder")
	buildFlags.BoolVar(&onlyValidate, "only-validate", false, "validate without building")

	buildFlags.AddFlagSet(roflCommon.DeploymentFlags)
	buildFlags.AddFlagSet(roflCommon.NoUpdateFlag)
	buildFlags.AddFlagSet(common.VerboseFlag)
	buildFlags.AddFlagSet(common.ForceFlag)

	Cmd.Flags().AddFlagSet(buildFlags)
}
