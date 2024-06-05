package config

import (
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
)

// Default is the default config that should be used in case no configuration file exists.
var Default = Config{
	Networks:      config.DefaultNetworks,
	LastMigration: latestMigrationVersion,
}

// OldNetwork contains information about an old version of a known network configuration.
type OldNetwork struct {
	// ChainContexts are the known chain contexts.
	ChainContexts []string
	// RPCs are the known RPC endpoint addresses.
	RPCs []string
}

// OldNetworks contains information about old versions (e.g. chain contexts) of known networks so
// they can be automatically migrated.
var OldNetworks = map[string]*OldNetwork{
	// Mainnet.
	"mainnet": {
		ChainContexts: []string{
			"b11b369e0da5bb230b220127f5e7b242d385ef8c6f54906243f30af63c815535", // oasis-3
		},
		RPCs: []string{
			"grpc.oasis.dev:443", // Deprecated in 2024.
		},
	},
	// Testnet.
	"testnet": {
		ChainContexts: []string{
			"50304f98ddb656620ea817cc1446c401752a05a249b36c9b90dba4616829977a", // 2022-03-03
		},
		RPCs: []string{
			"testnet.grpc.oasis.dev:443", // Deprecated in 2024.
		},
	},
}
