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

	"github.com/oasisprotocol/cli/build/rofl/provider"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var showCmd = &cobra.Command{
	Use:   "show <address>",
	Short: "Show details of a ROFL provider",
	Long: `Show detailed information about a specific ROFL provider.

This command queries on-chain provider data and displays all provider details
including address, scheduler app, nodes, payment address, metadata, and all offers.

Use --format json for machine-readable output.`,
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
	fmt.Printf("Provider: %s\n", provider.Address)
	fmt.Println()

	// Basic information.
	fmt.Println("=== Basic Information ===")
	fmt.Printf("Scheduler App:    %s\n", provider.SchedulerApp)

	// Payment address.
	var paymentAddr string
	switch {
	case provider.PaymentAddress.Native != nil:
		paymentAddr = provider.PaymentAddress.Native.String()
	case provider.PaymentAddress.Eth != nil:
		paymentAddr = fmt.Sprintf("0x%x", provider.PaymentAddress.Eth[:])
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
		fmt.Printf("Created At:       %s\n", time.Unix(int64(provider.CreatedAt), 0).UTC().Format(time.RFC3339))
	}
	if provider.UpdatedAt > 0 {
		fmt.Printf("Updated At:       %s\n", time.Unix(int64(provider.UpdatedAt), 0).UTC().Format(time.RFC3339))
	}

	// Metadata.
	fmt.Println()
	fmt.Println("=== Metadata ===")
	if len(provider.Metadata) == 0 {
		fmt.Println("<none>")
	} else {
		// Sort metadata keys for consistent output.
		keys := make([]string, 0, len(provider.Metadata))
		for k := range provider.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			fmt.Printf("  %s: %s\n", k, provider.Metadata[k])
		}
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

		for idx, offer := range offers {
			if idx > 0 {
				fmt.Println()
			}
			showOfferDetailed(npa, offer)
		}
	}
}

// showOfferDetailed outputs detailed information about an offer.
func showOfferDetailed(npa *common.NPASelection, offer *roflmarket.Offer) {
	// Extract offer name from metadata if available.
	name, ok := offer.Metadata[provider.SchedulerMetadataOfferKey]
	if !ok {
		name = "<unnamed>"
	}

	fmt.Printf("Offer: %s\n", name)
	fmt.Printf("  ID:              %s\n", offer.ID)
	fmt.Printf("  Capacity:        %d\n", offer.Capacity)

	// Resources.
	fmt.Println("  Resources:")
	fmt.Printf("    TEE:           %s\n", roflCommon.FormatTeeType(offer.Resources.TEE))
	fmt.Printf("    Memory:        %d MiB\n", offer.Resources.Memory)
	fmt.Printf("    vCPUs:         %d\n", offer.Resources.CPUCount)
	fmt.Printf("    Storage:       %.2f GiB\n", float64(offer.Resources.Storage)/1024.)

	if offer.Resources.GPU != nil {
		fmt.Printf("    GPU Count:     %d\n", offer.Resources.GPU.Count)
		if offer.Resources.GPU.Model != "" {
			fmt.Printf("    GPU Model:     %s\n", offer.Resources.GPU.Model)
		}
	}

	// Payment.
	fmt.Println("  Payment:")
	switch {
	case offer.Payment.Native != nil:
		fmt.Println("    Type:          Native")
		denom := offer.Payment.Native.Denomination
		if denom != "" {
			fmt.Printf("    Denomination:  %s\n", denom)
		}
		if len(offer.Payment.Native.Terms) > 0 {
			fmt.Println("    Terms:")
			// Sort term keys for consistent output.
			termKeys := make([]roflmarket.Term, 0, len(offer.Payment.Native.Terms))
			for k := range offer.Payment.Native.Terms {
				termKeys = append(termKeys, k)
			}
			sort.Slice(termKeys, func(i, j int) bool {
				return termKeys[i] < termKeys[j]
			})

			for _, term := range termKeys {
				amount := offer.Payment.Native.Terms[term]

				// Format amount with denomination.
				bu := types.NewBaseUnits(amount, denom)
				formattedAmount := helpers.FormatParaTimeDenomination(npa.ParaTime, bu)
				fmt.Printf("      %s: %s\n", roflCommon.FormatTermAdjectival(term), formattedAmount)
			}
		}
	case offer.Payment.EvmContract != nil:
		fmt.Println("    Type:          EVM Contract")
		fmt.Printf("    Address:       0x%x\n", offer.Payment.EvmContract.Address[:])
		if len(offer.Payment.EvmContract.Data) > 0 {
			fmt.Printf("    Data:          0x%x\n", offer.Payment.EvmContract.Data)
		}
	default:
		fmt.Println("    Type:          <none>")
	}

	// Metadata.
	if len(offer.Metadata) > 0 {
		fmt.Println("  Metadata:")
		// Sort metadata keys for consistent output.
		keys := make([]string, 0, len(offer.Metadata))
		for k := range offer.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			fmt.Printf("    %s: %s\n", k, offer.Metadata[k])
		}
	}
}

func init() {
	showCmd.Flags().AddFlagSet(common.SelectorNPFlags)
	showCmd.Flags().AddFlagSet(common.FormatFlag)
}
