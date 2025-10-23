package rofl

import (
	"bufio"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/sgx/pcs"
	"github.com/oasisprotocol/oasis-core/go/common/sgx/quote"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"

	buildDotenv "github.com/oasisprotocol/cli/build/dotenv"
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

	scheme  string
	pubName string

	appTEE  string
	appKind string
	reset   bool

	//go:embed init_artifacts/compose.yaml
	initArtifactCompose []byte

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

			var manifest *buildRofl.Manifest
			if !reset {
				// Fail in case there is an existing manifest.
				if buildRofl.ManifestExists() {
					cobra.CheckErr("Refusing to overwrite existing manifest.\nHint: To reset existing ROFL manifest, pass --reset flag.")
				}

				fmt.Printf("Creating a new app with default policy...\n")

				// Create a default manifest without any deployments.
				manifest = &buildRofl.Manifest{
					Name:    appName,
					Version: "0.1.0",
					TEE:     appTEE,
					Kind:    appKind,
					Resources: buildRofl.ResourcesConfig{
						Memory:   512,
						CPUCount: 1,
					},
				}
			} else {
				manifest, err = buildRofl.LoadManifest()
				cobra.CheckErr(err)

				fmt.Printf("\n")
				if !common.GetAnswerYes() {
					common.Confirm("Reset existing app manifest file by removing all configured ROFL deployments, secrets and policies", "not resetting")
				}

				manifest.Name = appName
				manifest.Deployments = make(map[string]*buildRofl.Deployment)
			}

			// TODO: Extract author and repository from Git configuration if available.

			err = manifest.Validate()
			cobra.CheckErr(err)

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
					artifacts := buildRofl.LatestContainerArtifacts // Copy.
					artifacts.Container.Compose = detectOrCreateComposeFile()
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

			// Check if we're inside an existing git repository.
			var gitRepoExists bool
			gitRevParseCmd := exec.Command("git", "rev-parse")
			if err = gitRevParseCmd.Run(); err == nil {
				gitRepoExists = true
			}

			if !gitRepoExists {
				// Initialize a new git repository in the app directory.
				gitInitCmd := exec.Command("git", "init")
				if err = gitInitCmd.Run(); err != nil {
					fmt.Printf("Git repository not initialized: %s.\n", err)
				} else {
					fmt.Printf("Git repository initialized.\n")
				}
			}

			// Initialize .gitignore in the app directory.
			needToAddOrcIgnore := true
			gitignore, err := os.Open(".gitignore")
			if err == nil {
				s := bufio.NewScanner(gitignore)
				for s.Scan() {
					line := s.Text()
					if line == "*.orc" {
						needToAddOrcIgnore = false
						break
					}
				}
				_ = gitignore.Close()
			}
			if needToAddOrcIgnore {
				gitignore, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				if err == nil {
					_, _ = gitignore.Write([]byte("*.orc\n"))
					_ = gitignore.Close()
				}
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
			deployment, ok := manifest.Deployments[roflCommon.DeploymentName]
			switch ok {
			case true:
				if deployment.AppID != "" {
					cobra.CheckErr(fmt.Errorf("ROFL app identifier already defined (%s) for deployment '%s', refusing to overwrite", deployment.AppID, roflCommon.DeploymentName))
				}

				// An existing deployment is defined, but without an AppID. Load everything else for
				// the deployment and proceed with creating a new app.
				manifest, deployment, npa = roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
					NeedAppID: false,
					NeedAdmin: true,
				})
			case false:
				// No deployment defined, create a new default one.
				npa.MustHaveAccount()
				npa.MustHaveParaTime()
				if txCfg.Offline {
					cobra.CheckErr("offline mode currently not supported")
				}

				// Determine latest height for the trust root.
				var height int64
				height, err = common.GetActualHeight(ctx, conn.Consensus().Core())
				cobra.CheckErr(err)

				var blk *consensus.Block
				blk, err = conn.Consensus().Core().GetBlock(ctx, height)
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
					Policy: &buildRofl.AppAuthPolicy{
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
				manifest.Deployments[roflCommon.DeploymentName] = deployment
			}

			idScheme, ok := identifierSchemes[scheme]
			if !ok {
				cobra.CheckErr(fmt.Errorf("unknown scheme %s", scheme))
			}

			// Prepare transaction.
			tx := rofl.NewCreateTx(nil, &rofl.Create{
				Policy:   *deployment.Policy.AsDescriptor(),
				Scheme:   idScheme,
				Metadata: manifest.GetMetadata(roflCommon.DeploymentName),
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			var appID rofl.AppID
			if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, &appID) {
				return
			}

			fmt.Printf("Created ROFL app: %s\n", appID)

			// Update the manifest with the given enclave identities, overwriting existing ones.
			deployment.AppID = appID.String()

			if !roflCommon.NoUpdate {
				if err = manifest.Save(); err != nil {
					cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
				}
			}

			fmt.Printf("Run `oasis rofl build` to build your ROFL app.\n")
		},
	}

	updateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update an existing ROFL application",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			txCfg := common.GetTransactionConfig()

			manifest, deployment, npa := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: true,
			})

			secrets := buildRofl.PrepareSecrets(deployment.Secrets)

			var appID rofl.AppID
			if err := appID.UnmarshalText([]byte(deployment.AppID)); err != nil {
				cobra.CheckErr(fmt.Errorf("malformed ROFL app ID: %w", err))
			}

			npa.MustHaveAccount()
			npa.MustHaveParaTime()

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
				Policy:   *deployment.Policy.AsDescriptor(),
				Metadata: manifest.GetMetadata(roflCommon.DeploymentName),
				Secrets:  secrets,
			}

			// Update administrator address.
			var err error
			updateBody.Admin, _, err = common.ResolveLocalAccountOrAddress(npa.Network, deployment.Admin)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("bad administrator address: %w", err))
			}

			// Prepare transaction.
			tx := rofl.NewUpdateTx(nil, &updateBody)

			acc := common.LoadAccount(cliConfig.Global(), npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil)
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

			var (
				rawAppID   string
				manifest   *buildRofl.Manifest
				deployment *buildRofl.Deployment
			)
			if len(args) > 0 {
				rawAppID = args[0]
			} else {
				manifest, deployment, npa = roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
					NeedAppID: true,
					NeedAdmin: true,
				})
				rawAppID = deployment.AppID
			}
			var appID rofl.AppID
			if err := appID.UnmarshalText([]byte(rawAppID)); err != nil {
				cobra.CheckErr(fmt.Errorf("malformed ROFL app ID: %w", err))
			}

			npa.MustHaveAccount()
			npa.MustHaveParaTime()

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			fmt.Printf("WARNING: Removing this ROFL app will DEREGISTER it, ERASE any on-chain secrets and local configuration!\n")
			fmt.Printf("WARNING: THIS ACTION IS IRREVERSIBLE!\n")
			if !common.GetAnswerYes() {
				common.Confirm(fmt.Sprintf("Remove ROFL app '%s' deployed on network '%s'", appID, npa.NetworkName), "not removing")
			}

			// Prepare transaction.
			tx := rofl.NewRemoveTx(nil, &rofl.Remove{
				ID: appID,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil)

			// Update manifest to clear the corresponding deployment section.
			if manifest != nil {
				delete(manifest.Deployments, roflCommon.DeploymentName)
				if err = manifest.Save(); err != nil {
					cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
				}
			}
		},
	}

	showCmd = &cobra.Command{
		Use:   "show [<app-id>]",
		Short: "Show information about a ROFL application",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			var rawAppID string
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			if len(args) > 0 {
				rawAppID = args[0]
			} else {
				var deployment *buildRofl.Deployment
				_, deployment, npa = roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
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

			replicas, err := conn.Runtime(npa.ParaTime).ROFL.AppInstances(ctx, client.RoundLatest, appID)
			cobra.CheckErr(err)

			if common.OutputFormat() == common.FormatJSON {
				output := struct {
					App      common.JSONPrettyPrintRoflAppConfig      `json:"app"`
					Replicas []common.JSONPrettyPrintRoflRegistration `json:"replicas"`
				}{
					App:      common.JSONPrettyPrintRoflAppConfig(*appCfg),
					Replicas: make([]common.JSONPrettyPrintRoflRegistration, 0, len(replicas)),
				}
				// Convert replicas to pretty-printable format.
				for _, ai := range replicas {
					output.Replicas = append(output.Replicas, common.JSONPrettyPrintRoflRegistration(*ai))
				}

				jsonOutput, err := common.PrettyJSONMarshal(output)
				cobra.CheckErr(err)
				fmt.Println(string(jsonOutput))
				return
			}

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
			fmt.Printf("=== Replicas ===\n")

			if len(replicas) > 0 {
				for _, ai := range replicas {
					fmt.Printf("- RAK:        %s\n", ai.RAK)
					fmt.Printf("  Node ID:    %s\n", ai.NodeID)
					fmt.Printf("  Expiration: %d\n", ai.Expiration)
					if len(ai.Metadata) > 0 {
						fmt.Printf("  Metadata:\n")
						for key, value := range ai.Metadata {
							fmt.Printf("    %s: %s\n", key, value)
						}
					}
				}
			} else {
				fmt.Println("No registered replicas.")
			}
		},
	}

	upgradeCmd = &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade all artifacts to their latest default versions",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			manifest, err := buildRofl.LoadManifest()
			cobra.CheckErr(err)

			var latestArtifacts buildRofl.ArtifactsConfig
			switch manifest.TEE {
			case buildRofl.TEETypeTDX:
				switch manifest.Kind {
				case buildRofl.AppKindRaw:
					latestArtifacts = buildRofl.LatestBasicArtifacts // Copy.
				case buildRofl.AppKindContainer:
					latestArtifacts = buildRofl.LatestContainerArtifacts // Copy.
				default:
				}
			default:
			}

			if !manifest.Artifacts.UpgradeTo(&latestArtifacts) {
				fmt.Printf("Artifacts already up-to-date.\n")
				return
			}

			// Update manifest.
			if err := manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}

			fmt.Printf("Run `oasis rofl build` to build your ROFL app.\n")
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
			secretName := args[0]
			secretFn := args[1]

			manifest, deployment, npa := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
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

			var replaced bool
			for i, sc := range deployment.Secrets {
				if sc.Name == secretName {
					common.CheckForceErr(fmt.Errorf("the secret named '%s' for deployment '%s' already exists", secretName, roflCommon.DeploymentName))
					deployment.Secrets[i] = &secretCfg
					replaced = true
					break
				}
			}
			if !replaced {
				deployment.Secrets = append(deployment.Secrets, &secretCfg)
			}

			// Update manifest.
			if err = manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}

			fmt.Printf("Run `oasis rofl update` to update your ROFL app's on-chain configuration.\n")
		},
	}

	// secretImportCmd bulk-imports secrets from a .env file (key=value with # comments).
	// Supports '-' to read from stdin. Existing secrets are replaced only with --force.
	secretImportCmd = &cobra.Command{
		Use:   "import <dot-env-file>|-",
		Short: "Import multiple secrets from a .env file",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			envFn := args[0]

			manifest, deployment, npa := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: false,
			})
			var appID rofl.AppID
			if err := appID.UnmarshalText([]byte(deployment.AppID)); err != nil {
				cobra.CheckErr(fmt.Errorf("malformed ROFL app ID: %w", err))
			}

			// Establish connection with the target network to obtain SEK.
			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			appCfg, err := conn.Runtime(npa.ParaTime).ROFL.App(ctx, client.RoundLatest, appID)
			cobra.CheckErr(err)

			// Read .env data (supports stdin via "-").
			var raw []byte
			if envFn == "-" {
				raw, err = io.ReadAll(os.Stdin)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to read .env from standard input: %w", err))
				}
			} else {
				raw, err = os.ReadFile(envFn)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to read .env file: %w", err))
				}
			}

			entries, err := buildDotenv.Parse(string(raw))
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to parse .env: %w", err))
			}
			if len(entries) == 0 {
				fmt.Println("No key=value pairs found in the provided .env input.")
				return
			}

			// Deterministic iteration for stable updates.
			names := make([]string, 0, len(entries))
			for k := range entries {
				names = append(names, k)
			}
			sort.Strings(names)

			var imported, updated int
			for _, name := range names {
				value := []byte(entries[name])

				encValue, err := buildRofl.EncryptSecret(name, value, appCfg.SEK)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to encrypt secret %s: %w", name, err))
				}

				secretCfg := buildRofl.SecretConfig{
					Name:  name,
					Value: encValue,
				}

				var replaced bool
				for i, sc := range deployment.Secrets {
					if sc.Name == name {
						common.CheckForceErr(fmt.Errorf("the secret named '%s' for deployment '%s' already exists", name, roflCommon.DeploymentName))
						// Preserve any existing public name unless we add a way to override it via import flags.
						if sc.PublicName != "" && secretCfg.PublicName == "" {
							secretCfg.PublicName = sc.PublicName
						}
						deployment.Secrets[i] = &secretCfg
						replaced = true
						updated++
						break
					}
				}
				if !replaced {
					deployment.Secrets = append(deployment.Secrets, &secretCfg)
					imported++
				}
			}

			// Update manifest.
			if err = manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}

			src := envFn
			if envFn == "-" {
				src = "stdin"
			}
			fmt.Printf("Imported %d secrets, updated %d existing from '%s'.\n", imported, updated, src)
			fmt.Printf("Run `oasis rofl update` to update your ROFL app's on-chain configuration.\n")
		},
	}

	secretGetCmd = &cobra.Command{
		Use:   "get <name>",
		Short: "Show metadata about the given secret",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			secretName := args[0]

			_, deployment, _ := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
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
				cobra.CheckErr(fmt.Errorf("secret named '%s' does not exist for deployment '%s'", secretName, roflCommon.DeploymentName))
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
			secretName := args[0]

			manifest, deployment, _ := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
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
				cobra.CheckErr(fmt.Errorf("secret named '%s' does not exist for deployment '%s'", secretName, roflCommon.DeploymentName))
			}
			deployment.Secrets = newSecrets

			// Update manifest.
			if err := manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}

			fmt.Printf("Run `oasis rofl update` to update your ROFL app's on-chain configuration.\n")
		},
	}
)

// detectOrCreateComposeFile detects the existing compose.yaml-like file and returns its filename. If it doesn't exist, it creates compose.yaml and populates it.
func detectOrCreateComposeFile() string {
	for _, filename := range []string{"rofl-compose.yaml", "rofl-compose.yml", "docker-compose.yaml", "docker-compose.yml", "compose.yml", "compose.yaml"} {
		if _, err := os.Stat(filename); err == nil {
			return filename
		}
	}

	filename := "compose.yaml"
	if err := os.WriteFile(filename, initArtifactCompose, 0o644); err != nil { //nolint: gosec
		return ""
	}

	return filename
}

func init() {
	initCmd.Flags().StringVar(&appTEE, "tee", "tdx", "TEE kind [tdx, sgx]")
	initCmd.Flags().StringVar(&appKind, "kind", "container", "ROFL app kind [container, raw]")
	initCmd.Flags().BoolVar(&reset, "reset", false, "reset the existing ROFL manifest")
	initCmd.Flags().AddFlagSet(common.AnswerYesFlag)

	createCmd.Flags().AddFlagSet(common.SelectorFlags)
	createCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	createCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	createCmd.Flags().AddFlagSet(roflCommon.NoUpdateFlag)
	createCmd.Flags().StringVar(&scheme, "scheme", "cn", "app ID generation scheme: creator+round+index [cri] or creator+nonce [cn]")

	updateCmd.Flags().AddFlagSet(common.AccountFlag)
	updateCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	updateCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)

	removeCmd.Flags().AddFlagSet(common.SelectorFlags)
	removeCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	removeCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)

	showCmd.Flags().AddFlagSet(common.SelectorFlags)
	showCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	showCmd.Flags().AddFlagSet(common.FormatFlag)

	secretSetCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	secretSetCmd.Flags().StringVar(&pubName, "public-name", "", "public secret name")
	secretSetCmd.Flags().AddFlagSet(common.ForceFlag)
	secretCmd.AddCommand(secretSetCmd)

	// secret import flags and registration.
	secretImportCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	secretImportCmd.Flags().AddFlagSet(common.ForceFlag)
	secretCmd.AddCommand(secretImportCmd)

	secretGetCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	secretCmd.AddCommand(secretGetCmd)

	secretRmCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	secretCmd.AddCommand(secretRmCmd)
}
