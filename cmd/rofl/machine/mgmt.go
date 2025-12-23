package machine

import (
	"context"
	"fmt"
	"os"

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
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	restartCmd = &cobra.Command{
		Use:   "restart [<machine-name>]",
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
		Use:     "stop [<machine-name>]",
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
		Use:     "remove [<machine-name>]",
		Short:   "Cancel rental and remove the machine",
		Aliases: []string{"cancel", "rm"},
		Args:    cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			txCfg := common.GetTransactionConfig()

			manifest, deployment, npa := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: false,
			})

			machine, machineName, machineID := resolveMachine(args, deployment)

			// Resolve provider address.
			providerAddr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, machine.Provider)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("Invalid provider address: %s", err))
			}

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			fmt.Printf("Using provider:     %s (%s)\n", machine.Provider, providerAddr)
			fmt.Printf("Canceling machine:  %s [%s]\n", machineName, machine.ID)
			common.Warn("WARNING: Canceling a machine will permanently destroy it including any persistent storage!")
			roflCommon.PrintRentRefundWarning()

			// Prepare transaction.
			tx := roflmarket.NewInstanceCancelTx(nil, &roflmarket.InstanceCancel{
				Provider: *providerAddr,
				ID:       machineID,
			})

			acc := common.LoadAccount(cliConfig.Global(), npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
				return
			}

			fmt.Printf("Machine removed.\n")

			// Update manifest to clear the machine ID as it has been cancelled.
			machine.ID = ""

			if err = manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}
		},
	}

	changeAdminCmd = &cobra.Command{
		Use:   "change-admin [<machine-name>] <new-admin>",
		Short: "Change the machine administrator",
		Args:  cobra.RangeArgs(1, 2),
		Run: func(_ *cobra.Command, args []string) {
			txCfg := common.GetTransactionConfig()

			_, deployment, npa := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: false,
			})

			var newAdminAddress string
			switch len(args) {
			case 1:
				// Just admin address.
				newAdminAddress = args[0]
				args = nil
			case 2:
				// Machine and admin address.
				newAdminAddress = args[1]
				args = args[:1]
			}

			machine, machineName, machineID := resolveMachine(args, deployment)

			// Resolve provider address.
			providerAddr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, machine.Provider)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("Invalid provider address: %s", err))
			}

			// Resolve new admin address.
			newAdminAddr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, newAdminAddress)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("Invalid admin address: %s", err))
			}

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			fmt.Printf("Provider:  %s (%s)\n", machine.Provider, providerAddr)
			fmt.Printf("Machine:   %s [%s]\n", machineName, machine.ID)

			// Resolve old admin in online mode.
			if !txCfg.Offline {
				insDsc, err := conn.Runtime(npa.ParaTime).ROFLMarket.Instance(ctx, client.RoundLatest, *providerAddr, machineID)
				cobra.CheckErr(err)

				fmt.Printf("Old admin: %s\n", insDsc.Admin)
			}

			fmt.Printf("New admin: %s\n", newAdminAddr)

			// Prepare transaction.
			tx := roflmarket.NewInstanceChangeAdmin(nil, &roflmarket.InstanceChangeAdmin{
				Provider: *providerAddr,
				ID:       machineID,
				Admin:    *newAdminAddr,
			})

			acc := common.LoadAccount(cliConfig.Global(), npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
				return
			}

			fmt.Printf("Machine admin address changed.\n")
		},
	}

	topUpCmd = &cobra.Command{
		Use:   "top-up [<machine-name>]",
		Short: "Top-up payment for a machine",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			txCfg := common.GetTransactionConfig()

			_, deployment, npa := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: false,
			})
			ctx := context.Background()

			// This is required for the price pretty printer to work...
			if npa.ParaTime != nil {
				ctx = context.WithValue(ctx, config.ContextKeyParaTimeCfg, npa.ParaTime)
			}

			machine, machineName, machineID := resolveMachine(args, deployment)

			// Resolve provider address.
			providerAddr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, machine.Provider)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("invalid provider address: %s", err))
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
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)

				// Fetch chosen offer, so we can calculate price.
				offers, err := conn.Runtime(npa.ParaTime).ROFLMarket.Offers(ctx, client.RoundLatest, *providerAddr)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("failed to query provider: %s", err))
				}

				for _, of := range offers {
					if of.Metadata[buildRoflProvider.SchedulerMetadataOfferKey] == machine.Offer {
						offer = of
						break
					}
				}
				if offer == nil {
					cobra.CheckErr(fmt.Errorf("unable to find existing machine offer (%s) among market offers", machine.Offer))
				}
			}

			fmt.Printf("Using provider:     %s (%s)\n", machine.Provider, providerAddr)
			fmt.Printf("Top-up machine:     %s [%s]\n", machineName, machine.ID)
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
				Provider:  *providerAddr,
				ID:        machineID,
				Term:      term,
				TermCount: roflCommon.TermCount,
			})

			acc := common.LoadAccount(cliConfig.Global(), npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
				return
			}

			fmt.Printf("Machine topped up.\n")
		},
	}
)

func resolveMachine(args []string, deployment *buildRofl.Deployment) (*buildRofl.Machine, string, roflmarket.InstanceID) {
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
	txCfg := common.GetTransactionConfig()

	_, deployment, npa := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
		NeedAppID: true,
		NeedAdmin: false,
	})

	machine, machineName, machineID := resolveMachine(cliArgs, deployment)

	// Resolve provider address.
	providerAddr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, machine.Provider)
	if err != nil {
		cobra.CheckErr(fmt.Sprintf("Invalid provider address: %s", err))
	}

	// When not in offline mode, connect to the given network endpoint.
	ctx := context.Background()
	var conn connection.Connection
	if !txCfg.Offline {
		conn, err = connection.Connect(ctx, npa.Network)
		cobra.CheckErr(err)
	}

	fmt.Printf("Using provider: %s (%s)\n", machine.Provider, providerAddr)
	fmt.Printf("Machine:        %s [%s]\n", machineName, machine.ID)
	fmt.Printf("Command:        %s\n", method)
	fmt.Printf("Args:\n")
	fmt.Println(common.PrettyPrint(npa, "  ", args))

	// Prepare transaction.
	tx := roflmarket.NewInstanceExecuteCmdsTx(nil, &roflmarket.InstanceExecuteCmds{
		Provider: *providerAddr,
		ID:       machineID,
		Cmds: [][]byte{cbor.Marshal(scheduler.Command{
			Method: method,
			Args:   cbor.Marshal(args),
		})},
	})

	acc := common.LoadAccount(cliConfig.Global(), npa.AccountName)
	sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
	cobra.CheckErr(err)

	if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
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
