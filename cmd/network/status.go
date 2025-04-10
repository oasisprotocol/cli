package network

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	coreCommon "github.com/oasisprotocol/oasis-core/go/common"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

func getParatimeName(cfg *cliConfig.Config, id string) string {
	for _, net := range cfg.Networks.All {
		for ptName, pt := range net.ParaTimes.All {
			if id == pt.ID {
				return (ptName)
			}
		}
	}
	return ("unknown")
}

func getNetworkName(context string) string {
	for key, net := range sdkConfig.DefaultNetworks.All {
		if context == net.ChainContext {
			return (key)
		}
	}
	return ("unknown")
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current status of the node and the network",
	Args:  cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)

		// Establish connection with the target network.
		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		cobra.CheckErr(err)

		ctrlConn := conn.Control()

		nodeStatus, err := ctrlConn.GetStatus(ctx)
		cobra.CheckErr(err)

		switch common.OutputFormat() {
		case common.FormatJSON:
			nodeStr, err := common.PrettyJSONMarshal(nodeStatus)
			cobra.CheckErr(err)

			fmt.Println(string(nodeStr))
		default:
			fmt.Println("=== NETWORK STATUS ===")
			fmt.Printf("Network:      %s", npa.PrettyPrintNetwork())
			fmt.Println()

			fmt.Printf("Node's ID:    %s", nodeStatus.Identity.Node)
			fmt.Println()

			fmt.Printf("Core version: %s", nodeStatus.SoftwareVersion)
			fmt.Println()

			fmt.Println()
			fmt.Printf("==== Consensus ====")
			fmt.Println()

			consensus := nodeStatus.Consensus
			if consensus != nil {
				fmt.Printf("Status:               %s", consensus.Status.String())
				fmt.Println()

				fmt.Printf("Version:              %s", consensus.Version.String())
				fmt.Println()

				fmt.Printf("Chain context:        %s (%s)", getNetworkName(consensus.ChainContext), consensus.ChainContext)
				fmt.Println()

				date := time.Unix(nodeStatus.Consensus.LatestTime.Unix(), 0)
				fmt.Printf("Latest height:        %d (%s)", consensus.LatestHeight, date)
				fmt.Println()

				fmt.Printf("Latest block hash:    %s", consensus.LatestHash)
				fmt.Println()

				fmt.Printf("Latest epoch:         %d", consensus.LatestEpoch)
				fmt.Println()

				fmt.Printf("Is validator:         %t", consensus.IsValidator)
				fmt.Println()
			}

			registration := nodeStatus.Registration
			if registration != nil {
				fmt.Printf("Registration:         %t", registration.LastAttemptSuccessful)
				fmt.Println()

				if !registration.LastRegistration.IsZero() {
					fmt.Printf("  Last:               %s", registration.LastRegistration)
					fmt.Println()
				}

				if !nodeStatus.Registration.LastAttemptSuccessful {
					if !registration.LastAttempt.IsZero() {
						fmt.Printf("  Last attempt:       %s", registration.LastAttempt)
						fmt.Println()
					}

					if registration.LastAttemptErrorMessage != "" {
						fmt.Printf("  Last attempt error: %s", registration.LastAttemptErrorMessage)
						fmt.Println()
					}
				}

				descriptor := registration.Descriptor
				if descriptor != nil {
					fmt.Printf("  Entity ID:          %s", descriptor.EntityID)
					fmt.Println()
				}
			}

			if len(nodeStatus.Runtimes) > 0 {
				fmt.Println()
				fmt.Println("==== ParaTimes ====")
			}

			keys := make([]coreCommon.Namespace, 0, len(nodeStatus.Runtimes))
			for key := range nodeStatus.Runtimes {
				keys = append(keys, key)
			}
			sort.Slice(keys, func(i, j int) bool {
				var iParatimeName, jParatimeName string

				iKey := keys[i].String()
				jKey := keys[j].String()

				iParatimeName = getParatimeName(cfg, iKey) + iKey
				jParatimeName = getParatimeName(cfg, jKey) + jKey

				return iParatimeName < jParatimeName
			})

			for _, key := range keys {
				runtime := nodeStatus.Runtimes[key]

				paratimeName := getParatimeName(cfg, key.String())
				fmt.Printf("%s (%s):", paratimeName, key.String())
				fmt.Println()

				descriptor := runtime.Descriptor
				if descriptor != nil {
					fmt.Printf("  Kind:                 %s", descriptor.Kind)
					fmt.Println()

					fmt.Printf("  Is confidential:      %t", (descriptor.TEEHardware > 0))
					fmt.Println()
				}

				committee := runtime.Committee
				if committee != nil {
					fmt.Printf("  Status:               %s", committee.Status)
					fmt.Println()
				}

				date := time.Unix(int64(runtime.LatestTime), 0)
				fmt.Printf("  Latest round:         %d (%s)", runtime.LatestRound, date)
				fmt.Println()

				storage := runtime.Storage
				if storage != nil {
					fmt.Printf("  Last finalized round: %d", storage.LastFinalizedRound)
					fmt.Println()

					fmt.Printf("  Storage status:       %s", storage.Status)
					fmt.Println()
				}

				if committee != nil {
					activeVersion := committee.ActiveVersion
					if activeVersion != nil {
						fmt.Printf("  Active version:       %s", activeVersion)
						fmt.Println()
					}
				}

				var availableVersions []string
				if descriptor != nil {
					deployments := descriptor.Deployments
					if deployments != nil {
						for _, deployment := range deployments {
							availableVersions = append(availableVersions, deployment.Version.String())
						}
						fmt.Printf("  Available version(s): %s", strings.Join(availableVersions, ", "))
						fmt.Println()
					}
				}

				if committee != nil {
					fmt.Printf("  Number of peers:      %d", len(committee.Peers))
					fmt.Println()
				}
			}
		}
	},
}

func init() {
	statusCmd.Flags().AddFlagSet(common.FormatFlag)
	statusCmd.Flags().AddFlagSet(common.SelectorNFlags)
}
