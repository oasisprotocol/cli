package build

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/sgx"
	"github.com/oasisprotocol/oasis-core/go/common/version"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-core/go/runtime/bundle"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

// Build modes.
const (
	buildModeProduction = "production"
	buildModeUnsafe     = "unsafe"
	buildModeAuto       = "auto"
)

var (
	outputFn       string
	buildMode      string
	offline        bool
	doUpdate       bool
	deploymentName string

	Cmd = &cobra.Command{
		Use:   "build",
		Short: "Build a ROFL application",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			manifest, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, true)

			fmt.Println("Building a ROFL application...")
			fmt.Printf("Deployment: %s\n", deploymentName)
			fmt.Printf("Network:    %s\n", deployment.Network)
			fmt.Printf("ParaTime:   %s\n", deployment.ParaTime)
			fmt.Printf("App ID:     %s\n", deployment.AppID)
			fmt.Printf("Name:       %s\n", manifest.Name)
			fmt.Printf("Version:    %s\n", manifest.Version)
			fmt.Printf("TEE:        %s\n", manifest.TEE)
			fmt.Printf("Kind:       %s\n", manifest.Kind)

			// Prepare temporary build directory.
			tmpDir, err := os.MkdirTemp("", "oasis-build")
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to create temporary build directory: %w", err))
			}
			defer os.RemoveAll(tmpDir)

			bnd := &bundle.Bundle{
				Manifest: &bundle.Manifest{
					Name: deployment.AppID,
					ID:   npa.ParaTime.Namespace(),
				},
			}
			bnd.Manifest.Version, err = version.FromString(manifest.Version)
			if err != nil {
				fmt.Printf("unsupported package version format: %s\n", err)
				return
			}

			// Setup some build environment variables.
			os.Setenv("ROFL_MANIFEST", manifest.SourceFileName())
			os.Setenv("ROFL_DEPLOYMENT", deploymentName)
			os.Setenv("ROFL_DEPLOYMENT_NETWORK", deployment.Network)
			os.Setenv("ROFL_DEPLOYMENT_PARATIME", deployment.ParaTime)
			os.Setenv("ROFL_TMPDIR", tmpDir)

			runScript(manifest, buildRofl.ScriptBuildPre)

			switch manifest.TEE {
			case buildRofl.TEETypeSGX:
				// SGX.
				if manifest.Kind != buildRofl.AppKindRaw {
					fmt.Printf("unsupported app kind for SGX TEE: %s\n", manifest.Kind)
					return
				}

				sgxBuild(npa, manifest, deployment, bnd)
			case buildRofl.TEETypeTDX:
				// TDX.
				switch manifest.Kind {
				case buildRofl.AppKindRaw:
					err = tdxBuildRaw(tmpDir, npa, manifest, deployment, bnd)
				case buildRofl.AppKindContainer:
					err = tdxBuildContainer(tmpDir, npa, manifest, deployment, bnd)
				}
			default:
				fmt.Printf("unsupported TEE kind: %s\n", manifest.TEE)
				return
			}
			if err != nil {
				fmt.Printf("%s\n", err)
				return
			}

			runScript(manifest, buildRofl.ScriptBuildPost)

			// Write the bundle out.
			outFn := fmt.Sprintf("%s.%s.orc", manifest.Name, deploymentName)
			if outputFn != "" {
				outFn = outputFn
			}
			if err = bnd.Write(outFn); err != nil {
				fmt.Printf("failed to write output bundle: %s\n", err)
				return
			}

			fmt.Printf("ROFL app built and bundle written to '%s'.\n", outFn)

			fmt.Println("Computing enclave identity...")

			eids, err := roflCommon.ComputeEnclaveIdentity(bnd, "")
			if err != nil {
				fmt.Printf("%s\n", err)
				return
			}

			// Override the update manifest flag in case the policy does not exist.
			if deployment.Policy == nil {
				doUpdate = false
			}

			switch doUpdate {
			case false:
				// Ask the user to update the manifest manually.
				fmt.Println("Update the manifest with the following identities to use the new app:")
				fmt.Println()
				fmt.Printf("deployments:\n")
				fmt.Printf("  %s:\n", deploymentName)
				fmt.Printf("    policy:\n")
				fmt.Printf("      enclaves:\n")
				for _, enclaveID := range eids {
					data, _ := enclaveID.MarshalText()
					fmt.Printf("        - \"%s\"\n", string(data))
				}
				fmt.Println()
				fmt.Println("Next time you can also use the --update-manifest flag to apply changes.")
			case true:
				// Update the manifest with the given enclave identities, overwriting existing ones.
				deployment.Policy.Enclaves = make([]sgx.EnclaveIdentity, 0, len(eids))
				for _, eid := range eids {
					deployment.Policy.Enclaves = append(deployment.Policy.Enclaves, *eid)
				}

				if err = manifest.Save(); err != nil {
					cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
				}
			}
		},
	}
)

func detectBuildMode(npa *common.NPASelection) {
	// Configure build mode. In case auto is selected and not offline, query the network. If
	// autodetection fails, default to production mode.
	switch {
	case buildMode == buildModeAuto && !offline:
		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("unable to autodetect build mode, please provide --mode flag manually (failed to connect to GRPC endpoint: %w)", err))
		}

		params, err := conn.Consensus().Registry().ConsensusParameters(ctx, consensus.HeightLatest)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("unable to autodetect build mode, please provide --mode flag manually (failed to get consensus parameters: %w)", err))
		}

		if params.DebugAllowTestRuntimes {
			buildMode = buildModeUnsafe
		}
	default:
	}
}

func setupBuildEnv(deployment *buildRofl.Deployment, npa *common.NPASelection) {
	// Configure app ID.
	os.Setenv("ROFL_APP_ID", deployment.AppID)

	// Obtain and configure trust root.
	trustRoot, err := fetchTrustRoot(npa, deployment.TrustRoot)
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
			height, err = common.GetActualHeight(ctx, conn.Consensus())
			if err != nil {
				return "", err
			}
		default:
			// Use configured height.
			height = int64(cfg.Height)
		}

		blk, err := conn.Consensus().GetBlock(ctx, height)
		if err != nil {
			return "", err
		}
		hash = blk.Hash.Hex()
	default:
		// Hash is known, just use it.
		height = int64(cfg.Height)
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
		Height:       uint64(height),
		Hash:         hash,
		RuntimeID:    npa.ParaTime.Namespace(),
		ChainContext: npa.Network.ChainContext,
	}
	encRoot := cbor.Marshal(root)
	return base64.StdEncoding.EncodeToString(encRoot), nil
}

func init() {
	buildFlags := flag.NewFlagSet("", flag.ContinueOnError)
	buildFlags.StringVar(&buildMode, "mode", "auto", "build mode [production, unsafe, auto]")
	buildFlags.BoolVar(&offline, "offline", false, "do not perform any operations requiring network access")
	buildFlags.StringVar(&outputFn, "output", "", "output bundle filename")
	buildFlags.BoolVar(&doUpdate, "update-manifest", false, "automatically update the manifest")
	buildFlags.StringVar(&deploymentName, "deployment", buildRofl.DefaultDeploymentName, "deployment name")

	Cmd.Flags().AddFlagSet(buildFlags)
}
