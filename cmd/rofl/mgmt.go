package rofl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/common/sgx/pcs"
	"github.com/oasisprotocol/oasis-core/go/common/sgx/quote"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	identifierSchemes = map[string]rofl.IdentifierScheme{
		"cri": rofl.CreatorRoundIndex,
		"cn":  rofl.CreatorNonce,
	}

	policyFn     string
	scheme       string
	adminAddress string
	pubName      string

	appTEE         string
	appKind        string
	deploymentName string

	initCmd = &cobra.Command{
		Use:   "init [<name>] [--tee TEE] [--kind KIND]",
		Short: "Initialize a ROFL app manifest",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			// Determine the application directory.
			appPath := "."
			if len(args) > 0 {
				appPath = args[0]
			}
			appPath, err := filepath.Abs(appPath)
			cobra.CheckErr(err)
			appName := filepath.Base(appPath)
			if err = os.MkdirAll(appPath, 0o755); err != nil {
				cobra.CheckErr(err)
			}
			err = os.Chdir(appPath)
			cobra.CheckErr(err)

			// Fail in case there is an existing manifest.
			if buildRofl.ManifestExists() {
				cobra.CheckErr("refusing to overwrite existing manifest")
			}

			// Create a default manifest without any deployments.
			manifest := buildRofl.Manifest{
				Name:    appName,
				Version: "0.1.0",
				TEE:     appTEE,
				Kind:    appKind,
				Resources: buildRofl.ResourcesConfig{
					Memory:   512,
					CPUCount: 1,
					Storage: &buildRofl.StorageConfig{
						Kind: buildRofl.StorageKindDiskPersistent,
						Size: 512,
					},
				},
			}
			err = manifest.Validate()
			cobra.CheckErr(err)

			fmt.Printf("Creating a new ROFL app with default policy...\n")
			fmt.Printf("Name:     %s\n", manifest.Name)
			fmt.Printf("Version:  %s\n", manifest.Version)
			fmt.Printf("TEE:      %s\n", manifest.TEE)
			fmt.Printf("Kind:     %s\n", manifest.Kind)

			switch manifest.TEE {
			case buildRofl.TEETypeTDX:
				switch appKind {
				case buildRofl.AppKindRaw:
					artifacts := buildRofl.LatestBasicArtifacts // Copy.
					manifest.Artifacts = &artifacts
				case buildRofl.AppKindContainer:
					// For container app kind also create an en empty compose.yaml file if it doesn't exist.
					var f *os.File
					f, err = os.OpenFile("compose.yaml", os.O_RDONLY|os.O_CREATE, 0o644)
					if err == nil {
						f.Close()
					}

					artifacts := buildRofl.LatestContainerArtifacts // Copy.
					manifest.Artifacts = &artifacts
				default:
				}
			default:
			}

			// Serialize manifest and write it to file.
			err = manifest.Save()
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to write manifest: %w", err))
			}

			fmt.Printf("Created manifest in '%s'.\n", manifest.SourceFileName())
			fmt.Printf("Run `oasis rofl create` to register your ROFL app and configure an app ID.\n")
		},
	}

	createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new ROFL application",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			manifest, err := buildRofl.LoadManifest()
			cobra.CheckErr(err)

			// Load or create a deployment.
			deployment, ok := manifest.Deployments[deploymentName]
			switch ok {
			case true:
				if deployment.AppID != "" {
					cobra.CheckErr(fmt.Errorf("ROFL app identifier already defined (%s) for deployment '%s', refusing to overwrite", deployment.AppID, deploymentName))
				}

				// An existing deployment is defined, but without an AppID. Load everything else for
				// the deployment and proceed with creating a new app.
				manifest, deployment = roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
					NeedAppID: false,
					NeedAdmin: true,
				})
			case false:
				// No deployment defined, create a new default one.
				if npa.Account == nil {
					cobra.CheckErr("no accounts configured in your wallet")
				}
				if npa.ParaTime == nil {
					cobra.CheckErr("no ParaTime selected")
				}
				if txCfg.Offline {
					cobra.CheckErr("offline mode currently not supported")
				}

				// Determine latest height for the trust root.
				var height int64
				height, err = common.GetActualHeight(ctx, conn.Consensus())
				cobra.CheckErr(err)

				var blk *consensus.Block
				blk, err = conn.Consensus().GetBlock(ctx, height)
				cobra.CheckErr(err)

				// Determine debug mode.
				var (
					debugMode bool
					params    *registry.ConsensusParameters
				)
				params, err = conn.Consensus().Registry().ConsensusParameters(ctx, height)
				if err == nil {
					debugMode = params.DebugAllowTestRuntimes
				}

				// Generate manifest and a default policy which does not accept any enclaves.
				deployment = &buildRofl.Deployment{
					Network:  npa.NetworkName,
					ParaTime: npa.ParaTimeName,
					Admin:    npa.AccountName,
					Debug:    debugMode,
					Policy: &rofl.AppAuthPolicy{
						Quotes: quote.Policy{
							PCS: &pcs.QuotePolicy{
								TCBValidityPeriod:          30,
								MinTCBEvaluationDataNumber: 18,
								TDX:                        &pcs.TdxQuotePolicy{},
							},
						},
						Endorsements: []rofl.AllowedEndorsement{
							{Any: &struct{}{}},
						},
						Fees:          rofl.FeePolicyEndorsingNodePays,
						MaxExpiration: 3,
					},
					TrustRoot: &buildRofl.TrustRootConfig{
						Height: uint64(height),
						Hash:   blk.Hash.Hex(),
					},
				}
				if manifest.Deployments == nil {
					manifest.Deployments = make(map[string]*buildRofl.Deployment)
				}
				manifest.Deployments[deploymentName] = deployment
			}

			idScheme, ok := identifierSchemes[scheme]
			if !ok {
				cobra.CheckErr(fmt.Errorf("unknown scheme %s", scheme))
			}

			// Prepare transaction.
			tx := rofl.NewCreateTx(nil, &rofl.Create{
				Policy: *deployment.Policy,
				Scheme: idScheme,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			var appID rofl.AppID
			if !common.BroadcastOrExportTransaction(ctx, npa.ParaTime, conn, sigTx, meta, &appID) {
				return
			}

			fmt.Printf("Created ROFL app: %s\n", appID)

			// Update the manifest with the given enclave identities, overwriting existing ones.
			deployment.AppID = appID.String()

			if err = manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}

			fmt.Printf("Run `oasis rofl build --update-manifest` to build your ROFL app.\n")
		},
	}

	updateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update an existing ROFL application",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			_, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: true,
			})

			if adminAddress == "" && deployment.Admin != "" {
				adminAddress = "self"
			}
			secrets := buildRofl.PrepareSecrets(deployment.Secrets)

			var appID rofl.AppID
			if err := appID.UnmarshalText([]byte(deployment.AppID)); err != nil {
				cobra.CheckErr(fmt.Errorf("malformed ROFL app ID: %w", err))
			}

			if npa.Account == nil {
				cobra.CheckErr("no accounts configured in your wallet")
			}
			if npa.ParaTime == nil {
				cobra.CheckErr("no ParaTime selected")
			}

			if adminAddress == "" {
				fmt.Println("You must specify --admin or configure an admin in the manifest.")
				return
			}
			if deployment.Policy == nil {
				fmt.Println("You must configure policy in the manifest.")
				return
			}

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			updateBody := rofl.Update{
				ID:       appID,
				Policy:   *deployment.Policy,
				Metadata: deployment.Metadata,
				Secrets:  secrets,
			}

			// Update administrator address.
			if adminAddress == "self" {
				adminAddress = npa.AccountName
			}
			var err error
			updateBody.Admin, _, err = common.ResolveLocalAccountOrAddress(npa.Network, adminAddress)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("bad administrator address: %w", err))
			}

			// Prepare transaction.
			tx := rofl.NewUpdateTx(nil, &updateBody)

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			common.BroadcastOrExportTransaction(ctx, npa.ParaTime, conn, sigTx, meta, nil)
		},
	}

	removeCmd = &cobra.Command{
		Use:   "remove [<app-id>]",
		Short: "Remove an existing ROFL application",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			var rawAppID string
			if len(args) > 0 {
				rawAppID = args[0]
			} else {
				_, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
					NeedAppID: true,
					NeedAdmin: true,
				})
				rawAppID = deployment.AppID
			}
			var appID rofl.AppID
			if err := appID.UnmarshalText([]byte(rawAppID)); err != nil {
				cobra.CheckErr(fmt.Errorf("malformed ROFL app ID: %w", err))
			}

			if npa.Account == nil {
				cobra.CheckErr("no accounts configured in your wallet")
			}
			if npa.ParaTime == nil {
				cobra.CheckErr("no ParaTime selected")
			}

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			// Prepare transaction.
			tx := rofl.NewRemoveTx(nil, &rofl.Remove{
				ID: appID,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			common.BroadcastOrExportTransaction(ctx, npa.ParaTime, conn, sigTx, meta, nil)
		},
	}

	showCmd = &cobra.Command{
		Use:   "show [<app-id>]",
		Short: "Show information about a ROFL application",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)

			var rawAppID string
			if len(args) > 0 {
				rawAppID = args[0]
			} else {
				_, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
					NeedAppID: true,
					NeedAdmin: false,
				})
				rawAppID = deployment.AppID
			}
			var appID rofl.AppID
			if err := appID.UnmarshalText([]byte(rawAppID)); err != nil {
				cobra.CheckErr(fmt.Errorf("malformed ROFL app ID: %w", err))
			}

			// Establish connection with the target network.
			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			appCfg, err := conn.Runtime(npa.ParaTime).ROFL.App(ctx, client.RoundLatest, appID)
			cobra.CheckErr(err)

			fmt.Printf("App ID:        %s\n", appCfg.ID)
			fmt.Printf("Admin:         ")
			switch appCfg.Admin {
			case nil:
				fmt.Printf("none\n")
			default:
				fmt.Printf("%s\n", *appCfg.Admin)
			}
			stakedAmnt := helpers.FormatParaTimeDenomination(npa.ParaTime, appCfg.Stake)
			fmt.Printf("Staked amount: %s\n", stakedAmnt)

			if len(appCfg.Metadata) > 0 {
				fmt.Printf("Metadata:\n")
				for key, value := range appCfg.Metadata {
					fmt.Printf("  %s: %s\n", key, value)
				}
			}

			if len(appCfg.Secrets) > 0 {
				fmt.Printf("Secrets:\n")
				for key, value := range appCfg.Secrets {
					fmt.Printf("  %s: [%d bytes]\n", key, len(value))
				}
			}

			fmt.Printf("Policy:\n")
			policyJSON, _ := json.MarshalIndent(appCfg.Policy, "  ", "  ")
			fmt.Printf("  %s\n", string(policyJSON))

			fmt.Println()
			fmt.Printf("=== Instances ===\n")

			appInstances, err := conn.Runtime(npa.ParaTime).ROFL.AppInstances(ctx, client.RoundLatest, appID)
			cobra.CheckErr(err)

			if len(appInstances) > 0 {
				for _, ai := range appInstances {
					fmt.Printf("- RAK:        %s\n", ai.RAK)
					fmt.Printf("  Node ID:    %s\n", ai.NodeID)
					fmt.Printf("  Expiration: %d\n", ai.Expiration)
				}
			} else {
				fmt.Println("No registered app instances.")
			}
		},
	}

	upgradeCmd = &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade all artifacts to their latest default versions",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)

			manifest, _ := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
				NeedAppID: false,
				NeedAdmin: false,
			})

			switch manifest.TEE {
			case buildRofl.TEETypeTDX:
				switch manifest.Kind {
				case buildRofl.AppKindRaw:
					artifacts := buildRofl.LatestBasicArtifacts // Copy.
					manifest.Artifacts = &artifacts
				case buildRofl.AppKindContainer:
					artifacts := buildRofl.LatestContainerArtifacts // Copy.
					manifest.Artifacts = &artifacts
				default:
				}
			default:
			}

			// Update manifest.
			if err := manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}
		},
	}

	secretCmd = &cobra.Command{
		Use:   "secret",
		Short: "Encrypted secret management commands",
	}

	secretSetCmd = &cobra.Command{
		Use:   "set <name> <file>|- [--public-name <public-name>]",
		Short: "Encrypt the given secret into the manifest, reading the value from file or stdin",
		Args:  cobra.ExactArgs(2),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			secretName := args[0]
			secretFn := args[1]

			manifest, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: false,
			})
			var appID rofl.AppID
			if err := appID.UnmarshalText([]byte(deployment.AppID)); err != nil {
				cobra.CheckErr(fmt.Errorf("malformed ROFL app ID: %w", err))
			}

			// Establish connection with the target network.
			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			appCfg, err := conn.Runtime(npa.ParaTime).ROFL.App(ctx, client.RoundLatest, appID)
			cobra.CheckErr(err)

			// Read secret.
			var secretValue []byte
			if secretFn == "-" {
				secretValue, err = io.ReadAll(os.Stdin)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to read secrets from standard input: %w", err))
				}
			} else {
				secretValue, err = os.ReadFile(secretFn)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to read secrets from file: %w", err))
				}
			}

			// Encrypt the secret.
			encValue, err := buildRofl.EncryptSecret(secretName, secretValue, appCfg.SEK)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to encrypt secret: %w", err))
			}

			secretCfg := buildRofl.SecretConfig{
				Name:  secretName,
				Value: encValue,
			}
			if pubName != "" {
				secretCfg.PublicName = pubName
			}
			for _, sc := range deployment.Secrets {
				if sc.Name == secretName {
					cobra.CheckErr(fmt.Errorf("secret named '%s' already exists for deployment '%s'", secretName, deploymentName))
				}
			}
			deployment.Secrets = append(deployment.Secrets, &secretCfg)

			// Update manifest.
			if err = manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}

			fmt.Printf("Run `oasis rofl update` to update your ROFL app's on-chain configuration.\n")
		},
	}

	secretGetCmd = &cobra.Command{
		Use:   "get <name>",
		Short: "Show metadata about the given secret",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			secretName := args[0]

			_, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
				NeedAppID: false,
				NeedAdmin: false,
			})
			var secret *buildRofl.SecretConfig
			for _, sc := range deployment.Secrets {
				if sc.Name != secretName {
					continue
				}
				secret = sc
				break
			}
			if secret == nil {
				cobra.CheckErr(fmt.Errorf("secret named '%s' does not exist for deployment '%s'", secretName, deploymentName))
				return // Lint doesn't know that cobra.CheckErr never returns.
			}

			fmt.Printf("Name:        %s\n", secret.Name)
			if secret.PublicName != "" {
				fmt.Printf("Public name: %s\n", secret.PublicName)
			}
			fmt.Printf("Size:        %d bytes\n", len(secret.Value))
		},
	}

	secretRmCmd = &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove the given secret from the manifest",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			secretName := args[0]

			manifest, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
				NeedAppID: false,
				NeedAdmin: false,
			})

			var (
				newSecrets []*buildRofl.SecretConfig
				found      bool
			)
			for _, sc := range deployment.Secrets {
				if sc.Name == secretName {
					found = true
					continue
				}
				newSecrets = append(newSecrets, sc)
			}
			if !found {
				cobra.CheckErr(fmt.Errorf("secret named '%s' does not exist for deployment '%s'", secretName, deploymentName))
			}
			deployment.Secrets = newSecrets

			// Update manifest.
			if err := manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}
		},
	}
)

func init() {
	deploymentFlags := flag.NewFlagSet("", flag.ContinueOnError)
	deploymentFlags.StringVar(&deploymentName, "deployment", buildRofl.DefaultDeploymentName, "deployment name")

	updateFlags := flag.NewFlagSet("", flag.ContinueOnError)
	updateFlags.StringVar(&policyFn, "policy", "", "set the ROFL application policy")
	updateFlags.StringVar(&adminAddress, "admin", "", "set the administrator address")
	updateCmd.Flags().AddFlagSet(deploymentFlags)

	initCmd.Flags().StringVar(&appTEE, "tee", "tdx", "TEE kind [tdx, sgx]")
	initCmd.Flags().StringVar(&appKind, "kind", "container", "ROFL app kind [container, raw]")

	createCmd.Flags().AddFlagSet(common.SelectorFlags)
	createCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	createCmd.Flags().AddFlagSet(deploymentFlags)
	createCmd.Flags().StringVar(&scheme, "scheme", "cn", "app ID generation scheme: creator+round+index [cri] or creator+nonce [cn]")

	updateCmd.Flags().AddFlagSet(common.SelectorFlags)
	updateCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	updateCmd.Flags().AddFlagSet(deploymentFlags)
	updateCmd.Flags().AddFlagSet(updateFlags)

	removeCmd.Flags().AddFlagSet(common.SelectorFlags)
	removeCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	removeCmd.Flags().AddFlagSet(deploymentFlags)

	showCmd.Flags().AddFlagSet(common.SelectorFlags)
	showCmd.Flags().AddFlagSet(deploymentFlags)

	secretSetCmd.Flags().AddFlagSet(deploymentFlags)
	secretSetCmd.Flags().StringVar(&pubName, "public-name", "", "public secret name")
	secretCmd.AddCommand(secretSetCmd)

	secretGetCmd.Flags().AddFlagSet(deploymentFlags)
	secretCmd.AddCommand(secretGetCmd)

	secretRmCmd.Flags().AddFlagSet(deploymentFlags)
	secretCmd.AddCommand(secretRmCmd)
}
