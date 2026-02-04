package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var showCmd = &cobra.Command{
	Use:   "show <address>",
	Short: "Show details of a ROFL provider",
	Long: `Show detailed information about a specific ROFL provider.

This command queries on-chain provider data and displays all provider details
including address, scheduler app, nodes, payment address, and all offers.

Use --format json for machine-readable output including provider metadata.`,
	Args: cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		cfg := cliConfig.Global()
		npa := common.GetNPASelection(cfg)
		npa.MustHaveParaTime()

		ctx := context.Background()
		conn, err := connection.Connect(ctx, npa.Network)
		cobra.CheckErr(err)

		// Parse provider address.
		providerAddr := args[0]
		var addr types.Address
		if err = addr.UnmarshalText([]byte(providerAddr)); err != nil {
			cobra.CheckErr(fmt.Errorf("invalid provider address '%s': %w", providerAddr, err))
		}

		// Query the provider from the chain.
		provider, err := conn.Runtime(npa.ParaTime).ROFLMarket.Provider(ctx, client.RoundLatest, addr)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("failed to query provider: %w", err))
		}

		// Query offers for the provider.
		offers, err := conn.Runtime(npa.ParaTime).ROFLMarket.Offers(ctx, client.RoundLatest, addr)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("failed to query offers for provider: %w", err))
		}

		// Output format handling.
		if common.OutputFormat() == common.FormatJSON {
			outputProviderJSON(provider, offers)
		} else {
			outputProviderText(npa, provider, offers)
		}
	},
}

// outputProviderJSON outputs provider details in JSON format.
func outputProviderJSON(provider *roflmarket.Provider, offers []*roflmarket.Offer) {
	type ProviderWithOffers struct {
		*roflmarket.Provider
		Offers []*roflmarket.Offer `json:"offers"`
	}

	output := ProviderWithOffers{
		Provider: provider,
		Offers:   offers,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	cobra.CheckErr(err)
	fmt.Printf("%s\n", data)
}

// outputProviderText outputs provider details in human-readable format.
func outputProviderText(npa *common.NPASelection, provider *roflmarket.Provider, offers []*roflmarket.Offer) {
	fmt.Printf("Provider: %s\n", common.PrettyAddress(provider.Address.String()))
	fmt.Println()

	// Basic information.
	fmt.Println("=== Basic Information ===")
	fmt.Printf("Scheduler App:    %s\n", provider.SchedulerApp)

	var paymentAddr string
	switch {
	case provider.PaymentAddress.Native != nil:
		paymentAddr = common.PrettyAddress(provider.PaymentAddress.Native.String())
	case provider.PaymentAddress.Eth != nil:
		ethAddr := fmt.Sprintf("0x%x", provider.PaymentAddress.Eth[:])
		paymentAddr = common.PrettyAddress(ethAddr)
	default:
		paymentAddr = "<none>"
	}
	fmt.Printf("Payment Address:  %s\n", paymentAddr)

	// Nodes.
	fmt.Printf("Nodes:            ")
	if len(provider.Nodes) == 0 {
		fmt.Println("<none>")
	} else {
		fmt.Printf("%d\n", len(provider.Nodes))
		for i, node := range provider.Nodes {
			fmt.Printf("  %d. %s\n", i+1, node)
		}
	}

	// Stake.
	stake := helpers.FormatParaTimeDenomination(npa.ParaTime, provider.Stake)
	fmt.Printf("Stake:            %s\n", stake)

	// Counts.
	fmt.Printf("Offers:           %d\n", provider.OffersCount)
	fmt.Printf("Instances:        %d\n", provider.InstancesCount)

	// Timestamps.
	if provider.CreatedAt > 0 {
		fmt.Printf("Created At:       %s\n", time.Unix(int64(provider.CreatedAt), 0).UTC().Format(time.RFC3339)) //nolint:gosec
	}
	if provider.UpdatedAt > 0 {
		fmt.Printf("Updated At:       %s\n", time.Unix(int64(provider.UpdatedAt), 0).UTC().Format(time.RFC3339)) //nolint:gosec
	}

	// Offers.
	fmt.Println()
	fmt.Println("=== Offers ===")
	if len(offers) == 0 {
		fmt.Println("<none>")
	} else {
		// Sort offers by ID for consistent output.
		sort.Slice(offers, func(i, j int) bool {
			for k := 0; k < 8; k++ {
				if offers[i].ID[k] != offers[j].ID[k] {
					return offers[i].ID[k] < offers[j].ID[k]
				}
			}
			return false
		})

		for _, offer := range offers {
			ShowOfferSummary(npa, offer)
		}
	}
}

func init() {
	common.AddSelectorNPFlags(showCmd)
	showCmd.Flags().AddFlagSet(common.FormatFlag)
}
