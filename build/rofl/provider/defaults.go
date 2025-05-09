package provider

import "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

// RoflServices contains per network/ParaTime default ROFL services.
type RoflServices struct {
	// Scheduler is a ROFL app ID of the preferred scheduler app.
	Scheduler string

	// Provider is the address of the preferred provider.
	Provider string
}

// DefaultRoflServices contains default built-in ROFL services for all supported networks/ParaTimes.
var DefaultRoflServices = map[string]RoflServices{
	config.DefaultNetworks.All["testnet"].ParaTimes.All["sapphire"].ID: {
		Scheduler: "rofl1qrqw99h0f7az3hwt2cl7yeew3wtz0fxunu7luyfg",
		Provider:  "oasis1qp2ens0hsp7gh23wajxa4hpetkdek3swyyulyrmz",
	},
}
