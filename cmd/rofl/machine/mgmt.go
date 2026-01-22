package machine

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	buildRoflProvider "github.com/oasisprotocol/cli/build/rofl/provider"
	"github.com/oasisprotocol/cli/build/rofl/scheduler"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCmdBuild "github.com/oasisprotocol/cli/cmd/rofl/build"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	restartCmd = &cobra.Command{
		Use:   "restart [<machine-name> | <provider-address>:<machine-id>]",
		Short: "Restart a running machine or start a stopped one",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			queueCommand(
				args,
				scheduler.MethodRestart,
				scheduler.RestartRequest{
					WipeStorage: roflCommon.WipeStorage,
				},
				"Machine restart scheduled.",
			)
		},
	}

	stopCmd = &cobra.Command{
		Use:     "stop [<machine-name> | <provider-address>:<machine-id>]",
		Short:   "Stop a machine",
		Aliases: []string{"terminate"},
		Args:    cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			queueCommand(
				args,
				scheduler.MethodTerminate,
				scheduler.TerminateRequest{
					WipeStorage: roflCommon.WipeStorage,
				},
				"Machine termination scheduled.",
			)
		},
	}

	removeCmd = &cobra.Command{
		Use:     "remove [<machine-name> | <provider-address>:<machine-id>]",
		Short:   "Cancel rental and remove the machine",
		Aliases: []string{"cancel", "rm"},
		Args:    cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			mCfg, err := resolveMachineCfg(args, &roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: false,
			})
			cobra.CheckErr(err)

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !common.GetTransactionConfig().Offline {
				conn, err = connection.Connect(ctx, mCfg.NPA.Network)
				cobra.CheckErr(err)
			}

			fmt.Printf("Using provider:     %s (%s)\n", mCfg.Machine.Provider, mCfg.ProviderAddr)
			fmt.Printf("Canceling machine:  %s [%s]\n", mCfg.MachineName, mCfg.Machine.ID)
			common.Warn("WARNING: Canceling a machine will permanently destroy it including any persistent storage!")
			roflCommon.PrintRentRefundWarning()

			// Prepare transaction.
			tx := roflmarket.NewInstanceCancelTx(nil, &roflmarket.InstanceCancel{
				Provider: *mCfg.ProviderAddr,
				ID:       mCfg.MachineID,
			})

			acc := common.LoadAccount(cliConfig.Global(), mCfg.NPA.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, mCfg.NPA, acc, conn, tx, nil)
			cobra.CheckErr(err)

			if !common.BroadcastOrExportTransaction(ctx, mCfg.NPA, conn, sigTx, meta, nil) {
				return
			}

			fmt.Printf("Machine removed.\n")

			// Update manifest to clear the machine ID as it has been cancelled.
			mCfg.Machine.ID = ""

			if mCfg.Manifest != nil {
				if err = mCfg.Manifest.Save(); err != nil {
					cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
				}
			}
		},
	}

	changeAdminCmd = &cobra.Command{
		Use:   "change-admin [<machine-name> | <provider-address>:<machine-id>] <new-admin>",
		Short: "Change the machine administrator",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(_ *cobra.Command, args []string) {
			txCfg := common.GetTransactionConfig()
			mCfg, err := resolveMachineCfg(args, &roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: false,
			})
			cobra.CheckErr(err)

			var newAdminAddress string
			switch len(args) {
			case 1:
				// Just admin address.
				newAdminAddress = args[0]
			case 2:
				// Machine and admin address.
				newAdminAddress = args[1]
			}

			// Resolve new admin address.
			newAdminAddr, _, err := common.ResolveLocalAccountOrAddress(mCfg.NPA.Network, newAdminAddress)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("invalid admin address: %w", err))
			}

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				conn, err = connection.Connect(ctx, mCfg.NPA.Network)
				cobra.CheckErr(err)
			}

			fmt.Printf("Provider:  %s (%s)\n", mCfg.Machine.Provider, mCfg.ProviderAddr)
			fmt.Printf("Machine:   %s [%s]\n", mCfg.MachineName, mCfg.Machine.ID)

			// Resolve old admin in online mode.
			if !txCfg.Offline {
				insDsc, err := conn.Runtime(mCfg.NPA.ParaTime).ROFLMarket.Instance(ctx, client.RoundLatest, *mCfg.ProviderAddr, mCfg.MachineID)
				cobra.CheckErr(err)

				fmt.Printf("Old admin: %s\n", insDsc.Admin)
			}

			fmt.Printf("New admin: %s\n", newAdminAddr)

			// Prepare transaction.
			tx := roflmarket.NewInstanceChangeAdmin(nil, &roflmarket.InstanceChangeAdmin{
				Provider: *mCfg.ProviderAddr,
				ID:       mCfg.MachineID,
				Admin:    *newAdminAddr,
			})

			acc := common.LoadAccount(cliConfig.Global(), mCfg.NPA.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, mCfg.NPA, acc, conn, tx, nil)
			cobra.CheckErr(err)

			if !common.BroadcastOrExportTransaction(ctx, mCfg.NPA, conn, sigTx, meta, nil) {
				return
			}

			fmt.Printf("Machine admin address changed.\n")
		},
	}

	topUpCmd = &cobra.Command{
		Use:   "top-up [<machine-name> | <provider-address>:<machine-id>]",
		Short: "Top-up payment for a machine",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			txCfg := common.GetTransactionConfig()

			mCfg, err := resolveMachineCfg(args, &roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: false,
			})
			cobra.CheckErr(err)
			ctx := context.Background()

			// This is required for the price pretty printer to work...
			if mCfg.NPA.ParaTime != nil {
				ctx = context.WithValue(ctx, config.ContextKeyParaTimeCfg, mCfg.NPA.ParaTime)
			}

			// Parse machine payment term.
			if roflCommon.Term == "" {
				cobra.CheckErr("no term period specified. Use --term to specify it")
			}
			term := roflCommon.ParseMachineTerm(roflCommon.Term)
			if roflCommon.TermCount < 1 {
				cobra.CheckErr("number of terms must be at least 1.")
			}

			// When not in offline mode, connect to the given network endpoint.
			var conn connection.Connection
			var offer *roflmarket.Offer
			if !txCfg.Offline {
				conn, err = connection.Connect(ctx, mCfg.NPA.Network)
				cobra.CheckErr(err)

				// Fetch chosen offer, so we can calculate price.
				offers, err := conn.Runtime(mCfg.NPA.ParaTime).ROFLMarket.Offers(ctx, client.RoundLatest, *mCfg.ProviderAddr)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to query provider: %w", err))
				}

				for _, of := range offers {
					if of.Metadata[buildRoflProvider.SchedulerMetadataOfferKey] == mCfg.Machine.Offer {
						offer = of
						break
					}
				}
				if offer == nil {
					machineDsc, err := conn.Runtime(mCfg.NPA.ParaTime).ROFLMarket.Instance(ctx, client.RoundLatest, *mCfg.ProviderAddr, mCfg.MachineID)
					if err != nil {
						cobra.CheckErr(fmt.Errorf("failed to query machine: %w", err))
					}
					for _, of := range offers {
						if of.ID == machineDsc.Offer {
							offer = of
							break
						}
					}
				}
				if offer == nil {
					cobra.CheckErr(fmt.Errorf("unable to find existing machine offer (%s) among market offers", mCfg.Machine.Offer))
				}
			}

			fmt.Printf("Using provider:     %s (%s)\n", mCfg.Machine.Provider, mCfg.ProviderAddr)
			fmt.Printf("Top-up machine:     %s [%s]\n", mCfg.MachineName, mCfg.Machine.ID)
			if txCfg.Offline {
				fmt.Printf("Top-up term:        %d x %s\n", roflCommon.TermCount, roflCommon.Term)
			} else {
				// Calculate total price and print term info.
				qTermCount := quantity.NewFromUint64(roflCommon.TermCount)
				totalPrice, ok := offer.Payment.Native.Terms[term]
				if !ok {
					cobra.CheckErr("previously selected term not found in offer terms")
				}
				cobra.CheckErr(totalPrice.Mul(qTermCount))
				tp := types.NewBaseUnits(totalPrice, offer.Payment.Native.Denomination)
				fmt.Printf("Top-up term:        %d x %s (", roflCommon.TermCount, roflCommon.Term)
				tp.PrettyPrint(ctx, "", os.Stdout)
				fmt.Println(" total)")
			}

			roflCommon.PrintRentRefundWarning()

			// Prepare transaction.
			tx := roflmarket.NewInstanceTopUpTx(nil, &roflmarket.InstanceTopUp{
				Provider:  *mCfg.ProviderAddr,
				ID:        mCfg.MachineID,
				Term:      term,
				TermCount: roflCommon.TermCount,
			})

			acc := common.LoadAccount(cliConfig.Global(), mCfg.NPA.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, mCfg.NPA, acc, conn, tx, nil)
			cobra.CheckErr(err)

			if !common.BroadcastOrExportTransaction(ctx, mCfg.NPA, conn, sigTx, meta, nil) {
				return
			}

			fmt.Printf("Machine topped up.\n")
		},
	}
)

// machineCfg contains all resolved machine configuration and related metadata.
type machineCfg struct {
	Machine      *buildRofl.Machine
	MachineName  string
	MachineID    roflmarket.InstanceID
	Manifest     *buildRofl.Manifest
	ExtraCfg     *roflCmdBuild.AppExtraConfig
	ProviderAddr *types.Address
	NPA          *common.NPASelection
}

// resolveMachineCfg resolves machine configuration from command-line arguments or manifest file.
// If args contains a provider:machine-id pair, it constructs the configuration from that.
// Otherwise, it loads the manifest file and resolves the machine from the deployment section.
// The function also resolves the provider address and validates the configuration.
func resolveMachineCfg(args []string, manifestOpts *roflCommon.ManifestOptions) (*machineCfg, error) {
	mCfg, err := resolveMachineCfgFromArgs(args)
	if err != nil {
		return nil, err
	}

	if mCfg != nil {
		mCfg.NPA = common.GetNPASelection(cliConfig.Global())
	} else {
		var deployment *buildRofl.Deployment
		mCfg = &machineCfg{}
		mCfg.Manifest, deployment, mCfg.NPA = roflCommon.LoadManifestAndSetNPA(manifestOpts)
		mCfg.Machine, mCfg.MachineName, mCfg.MachineID = resolveMachineFromManifest(args, deployment)

		var appID rofl.AppID
		if err = appID.UnmarshalText([]byte(deployment.AppID)); err != nil {
			return nil, fmt.Errorf("malformed app id: %w", err)
		}

		if mCfg.ExtraCfg, err = roflCmdBuild.ValidateApp(mCfg.Manifest, roflCmdBuild.ValidationOpts{
			Offline: true,
		}); err != nil {
			return nil, fmt.Errorf("failed to validate app: %w", err)
		}
	}

	if mCfg.ProviderAddr, _, err = common.ResolveLocalAccountOrAddress(mCfg.NPA.Network, mCfg.Machine.Provider); err != nil {
		return nil, fmt.Errorf("invalid provider address: %w", err)
	}

	return mCfg, nil
}

func resolveMachineCfgFromArgs(args []string) (*machineCfg, error) {
	mCfg := &machineCfg{}
	if len(args) == 0 {
		return nil, nil
	}
	parts := strings.Split(args[0], ":")
	if len(parts) != 2 {
		return nil, nil
	}

	mCfg.MachineName = args[0]
	mCfg.Machine = &buildRofl.Machine{
		Provider: parts[0],
		ID:       parts[1],
	}
	if err := mCfg.MachineID.UnmarshalText([]byte(mCfg.Machine.ID)); err != nil {
		return nil, fmt.Errorf("malformed machine ID: %w", err)
	}
	return mCfg, nil
}

func resolveMachineFromManifest(args []string, deployment *buildRofl.Deployment) (*buildRofl.Machine, string, roflmarket.InstanceID) {
	machineName := buildRofl.DefaultMachineName
	if len(args) > 0 {
		machineName = args[0]
	}
	var appID rofl.AppID
	if err := appID.UnmarshalText([]byte(deployment.AppID)); err != nil {
		cobra.CheckErr(fmt.Errorf("malformed ROFL app ID: %w", err))
	}

	var machine *buildRofl.Machine
	for name, mach := range deployment.Machines {
		if name == machineName {
			machine = mach
			break
		}
	}
	if machine == nil {
		fmt.Println("The following machines are configured in the app manifest:")
		for name := range deployment.Machines {
			fmt.Printf("  - %s\n", name)
		}
		cobra.CheckErr(fmt.Errorf("machine '%s' not found in manifest", machineName))
		return nil, "", roflmarket.InstanceID{}
	}
	if machine.ID == "" {
		cobra.CheckErr(fmt.Errorf("machine '%s' not yet deployed, use `oasis rofl deploy`", machineName))
	}

	var machineID roflmarket.InstanceID
	if err := machineID.UnmarshalText([]byte(machine.ID)); err != nil {
		cobra.CheckErr(fmt.Errorf("malformed machine ID: %w", err))
	}

	return machine, machineName, machineID
}

func queueCommand(cliArgs []string, method string, args any, msgAfter string) {
	mCfg, err := resolveMachineCfg(cliArgs, &roflCommon.ManifestOptions{
		NeedAppID: true,
		NeedAdmin: false,
	})
	cobra.CheckErr(err)

	// When not in offline mode, connect to the given network endpoint.
	ctx := context.Background()
	var conn connection.Connection
	if !common.GetTransactionConfig().Offline {
		conn, err = connection.Connect(ctx, mCfg.NPA.Network)
		cobra.CheckErr(err)
	}

	fmt.Printf("Using provider: %s (%s)\n", mCfg.Machine.Provider, mCfg.ProviderAddr)
	fmt.Printf("Machine:        %s [%s]\n", mCfg.MachineName, mCfg.Machine.ID)
	fmt.Printf("Command:        %s\n", method)
	fmt.Printf("Args:\n")
	fmt.Println(common.PrettyPrint(mCfg.NPA, "  ", args))

	// Prepare transaction.
	tx := roflmarket.NewInstanceExecuteCmdsTx(nil, &roflmarket.InstanceExecuteCmds{
		Provider: *mCfg.ProviderAddr,
		ID:       mCfg.MachineID,
		Cmds: [][]byte{cbor.Marshal(scheduler.Command{
			Method: method,
			Args:   cbor.Marshal(args),
		})},
	})

	acc := common.LoadAccount(cliConfig.Global(), mCfg.NPA.AccountName)
	sigTx, meta, err := common.SignParaTimeTransaction(ctx, mCfg.NPA, acc, conn, tx, nil)
	cobra.CheckErr(err)

	if !common.BroadcastOrExportTransaction(ctx, mCfg.NPA, conn, sigTx, meta, nil) {
		return
	}

	fmt.Println(msgAfter)
	fmt.Printf("Use `oasis rofl machine show` to see status.\n")
}

func showCommandArgs[V any](npa *common.NPASelection, raw []byte, args V) {
	if err := cbor.Unmarshal(raw, &args); err != nil {
		fmt.Printf("      %X\n", raw)
	} else {
		fmt.Println(common.PrettyPrint(npa, "      ", args))
	}
}

func init() {
	common.AddSelectorFlags(restartCmd)
	restartCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	restartCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	restartCmd.Flags().AddFlagSet(roflCommon.WipeFlags)

	common.AddSelectorFlags(stopCmd)
	stopCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	stopCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	stopCmd.Flags().AddFlagSet(roflCommon.WipeFlags)

	common.AddSelectorFlags(removeCmd)
	removeCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	removeCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)

	common.AddAccountFlag(changeAdminCmd)
	changeAdminCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	changeAdminCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)

	common.AddSelectorFlags(topUpCmd)
	topUpCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	topUpCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	topUpCmd.Flags().AddFlagSet(roflCommon.TermFlags)
}
