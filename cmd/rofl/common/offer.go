package common

import (
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"

	"github.com/oasisprotocol/cli/build/rofl/provider"
)

// FilterOffers removes the offers marked as private from the given listing, unless the user
// explicitly asked for them via --all.
func FilterOffers(offers []*roflmarket.Offer) []*roflmarket.Offer {
	if ShowPrivateOffers {
		return offers
	}
	return PublicOffers(offers)
}

// PublicOffers returns only the offers that are not marked as private.
func PublicOffers(offers []*roflmarket.Offer) []*roflmarket.Offer {
	public := make([]*roflmarket.Offer, 0, len(offers))
	for _, offer := range offers {
		if provider.IsOfferPrivate(offer) {
			continue
		}
		public = append(public, offer)
	}
	return public
}
