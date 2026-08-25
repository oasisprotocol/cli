package common

import (
	"fmt"
	"sort"
	"strings"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/build/rofl/provider"
	"github.com/oasisprotocol/cli/cmd/common"
)

// FilterOffers removes the offers marked as private from the given listing, unless the user
// explicitly asked for them via --all.
func FilterOffers(offers []*roflmarket.Offer) []*roflmarket.Offer {
	if ShowPrivateOffers {
		return offers
	}

	public := make([]*roflmarket.Offer, 0, len(offers))
	for _, offer := range offers {
		if provider.IsOfferPrivate(offer) {
			continue
		}
		public = append(public, offer)
	}
	return public
}

// ShowOfferSummary outputs a summary of a single offer.
func ShowOfferSummary(npa *common.NPASelection, offer *roflmarket.Offer) {
	// Extract offer name from metadata if available.
	name, ok := offer.Metadata[provider.SchedulerMetadataOfferKey]
	if !ok {
		name = "<unnamed>"
	}

	// Determine TEE type.
	tee := FormatTeeType(offer.Resources.TEE)

	// Format GPU info if present.
	var gpu string
	if offer.Resources.GPU != nil {
		gpu = fmt.Sprintf(" | GPU: %d", offer.Resources.GPU.Count)
		if offer.Resources.GPU.Model != "" {
			gpu += fmt.Sprintf(" (%s)", offer.Resources.GPU.Model)
		}
	}

	var private string
	if provider.IsOfferPrivate(offer) {
		private = " (private)"
	}

	fmt.Printf("  - %s [%s]%s\n", name, offer.ID, private)
	fmt.Printf("    TEE: %s | Memory: %d MiB | vCPUs: %d | Storage: %.2f GiB%s\n",
		tee,
		offer.Resources.Memory,
		offer.Resources.CPUCount,
		float64(offer.Resources.Storage)/1024.,
		gpu,
	)
	fmt.Printf("    Capacity: %d\n", offer.Capacity)

	// Note and Description from metadata.
	if note, ok := offer.Metadata[provider.NoteMetadataKey]; ok {
		fmt.Printf("    Note: %s\n", note)
	}
	if desc, ok := offer.Metadata[provider.DescriptionMetadataKey]; ok {
		fmt.Printf("    Description:\n      %s\n", strings.ReplaceAll(desc, "\n", "\n      "))
	}

	// Accounts allowed to rent machines, when the offer is restricted.
	if creators := provider.OfferAllowedCreators(offer); len(creators) > 0 {
		prettyCreators := make([]string, 0, len(creators))
		for _, creator := range creators {
			prettyCreators = append(prettyCreators, common.PrettyAddress(creator))
		}
		fmt.Printf("    Allowed creators: %s\n", strings.Join(prettyCreators, ", "))
	}

	// Payment info.
	switch {
	case offer.Payment.Native != nil:
		if len(offer.Payment.Native.Terms) > 0 {
			var terms []string //nolint: prealloc
			for term, amount := range offer.Payment.Native.Terms {
				bu := types.NewBaseUnits(amount, offer.Payment.Native.Denomination)
				formattedAmount := helpers.FormatParaTimeDenomination(npa.ParaTime, bu)
				terms = append(terms, fmt.Sprintf("%s: %s", FormatTermAdjectival(term), formattedAmount))
			}
			sort.Strings(terms)
			fmt.Printf("    Payment: %s\n", strings.Join(terms, ", "))
		}
	case offer.Payment.EvmContract != nil:
		fmt.Printf("    Payment: EVM Contract (0x%x)\n", offer.Payment.EvmContract.Address[:])
	}
}
