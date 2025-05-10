package machine

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
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
	deploymentName string
	wipeStorage    bool
	topUpTerm      string
	topUpTermCount uint64

	showCmd = &cobra.Command{
		Use:   "show [<machine-name>]",
		Short: "Show information about a machine",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)

			_, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: false,
			})

			machine, machineName, machineID := resolveMachine(args, deployment)

			// Establish connection with the target network.
			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			// Resolve provider address.
			providerAddr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, machine.Provider)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("Invalid provider address: %s", err))
			}

			insDsc, err := conn.Runtime(npa.ParaTime).ROFLMarket.Instance(ctx, client.RoundLatest, *providerAddr, machineID)
			cobra.CheckErr(err)

			insCmds, err := conn.Runtime(npa.ParaTime).ROFLMarket.InstanceCommands(ctx, client.RoundLatest, *providerAddr, machineID)
			cobra.CheckErr(err)

			fmt.Printf("Name:       %s\n", machineName)
			fmt.Printf("Provider:   %s\n", insDsc.Provider)
			fmt.Printf("ID:         %s\n", insDsc.ID)
			fmt.Printf("Offer:      %s\n", insDsc.Offer)
			fmt.Printf("Status:     %s\n", insDsc.Status)
			fmt.Printf("Creator:    %s\n", insDsc.Creator)
			fmt.Printf("Admin:      %s\n", insDsc.Admin)
			switch insDsc.NodeID {
			case nil:
				fmt.Printf("Node ID:    <none>\n")
			default:
				fmt.Printf("Node ID:    %s\n", insDsc.NodeID)
			}

			fmt.Printf("Created at: %s\n", time.Unix(int64(insDsc.CreatedAt), 0))
			fmt.Printf("Updated at: %s\n", time.Unix(int64(insDsc.UpdatedAt), 0))
			fmt.Printf("Paid until: %s\n", time.Unix(int64(insDsc.PaidUntil), 0))

			if len(insDsc.Metadata) > 0 {
				fmt.Printf("Metadata:\n")
				for key, value := range insDsc.Metadata {
					fmt.Printf("  %s: %s\n", key, value)
				}
			}

			fmt.Printf("Resources:\n")

			fmt.Printf("  TEE:     ")
			switch insDsc.Resources.TEE {
			case roflmarket.TeeTypeSGX:
				fmt.Printf("Intel SGX\n")
			case roflmarket.TeeTypeTDX:
				fmt.Printf("Intel TDX\n")
			default:
				fmt.Printf("[unknown: %d]\n", insDsc.Resources.TEE)
			}

			fmt.Printf("  Memory:  %d MiB\n", insDsc.Resources.Memory)
			fmt.Printf("  vCPUs:   %d\n", insDsc.Resources.CPUCount)
			fmt.Printf("  Storage: %d MiB\n", insDsc.Resources.Storage)
			if insDsc.Resources.GPU != nil {
				fmt.Printf("  GPU:\n")
				if insDsc.Resources.GPU.Model != "" {
					fmt.Printf("    Model: %s\n", insDsc.Resources.GPU.Model)
				} else {
					fmt.Printf("    Model: <any>\n")
				}
				fmt.Printf("    Count: %d\n", insDsc.Resources.GPU.Count)
			}

			switch insDsc.Deployment {
			default:
				fmt.Printf("Deployment:\n")
				fmt.Printf("  App ID: %s\n", insDsc.Deployment.AppID)

				if len(insDsc.Deployment.Metadata) > 0 {
					fmt.Printf("  Metadata:\n")
					for key, value := range insDsc.Deployment.Metadata {
						fmt.Printf("    %s: %s\n", key, value)
					}
				}
			case nil:
				fmt.Printf("Deployment: <no current deployment>\n")
			}

			// Show commands.
			fmt.Printf("Commands:\n")
			if len(insCmds) > 0 {
				for _, qc := range insCmds {
					fmt.Printf("  - ID: %s\n", qc.ID)

					var cmd scheduler.Command
					err := cbor.Unmarshal(qc.Cmd, &cmd)
					switch err {
					case nil:
						// Decodable scheduler command.
						fmt.Printf("    Method: %s\n", cmd.Method)
						fmt.Printf("    Args:\n")

						switch cmd.Method {
						case scheduler.MethodDeploy:
							showCommandArgs(npa, cmd.Args, scheduler.DeployRequest{})
						case scheduler.MethodRestart:
							showCommandArgs(npa, cmd.Args, scheduler.RestartRequest{})
						case scheduler.MethodTerminate:
							showCommandArgs(npa, cmd.Args, scheduler.TerminateRequest{})
						default:
							showCommandArgs(npa, cmd.Args, make(map[string]interface{}))
						}
					default:
						// Unknown command format.
						fmt.Printf("    <unknown format: %X>\n", qc.Cmd)
					}
				}
			} else {
				fmt.Printf("  <no queued commands>\n")
			}
		},
	}

	restartCmd = &cobra.Command{
		Use:   "restart [<machine-name>]",
		Short: "Restart a running machine or start a stopped one",
		Args:  cobra.MaximumNArgs(1),
		Run: func(_ *cobra.Command, args []string) {
			queueCommand(
				args,
				scheduler.MethodRestart,
				scheduler.RestartRequest{
					WipeStorage: wipeStorage,
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
					WipeStorage: wipeStorage,
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
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			manifest, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
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

			acc := common.LoadAccount(cfg, npa.AccountName)
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
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			_, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: false,
			})

			machine, machineName, machineID := resolveMachine(args, deployment)

			// Resolve provider address.
			providerAddr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, machine.Provider)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("Invalid provider address: %s", err))
			}

			// Parse machine payment term.
			term := roflCommon.ParseMachineTerm(topUpTerm)
			if topUpTermCount < 1 {
				cobra.CheckErr("Number of terms must be at least 1.")
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
			fmt.Printf("Top-up term:        %d x %s\n", topUpTermCount, topUpTerm)

			// Prepare transaction.
			tx := roflmarket.NewInstanceTopUpTx(nil, &roflmarket.InstanceTopUp{
				Provider:  *providerAddr,
				ID:        machineID,
				Term:      term,
				TermCount: topUpTermCount,
			})

			acc := common.LoadAccount(cfg, npa.AccountName)
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
	cfg := cliConfig.Global()
	npa := common.GetNPASelection(cfg)
	txCfg := common.GetTransactionConfig()

	_, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
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

	acc := common.LoadAccount(cfg, npa.AccountName)
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
	deploymentFlags := flag.NewFlagSet("", flag.ContinueOnError)
	deploymentFlags.StringVar(&deploymentName, "deployment", buildRofl.DefaultDeploymentName, "deployment name")

	wipeFlags := flag.NewFlagSet("", flag.ContinueOnError)
	wipeFlags.BoolVar(&wipeStorage, "wipe-storage", false, "whether to wipe machine storage")

	topUpFlags := flag.NewFlagSet("", flag.ContinueOnError)
	topUpFlags.StringVar(&topUpTerm, "term", roflCommon.TermMonth, "term to pay for in advance")
	topUpFlags.Uint64Var(&topUpTermCount, "term-count", 1, "number of terms to pay for in advance")

	showCmd.Flags().AddFlagSet(common.SelectorFlags)
	showCmd.Flags().AddFlagSet(deploymentFlags)

	restartCmd.Flags().AddFlagSet(common.SelectorFlags)
	restartCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	restartCmd.Flags().AddFlagSet(deploymentFlags)
	restartCmd.Flags().AddFlagSet(wipeFlags)

	stopCmd.Flags().AddFlagSet(common.SelectorFlags)
	stopCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	stopCmd.Flags().AddFlagSet(deploymentFlags)
	stopCmd.Flags().AddFlagSet(wipeFlags)

	removeCmd.Flags().AddFlagSet(common.SelectorFlags)
	removeCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	removeCmd.Flags().AddFlagSet(deploymentFlags)

	topUpCmd.Flags().AddFlagSet(common.SelectorFlags)
	topUpCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	topUpCmd.Flags().AddFlagSet(deploymentFlags)
	topUpCmd.Flags().AddFlagSet(topUpFlags)
}
