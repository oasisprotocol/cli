package config

import (
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
)

// Default is the default config that should be used in case no configuration file exists.
var Default = Config{
	Networks: config.DefaultNetworks,
}

// OldNetworks contains information about old versions (e.g. chain contexts) of known networks so
// they can be automatically migrated.
var OldNetworks = map[string][]string{
	// Testnet.
	"testnet": {
		"50304f98ddb656620ea817cc1446c401752a05a249b36c9b90dba4616829977a", // 2022-03-03
	},
}
