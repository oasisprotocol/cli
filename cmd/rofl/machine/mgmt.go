package machine

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
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
			fmt.Printf("WARNING: Canceling a machine will permanently destroy it including any persistent storage!\n")

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
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			fmt.Printf("Using provider:     %s (%s)\n", machine.Provider, providerAddr)
			fmt.Printf("Top-up machine:     %s [%s]\n", machineName, machine.ID)
			fmt.Printf("Top-up term:        %d x %s\n", roflCommon.TermCount, roflCommon.Term)

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
	restartCmd.Flags().AddFlagSet(common.SelectorFlags)
	restartCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	restartCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	restartCmd.Flags().AddFlagSet(roflCommon.WipeFlags)

	stopCmd.Flags().AddFlagSet(common.SelectorFlags)
	stopCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	stopCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	stopCmd.Flags().AddFlagSet(roflCommon.WipeFlags)

	removeCmd.Flags().AddFlagSet(common.SelectorFlags)
	removeCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	removeCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)

	topUpCmd.Flags().AddFlagSet(common.SelectorFlags)
	topUpCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	topUpCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	topUpCmd.Flags().AddFlagSet(roflCommon.TermFlags)
}
