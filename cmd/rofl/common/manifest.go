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
func LoadManifestAndSetNPA(cfg *config.Config, npa *common.NPASelection, deployment string, needAppID bool) (*rofl.Manifest, *rofl.Deployment) {
	manifest, d, err := MaybeLoadManifestAndSetNPA(cfg, npa, deployment)
	cobra.CheckErr(err)
	if needAppID && !d.HasAppID() {
		cobra.CheckErr(fmt.Errorf("deployment '%s' does not have an app ID set, maybe you need to run `oasis rofl create`", deployment))
	}
	return manifest, d
}

// MaybeLoadManifestAndSetNPA loads the ROFL app manifest and reconfigures the
// network/paratime/account selection.
//
// In case there is an error in loading the manifest, it is returned.
func MaybeLoadManifestAndSetNPA(cfg *config.Config, npa *common.NPASelection, deployment string) (*rofl.Manifest, *rofl.Deployment, error) {
	manifest, err := rofl.LoadManifest()
	if err != nil {
		return nil, nil, err
	}

	d, ok := manifest.Deployments[deployment]
	if !ok {
		return nil, nil, fmt.Errorf("deployment '%s' does not exist", deployment)
	}

	switch d.Network {
	case "":
		if npa.Network == nil {
			return nil, nil, fmt.Errorf("no network selected")
		}
	default:
		npa.Network = cfg.Networks.All[d.Network]
		if npa.Network == nil {
			return nil, nil, fmt.Errorf("network '%s' does not exist", d.Network)
		}
		npa.NetworkName = d.Network
	}
	switch d.ParaTime {
	case "":
		if npa.ParaTime == nil {
			return nil, nil, fmt.Errorf("no ParaTime selected")
		}
	default:
		npa.ParaTime = npa.Network.ParaTimes.All[d.ParaTime]
		if npa.ParaTime == nil {
			return nil, nil, fmt.Errorf("paratime '%s' does not exist", d.ParaTime)
		}
		npa.ParaTimeName = d.ParaTime
	}
	switch d.Admin {
	case "":
	default:
		accCfg, err := common.LoadAccountConfig(cfg, d.Admin)
		if err != nil {
			return nil, nil, err
		}
		npa.Account = accCfg
		npa.AccountName = d.Admin
	}
	return manifest, d, nil
}
