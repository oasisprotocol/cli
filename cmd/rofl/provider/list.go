package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"

	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
	"github.com/oasisprotocol/cli/table"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List ROFL providers",
	Long: `List all ROFL providers registered on the selected paratime.

This command queries on-chain provider data and displays provider addresses,
scheduler app IDs, node counts, and offer/instance counts.

Use --show-offers to expand and display all offers for each provider.
Private offers are hidden by default, use --all to include them.
Use --format json for machine-readable output.`,
	Args: cobra.NoArgs,
	Run: func(_ *cobra.Command, _ []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		npa.MustHaveParaTime()

		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		cobra.CheckErr(err)

		// Query all providers from the chain.
		providers, err := conn.Runtime(npa.ParaTime).ROFLMarket.Providers(ctx, client.RoundLatest)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("failed to query providers: %w", err))
		}

		if len(providers) == 0 {
			fmt.Println("No providers found.")
			return
		}

		// Sort providers by address for consistent output.
		sort.Slice(providers, func(i, j int) bool {
			return providers[i].Address.String() < providers[j].Address.String()
		})

		// Output format handling.
		if common.OutputFormat() == common.FormatJSON {
			outputJSON(ctx, npa, conn, providers)
		} else {
			outputText(ctx, npa, conn, providers)
		}
	},
}

// outputJSON returns providers in JSON format.
func outputJSON(ctx context.Context, npa *common.NPASelection, conn connection.Connection, providers []*roflmarket.Provider) {
	type ProviderWithOffers struct {
		*roflmarket.Provider
		Offers []*roflmarket.Offer `json:"offers,omitempty"`
	}

	output := make([]ProviderWithOffers, 0, len(providers))

	for _, provider := range providers {
		pwo := ProviderWithOffers{Provider: provider}

		if roflCommon.ShowOffers {
			offers, err := conn.Runtime(npa.ParaTime).ROFLMarket.Offers(ctx, client.RoundLatest, provider.Address)
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to query offers for provider %s: %w", provider.Address, err))
			}
			pwo.Offers = roflCommon.FilterOffers(offers)
		}

		output = append(output, pwo)
	}

	data, err := json.MarshalIndent(output, "", "  ")
	cobra.CheckErr(err)
	fmt.Printf("%s\n", data)
}

// outputText returns providers in human-readable table format.
func outputText(ctx context.Context, npa *common.NPASelection, conn connection.Connection, providers []*roflmarket.Provider) {
	table := table.New()
	table.Header("Provider Address", "Scheduler App", "Nodes", "Offers", "Instances")

	rows := make([][]string, 0, len(providers))
	for _, provider := range providers {
		// Format node count.
		var nodesList string
		if len(provider.Nodes) == 0 {
			nodesList = "0"
		} else {
			nodesList = fmt.Sprintf("%d", len(provider.Nodes))
		}

		rows = append(rows, []string{
			common.PrettyAddress(provider.Address.String()),
			provider.SchedulerApp.String(),
			nodesList,
			fmt.Sprintf("%d", provider.OffersCount),
			fmt.Sprintf("%d", provider.InstancesCount),
		})
	}

	cobra.CheckErr(table.Bulk(rows))
	cobra.CheckErr(table.Render())

	// If --show-offers is enabled, display offers for each provider.
	if roflCommon.ShowOffers {
		fmt.Println()
		for _, provider := range providers {
			showProviderOffersExpanded(ctx, npa, conn, provider)
		}
	}
}

// showProviderOffersExpanded returns all offers for a given provider with expanded display.
func showProviderOffersExpanded(ctx context.Context, npa *common.NPASelection, conn connection.Connection, provider *roflmarket.Provider) {
	offers, err := conn.Runtime(npa.ParaTime).ROFLMarket.Offers(ctx, client.RoundLatest, provider.Address)
	if err != nil {
		cobra.CheckErr(fmt.Errorf("failed to query offers for provider %s: %w", provider.Address, err))
	}
	offers = roflCommon.FilterOffers(offers)

	prettyAddr := common.PrettyAddress(provider.Address.String())

	if len(offers) == 0 {
		fmt.Printf("Provider %s: No offers\n", prettyAddr)
		return
	}

	// Sort offers by ID for consistent output.
	sort.Slice(offers, func(i, j int) bool {
		for k := 0; k < 8; k++ {
			if offers[i].ID[k] != offers[j].ID[k] {
				return offers[i].ID[k] < offers[j].ID[k]
			}
		}
		return false
	})

	fmt.Printf("Provider %s (%d offers):\n", prettyAddr, len(offers))
	for _, offer := range offers {
		roflCommon.ShowOfferSummary(npa, offer)
	}
	fmt.Println()
}

func init() {
	listCmd.Flags().AddFlagSet(roflCommon.ShowOffersFlag)
	listCmd.Flags().AddFlagSet(roflCommon.ShowPrivateOffersFlag)
	common.AddSelectorNPFlags(listCmd)
	listCmd.Flags().AddFlagSet(common.FormatFlag)
}
