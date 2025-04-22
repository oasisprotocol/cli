package common

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
	"github.com/oasisprotocol/cli/config"
)

// ManifestOptions configures the manifest options.
type ManifestOptions struct {
	// NeedAppID specifies whether a configured app ID is required in the manifest.
	NeedAppID bool
	// NeedAdmin specifies whether a valid admin is required in the manifest. In case this is set to
	// false and the admin account does not exist, the account will not be modified.
	NeedAdmin bool
}

// LoadManifestAndSetNPA loads the ROFL app manifest and reconfigures the network/paratime/account
// selection.
//
// In case there is an error in loading the manifest, it aborts the application.
func LoadManifestAndSetNPA(cfg *config.Config, npa *common.NPASelection, deployment string, opts *ManifestOptions) (*rofl.Manifest, *rofl.Deployment) {
	manifest, d, err := MaybeLoadManifestAndSetNPA(cfg, npa, deployment, opts)
	cobra.CheckErr(err)
	if opts != nil && opts.NeedAppID && !d.HasAppID() {
		cobra.CheckErr(fmt.Errorf("deployment '%s' does not have an app ID set, maybe you need to run `oasis rofl create`", deployment))
	}
	return manifest, d
}

// MaybeLoadManifestAndSetNPA loads the ROFL app manifest and reconfigures the
// network/paratime/account selection.
//
// In case there is an error in loading the manifest, it is returned.
func MaybeLoadManifestAndSetNPA(cfg *config.Config, npa *common.NPASelection, deployment string, opts *ManifestOptions) (*rofl.Manifest, *rofl.Deployment, error) {
	manifest, err := rofl.LoadManifest()
	if err != nil {
		return nil, nil, err
	}

	d, ok := manifest.Deployments[deployment]
	if !ok {
		fmt.Println("The following deployments are configured in the app manifest:")
		for name := range manifest.Deployments {
			fmt.Printf("  - %s\n", name)
		}
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
		npa.MustHaveParaTime()
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
		switch {
		case err == nil:
			npa.Account = accCfg
			npa.AccountName = d.Admin
		case opts != nil && opts.NeedAdmin:
			return nil, nil, err
		default:
			// Admin account is not valid, but it is also not required, so do not override.
		}
	}
	return manifest, d, nil
}

// GetOrcFilename generates a filename based on the project name and deployment.
func GetOrcFilename(manifest *rofl.Manifest, deploymentName string) string {
	return fmt.Sprintf("%s.%s.orc", manifest.Name, deploymentName)
}
