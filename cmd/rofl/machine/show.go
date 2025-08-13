package machine

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	compose "github.com/compose-spec/compose-go/v2/cli"
	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/ed25519"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/build/rofl/scheduler"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
)

var showCmd = &cobra.Command{
	Use:   "show [<machine-name>]",
	Short: "Show information about a machine",
	Args:  cobra.MaximumNArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		manifest, deployment, npa := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
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

		providerDsc, err := conn.Runtime(npa.ParaTime).ROFLMarket.Provider(ctx, client.RoundLatest, *providerAddr)
		cobra.CheckErr(err)

		var schedulerDsc *rofl.Registration
		switch schedulerRAKRaw, ok := insDsc.Metadata[scheduler.MetadataKeySchedulerRAK]; ok {
		case true:
			var schedulerRAK ed25519.PublicKey
			if err := schedulerRAK.UnmarshalText([]byte(schedulerRAKRaw)); err != nil {
				cobra.CheckErr(fmt.Sprintf("Malformed scheduler RAK metadata: %s", err))
			}
			pk := types.PublicKey{PublicKey: schedulerRAK}

			schedulerDsc, _ = conn.Runtime(npa.ParaTime).ROFL.AppInstance(ctx, client.RoundLatest, providerDsc.SchedulerApp, pk)
		default:
		}

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

		if proxyDomain, ok := schedulerDsc.Metadata[scheduler.MetadataKeyProxyDomain]; ok {
			numericMachineID := binary.BigEndian.Uint64(machineID[:])
			proxyDomain = fmt.Sprintf("m%d.%s", numericMachineID, proxyDomain)

			fmt.Printf("Proxy:\n")
			fmt.Printf("  Domain: %s\n", proxyDomain)

			showMachinePorts(manifest, proxyDomain)
		}

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

func showMachinePorts(manifest *buildRofl.Manifest, domain string) {
	if manifest.TEE != buildRofl.TEETypeTDX || manifest.Kind != buildRofl.AppKindContainer {
		return
	}
	if manifest.Artifacts == nil {
		return
	}
	composeFile := manifest.Artifacts.Container.Compose
	if composeFile == "" {
		return
	}

	// Parse the compose file.
	options, err := compose.NewProjectOptions([]string{composeFile}, compose.WithInterpolation(false))
	if err != nil {
		return
	}
	proj, err := options.LoadProject(context.Background())
	if err != nil {
		return
	}

	type portDsc struct {
		serviceName string
		port        string
	}
	var ports []portDsc
	for serviceName, service := range proj.Services {
		for _, port := range service.Ports {
			if port.Protocol != "tcp" {
				continue
			}
			mode := service.Annotations[fmt.Sprintf("net.oasis.proxy.ports.%s.mode", port.Published)]
			if mode == "ignore" {
				continue
			}
			ports = append(ports, portDsc{serviceName, port.Published})
		}
	}
	if len(ports) == 0 {
		return
	}

	fmt.Printf("  Ports from compose file:\n")
	for _, p := range ports {
		fmt.Printf("    %s (%s): https://p%s.%s\n", p.port, p.serviceName, p.port, domain)
	}
}

func init() {
	showCmd.Flags().AddFlagSet(common.SelectorFlags)
	showCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
}
