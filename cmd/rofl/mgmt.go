package rofl

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"gopkg.in/yaml.v3"

	"github.com/oasisprotocol/oasis-core/go/common/sgx/pcs"
	"github.com/oasisprotocol/oasis-core/go/common/sgx/quote"
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

	appTEE  string
	appKind string

	initCmd = &cobra.Command{
		Use:   "init <name> [--tee TEE] [--kind KIND]",
		Short: "Create a new ROFL app and initialize the manifest",
		Args:  cobra.ExactArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			if npa.Account == nil {
				cobra.CheckErr("no accounts configured in your wallet")
			}
			if npa.ParaTime == nil {
				cobra.CheckErr("no ParaTime selected")
			}
			if txCfg.Offline {
				cobra.CheckErr("offline mode currently not supported")
			}

			// TODO: Support an interactive mode.
			appName := args[0]
			// Fail in case there is an existing manifest.
			if buildRofl.ManifestExists() {
				cobra.CheckErr("refusing to overwrite existing manifest")
			}

			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			// Determine latest height for the trust root.
			height, err := common.GetActualHeight(ctx, conn.Consensus())
			cobra.CheckErr(err)

			// Generate manifest and a default policy which does not accept any enclaves.
			manifest := buildRofl.Manifest{
				AppID:    rofl.NewAppIDGlobalName("").String(), // Temporary for initial validation.
				Name:     appName,
				Version:  "0.1.0",
				Network:  npa.NetworkName,
				ParaTime: npa.ParaTimeName,
				Admin:    npa.AccountName,
				TEE:      appTEE,
				Kind:     appKind,
				Policy: &rofl.AppAuthPolicy{
					Quotes: quote.Policy{
						PCS: &pcs.QuotePolicy{
							TCBValidityPeriod:          30,
							MinTCBEvaluationDataNumber: 17,
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
				},
				Resources: buildRofl.ResourcesConfig{
					Memory:   512,
					CPUCount: 1,
					Storage: &buildRofl.StorageConfig{
						Kind: buildRofl.StorageKindDiskEphemeral,
						Size: 512,
					},
				},
			}
			err = manifest.Validate()
			cobra.CheckErr(err)

			fmt.Printf("Creating a new ROFL app with default policy...\n")
			fmt.Printf("Name:    %s\n", manifest.Name)
			fmt.Printf("Version: %s\n", manifest.Version)
			fmt.Printf("TEE:     %s\n", manifest.TEE)
			fmt.Printf("Kind:    %s\n", manifest.Kind)

			idScheme, ok := identifierSchemes[scheme]
			if !ok {
				cobra.CheckErr(fmt.Errorf("unknown scheme %s", scheme))
			}

			// Register a new ROFL application to determine the identifier.
			tx := rofl.NewCreateTx(nil, &rofl.Create{
				Policy: *manifest.Policy,
				Scheme: idScheme,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			var appID rofl.AppID
			common.BroadcastTransaction(ctx, npa.ParaTime, conn, sigTx, meta, &appID)
			manifest.AppID = appID.String()

			fmt.Printf("Created ROFL application: %s\n", appID)

			// Serialize manifest and write it to file.
			data, _ := yaml.Marshal(manifest)
			if err = os.WriteFile("rofl.yml", data, 0o644); err != nil { //nolint: gosec
				cobra.CheckErr(fmt.Errorf("failed to write manifest: %w", err))
			}
		},
	}

	createCmd = &cobra.Command{
		Use:   "create [<policy.yml>]",
		Short: "Create a new ROFL application",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			var policy *rofl.AppAuthPolicy
			if len(args) > 0 {
				policy = loadPolicy(args[0])
			} else {
				manifest := roflCommon.LoadManifestAndSetNPA(cfg, npa)
				policy = manifest.Policy
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

			idScheme, ok := identifierSchemes[scheme]
			if !ok {
				cobra.CheckErr(fmt.Errorf("unknown scheme %s", scheme))
			}

			// Prepare transaction.
			tx := rofl.NewCreateTx(nil, &rofl.Create{
				Policy: *policy,
				Scheme: idScheme,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			var appID rofl.AppID
			if !common.BroadcastOrExportTransaction(ctx, npa.ParaTime, conn, sigTx, meta, &appID) {
				return
			}

			fmt.Printf("Created ROFL application: %s\n", appID)
		},
	}

	updateCmd = &cobra.Command{
		Use:   "update [<app-id> --policy <policy.yml> --admin <address>]",
		Short: "Update an existing ROFL application",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			var (
				rawAppID string
				policy   *rofl.AppAuthPolicy
			)
			if len(args) > 0 {
				rawAppID = args[0]
				policy = loadPolicy(policyFn)
			} else {
				manifest := roflCommon.LoadManifestAndSetNPA(cfg, npa)
				rawAppID = manifest.AppID

				if adminAddress == "" && manifest.Admin != "" {
					adminAddress = "self"
				}
				policy = manifest.Policy
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

			if adminAddress == "" {
				fmt.Println("You must specify --admin or configure an admin in the manifest.")
				return
			}
			if policy == nil {
				fmt.Println("You must specify --policy or configure policy in the manifest.")
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
				ID:     appID,
				Policy: *policy,
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
				manifest := roflCommon.LoadManifestAndSetNPA(cfg, npa)
				rawAppID = manifest.AppID
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
				manifest := roflCommon.LoadManifestAndSetNPA(cfg, npa)
				rawAppID = manifest.AppID
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
)

func loadPolicy(fn string) *rofl.AppAuthPolicy {
	rawPolicy, err := os.ReadFile(fn)
	cobra.CheckErr(err)

	// Parse policy.
	var policy rofl.AppAuthPolicy
	if err = yaml.Unmarshal(rawPolicy, &policy); err != nil {
		cobra.CheckErr(fmt.Errorf("malformed ROFL app policy: %w", err))
	}
	return &policy
}

func init() {
	updateFlags := flag.NewFlagSet("", flag.ContinueOnError)
	updateFlags.StringVar(&policyFn, "policy", "", "set the ROFL application policy")
	updateFlags.StringVar(&adminAddress, "admin", "", "set the administrator address")

	initCmd.Flags().AddFlagSet(common.SelectorFlags)
	initCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	initCmd.Flags().StringVar(&appTEE, "tee", "tdx", "TEE kind [tdx, sgx]")
	initCmd.Flags().StringVar(&appKind, "kind", "container", "ROFL app kind [container, raw]")
	initCmd.Flags().StringVar(&scheme, "scheme", "cn", "app ID generation scheme: creator+round+index [cri] or creator+nonce [cn]")

	createCmd.Flags().AddFlagSet(common.SelectorFlags)
	createCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	createCmd.Flags().StringVar(&scheme, "scheme", "cn", "app ID generation scheme: creator+round+index [cri] or creator+nonce [cn]")

	updateCmd.Flags().AddFlagSet(common.SelectorFlags)
	updateCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	updateCmd.Flags().AddFlagSet(updateFlags)

	removeCmd.Flags().AddFlagSet(common.SelectorFlags)
	removeCmd.Flags().AddFlagSet(common.RuntimeTxFlags)

	showCmd.Flags().AddFlagSet(common.SelectorFlags)
}
