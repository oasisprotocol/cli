package machine

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/ed25519"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/build/rofl/scheduler"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCmdBuild "github.com/oasisprotocol/cli/cmd/rofl/build"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
)

type machineShowOutput struct {
	Machine         *roflmarket.Instance        `json:"machine,omitempty"`
	MachineCommands []*roflmarket.QueuedCommand `json:"machine_commands,omitempty"`
	Provider        *roflmarket.Provider        `json:"provider,omitempty"`
	Replica         *rofl.Registration          `json:"replica,omitempty"`
}

var showCmd = &cobra.Command{
	Use:   "show [<machine-name> | <provider-address>:<machine-id>]",
	Short: "Show information about a machine",
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		var out machineShowOutput
		mCfg, err := resolveMachineCfg(args, &roflCommon.ManifestOptions{
			NeedAppID: true,
			NeedAdmin: false,
		})
		cobra.CheckErr(err)

		// Establish connection with the target network.
		ctx := context.Background()
		conn, err := connection.Connect(ctx, mCfg.NPA.Network)
		cobra.CheckErr(err)

		out.Machine, err = conn.Runtime(mCfg.NPA.ParaTime).ROFLMarket.Instance(ctx, client.RoundLatest, *mCfg.ProviderAddr, mCfg.MachineID)
		if err != nil {
			// The "instance not found" error originates from Rust code, so we can't compare it nicely here.
			if strings.Contains(err.Error(), "instance not found") {
				switch common.OutputFormat() {
				case common.FormatJSON:
					fmt.Printf("{ \"error\": \"%s\" }\n", err)
				case common.FormatText:
					cobra.CheckErr("Machine instance not found.\nTip: This often happens when instances expire. Run `oasis rofl deploy --replace-machine` to rent a new one.")
				}
			}
			cobra.CheckErr(err)
		}

		out.MachineCommands, err = conn.Runtime(mCfg.NPA.ParaTime).ROFLMarket.InstanceCommands(ctx, client.RoundLatest, *mCfg.ProviderAddr, mCfg.MachineID)
		cobra.CheckErr(err)

		out.Provider, err = conn.Runtime(mCfg.NPA.ParaTime).ROFLMarket.Provider(ctx, client.RoundLatest, *mCfg.ProviderAddr)
		cobra.CheckErr(err)

		switch schedulerRAKRaw, ok := out.Machine.Metadata[scheduler.MetadataKeySchedulerRAK]; ok {
		case true:
			var schedulerRAK ed25519.PublicKey
			if err := schedulerRAK.UnmarshalText([]byte(schedulerRAKRaw)); err != nil {
				cobra.CheckErr(fmt.Errorf("malformed scheduler RAK metadata: %w", err))
			}
			pk := types.PublicKey{PublicKey: schedulerRAK}

			out.Replica, _ = conn.Runtime(mCfg.NPA.ParaTime).ROFL.AppInstance(ctx, client.RoundLatest, out.Provider.SchedulerApp, pk)
		default:
		}

		if common.OutputFormat() == common.FormatJSON {
			data, err := json.MarshalIndent(out, "", "  ")
			cobra.CheckErr(err)
			fmt.Printf("%s\n", data)
		} else {
			prettyPrintMachine(mCfg, &out)
		}
	},
}

// prettyPrintMachine prints a compact human-readable info of the ROFL machine.
func prettyPrintMachine(mCfg *machineCfg, out *machineShowOutput) {
	paidUntil := time.Unix(int64(out.Machine.PaidUntil), 0) //nolint:gosec
	expired := !time.Now().Before(paidUntil)

	fmt.Printf("Name:       %s\n", mCfg.MachineName)
	fmt.Printf("Provider:   %s\n", out.Machine.Provider)
	fmt.Printf("ID:         %s\n", out.Machine.ID)
	fmt.Printf("Offer:      %s\n", out.Machine.Offer)
	fmt.Printf("Status:     %s", out.Machine.Status)
	if expired {
		fmt.Printf(" (EXPIRED)")
	}
	fmt.Println()
	fmt.Printf("Creator:    %s\n", out.Machine.Creator)
	fmt.Printf("Admin:      %s\n", out.Machine.Admin)
	switch out.Machine.NodeID {
	case nil:
		fmt.Printf("Node ID:    <none>\n")
	default:
		fmt.Printf("Node ID:    %s\n", out.Machine.NodeID)
	}

	fmt.Printf("Created at: %s\n", time.Unix(int64(out.Machine.CreatedAt), 0)) //nolint:gosec
	fmt.Printf("Updated at: %s\n", time.Unix(int64(out.Machine.UpdatedAt), 0)) //nolint:gosec
	fmt.Printf("Paid until: %s\n", paidUntil)

	if out.Replica != nil {
		if proxyDomain, ok := out.Replica.Metadata[scheduler.MetadataKeyProxyDomain]; ok {
			numericMachineID := binary.BigEndian.Uint64(mCfg.MachineID[:])
			proxyDomain = fmt.Sprintf("m%d.%s", numericMachineID, proxyDomain)

			fmt.Printf("Proxy:\n")
			fmt.Printf("  Domain: %s\n", proxyDomain)

			prettyPrintMachinePorts(mCfg.ExtraCfg, out.Machine.Deployment.AppID, out.Machine, proxyDomain)
		}
	}

	if len(out.Machine.Metadata) > 0 {
		fmt.Printf("Metadata:\n")
		for key, value := range out.Machine.Metadata {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}

	fmt.Printf("Resources:\n")

	fmt.Printf("  TEE:     ")
	switch out.Machine.Resources.TEE {
	case roflmarket.TeeTypeSGX:
		fmt.Printf("Intel SGX\n")
	case roflmarket.TeeTypeTDX:
		fmt.Printf("Intel TDX\n")
	default:
		fmt.Printf("[unknown: %d]\n", out.Machine.Resources.TEE)
	}

	fmt.Printf("  Memory:  %d MiB\n", out.Machine.Resources.Memory)
	fmt.Printf("  vCPUs:   %d\n", out.Machine.Resources.CPUCount)
	fmt.Printf("  Storage: %d MiB\n", out.Machine.Resources.Storage)
	if out.Machine.Resources.GPU != nil {
		fmt.Printf("  GPU:\n")
		if out.Machine.Resources.GPU.Model != "" {
			fmt.Printf("    Model: %s\n", out.Machine.Resources.GPU.Model)
		} else {
			fmt.Printf("    Model: <any>\n")
		}
		fmt.Printf("    Count: %d\n", out.Machine.Resources.GPU.Count)
	}

	switch out.Machine.Deployment {
	default:
		fmt.Printf("Deployment:\n")
		fmt.Printf("  App ID: %s\n", out.Machine.Deployment.AppID)

		if len(out.Machine.Deployment.Metadata) > 0 {
			fmt.Printf("  Metadata:\n")
			for key, value := range out.Machine.Deployment.Metadata {
				fmt.Printf("    %s: %s\n", key, value)
			}
		}
	case nil:
		fmt.Printf("Deployment: <no current deployment>\n")
	}

	// Show commands.
	fmt.Printf("Commands:\n")
	if len(out.MachineCommands) > 0 {
		for _, qc := range out.MachineCommands {
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
					showCommandArgs(mCfg.NPA, cmd.Args, scheduler.DeployRequest{})
				case scheduler.MethodRestart:
					showCommandArgs(mCfg.NPA, cmd.Args, scheduler.RestartRequest{})
				case scheduler.MethodTerminate:
					showCommandArgs(mCfg.NPA, cmd.Args, scheduler.TerminateRequest{})
				default:
					showCommandArgs(mCfg.NPA, cmd.Args, make(map[string]any))
				}
			default:
				// Unknown command format.
				fmt.Printf("    <unknown format: %X>\n", qc.Cmd)
			}
		}
	} else {
		fmt.Printf("  <no queued commands>\n")
	}
}

func prettyPrintMachinePorts(extraCfg *roflCmdBuild.AppExtraConfig, appID rofl.AppID, insDsc *roflmarket.Instance, domain string) {
	if extraCfg == nil || len(extraCfg.Ports) == 0 {
		return
	}

	fmt.Printf("  Ports from compose file:\n")
	for i, p := range extraCfg.Ports {
		genericDomain := fmt.Sprintf("%s.%s", p.GenericDomain, domain)
		switch p.CustomDomain {
		case "":
			fmt.Printf("    %s (%s): https://%s\n", p.Port, p.ServiceName, genericDomain)
		default:
			fmt.Printf("    %s (%s): https://%s\n", p.Port, p.ServiceName, p.CustomDomain)

			domainToken := scheduler.DomainVerificationToken(insDsc, appID, p.CustomDomain)
			addrs, err := net.LookupHost(genericDomain)
			if err == nil {
				fmt.Printf("      * Point A record of your domain to: %s\n", addrs[0])
			}
			fmt.Printf("      * Add TXT record to your domain: oasis-rofl-verification=%s\n", domainToken)
		}

		if i < len(extraCfg.Ports)-1 {
			fmt.Println()
		}
	}
}

func init() {
	common.AddSelectorFlags(showCmd)
	showCmd.Flags().AddFlagSet(common.FormatFlag)
	showCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
}
