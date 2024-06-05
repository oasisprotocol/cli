package config

import (
	"fmt"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
)

// latestMigrationVersion is the target migration version. All migrations from prior versions will
// be executed to reach this target version.
const latestMigrationVersion = 1

// migrationFunc is a configuration migration function.
//
// It should return true if any changes have been applied as part of the migration.
type migrationFunc func(*Config) (bool, error)

var migrations = map[int]migrationFunc{
	0: migrateFromV0,
}

// migrateFromV0 performs the following migrations:
//
// - Any old Pontus-X ParaTimes on Testnet are removed.
// - New Pontus-X ParaTimes on Testnet are added.
func migrateFromV0(cfg *Config) (bool, error) {
	const testnetChainContext = "0b91b8e4e44b2003a7c5e23ddadb5e14ef5345c0ebcb3ddcae07fa2f244cab76"
	pontusx := map[string]*config.ParaTime{
		"pontusx_dev": {
			Description: "Pontus-X Devnet",
			ID:          "0000000000000000000000000000000000000000000000004febe52eb412b421",
			Denominations: map[string]*config.DenominationInfo{
				config.NativeDenominationKey: {
					Symbol:   "EUROe",
					Decimals: 18,
				},
				// The consensus layer denomination when deposited into the runtime.
				"TEST": {
					Symbol:   "TEST",
					Decimals: 18,
				},
			},
			ConsensusDenomination: "TEST",
		},
		"pontusx_test": {
			Description: "Pontus-X Testnet",
			ID:          "00000000000000000000000000000000000000000000000004a6f9071c007069",
			Denominations: map[string]*config.DenominationInfo{
				config.NativeDenominationKey: {
					Symbol:   "EUROe",
					Decimals: 18,
				},
				// The consensus layer denomination when deposited into the runtime.
				"TEST": {
					Symbol:   "TEST",
					Decimals: 18,
				},
			},
			ConsensusDenomination: "TEST",
		},
	}

	for _, net := range cfg.Networks.All {
		if net.ChainContext != testnetChainContext {
			continue
		}

		// Delete old Pontus-X ParaTimes and fix the defaults.
		for ptName, pt := range net.ParaTimes.All {
			for newPtName, newPt := range pontusx {
				if pt.ID != newPt.ID {
					continue
				}

				delete(net.ParaTimes.All, ptName)
				if net.ParaTimes.Default == ptName {
					net.ParaTimes.Default = newPtName
				}
			}
		}

		// Add new Pontus-X ParaTimes.
		for ptName, pt := range pontusx {
			ptCopy := *pt
			net.ParaTimes.All[ptName] = &ptCopy
		}
	}
	return true, nil
}

// migrateVersions applies migrations for all versions up to the latestMigrationVersion.
func (cfg *Config) migrateVersions() (bool, error) {
	var changes bool
	for v := cfg.LastMigration; v < latestMigrationVersion; v++ {
		fn, ok := migrations[v]
		if !ok {
			return false, fmt.Errorf("missing migration from version %d", v)
		}

		ch, err := fn(cfg)
		if err != nil {
			return false, fmt.Errorf("failed to apply migration from version %d: %w", v, err)
		}
		changes = changes || ch
		cfg.LastMigration = v + 1
	}
	return changes, nil
}

func init() {
	// Make sure that it is possible to migrate from zero to the latest migration version.
	for v := 0; v < latestMigrationVersion; v++ {
		if _, ok := migrations[v]; !ok {
			panic(fmt.Errorf("missing migration from version %d", v))
		}
	}
}
