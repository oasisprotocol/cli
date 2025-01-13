package common

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/config"
)

// LoadManifestAndSetNPA loads the ROFL app manifest and reconfigures the network/paratime/account
// selection.
//
// In case there is an error in loading the manifest, it aborts the application.
func LoadManifestAndSetNPA(cfg *config.Config, npa *common.NPASelection) *rofl.Manifest {
	manifest, err := MaybeLoadManifestAndSetNPA(cfg, npa)
	cobra.CheckErr(err)
	return manifest
}

// MaybeLoadManifestAndSetNPA loads the ROFL app manifest and reconfigures the
// network/paratime/account selection.
//
// In case there is an error in loading the manifest, it is returned.
func MaybeLoadManifestAndSetNPA(cfg *config.Config, npa *common.NPASelection) (*rofl.Manifest, error) {
	manifest, err := rofl.LoadManifest()
	if err != nil {
		return nil, err
	}

	switch manifest.Network {
	case "":
		if npa.Network == nil {
			return nil, fmt.Errorf("no network selected")
		}
	default:
		npa.Network = cfg.Networks.All[manifest.Network]
		if npa.Network == nil {
			return nil, fmt.Errorf("network '%s' does not exist", manifest.Network)
		}
		npa.NetworkName = manifest.Network
	}
	switch manifest.ParaTime {
	case "":
		if npa.ParaTime == nil {
			return nil, fmt.Errorf("no ParaTime selected")
		}
	default:
		npa.ParaTime = npa.Network.ParaTimes.All[manifest.ParaTime]
		if npa.ParaTime == nil {
			return nil, fmt.Errorf("paratime '%s' does not exist", manifest.ParaTime)
		}
		npa.ParaTimeName = manifest.ParaTime
	}
	switch manifest.Admin {
	case "":
	default:
		accCfg, err := common.LoadAccountConfig(cfg, manifest.Admin)
		if err != nil {
			return nil, err
		}
		npa.Account = accCfg
		npa.AccountName = manifest.Admin
	}
	return manifest, nil
}
