package common

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common/version"

	"github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
	cliVersion "github.com/oasisprotocol/cli/version"
)

var (
	// orcFilenameDisallowedChars is a regexp matching characters that are not allowed in filenames.
	orcFilenameDisallowedChars = regexp.MustCompile("[^a-zA-Z0-9-]")

	// orcFilenameRepeatedChars is a regexp matching repeats of dash characters in filenames.
	orcFilenameRepeatedChars = regexp.MustCompile("(-){2,}")
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
func LoadManifestAndSetNPA(opts *ManifestOptions) (*rofl.Manifest, *rofl.Deployment, *common.NPASelection) {
	cfg := cliConfig.Global()
	npa := common.GetNPASelection(cfg)

	manifest, resolvedName, d, err := MaybeLoadManifestAndSetNPA(cfg, npa, DeploymentName, opts)
	cobra.CheckErr(err)
	DeploymentName = resolvedName
	if opts != nil && opts.NeedAppID && !d.HasAppID() {
		cobra.CheckErr(fmt.Errorf("deployment '%s' does not have an app ID set, maybe you need to run `oasis rofl create`", DeploymentName))
	}
	return manifest, d, npa
}

// MaybeLoadManifestAndSetNPA loads the ROFL app manifest and reconfigures the
// network/paratime/account selection.
//
// In case there is an error in loading the manifest, it is returned. The resolved deployment name
// is always returned alongside the deployment.
func MaybeLoadManifestAndSetNPA(cfg *cliConfig.Config, npa *common.NPASelection, deployment string, opts *ManifestOptions) (*rofl.Manifest, string, *rofl.Deployment, error) {
	manifest, err := rofl.LoadManifest()
	if err != nil {
		return nil, "", nil, err
	}

	// Warn if manifest was created with an older CLI version.
	checkToolingVersion(manifest)

	// Resolve deployment name when not explicitly provided.
	if deployment == "" {
		deployment = manifest.DefaultDeployment()
	}
	if deployment == "" {
		if len(manifest.Deployments) == 0 {
			return nil, "", nil, fmt.Errorf("no deployments configured\nHint: use `oasis rofl create` to register a new ROFL app and create a deployment")
		}
		printAvailableDeployments(manifest)
		return nil, "", nil, fmt.Errorf("no deployment selected\nHint: Run this command with --deployment to select one or `oasis rofl set-default <name>` to set a default deployment")
	}

	d, ok := manifest.Deployments[deployment]
	if !ok {
		printAvailableDeployments(manifest)
		return nil, "", nil, fmt.Errorf("deployment '%s' does not exist", deployment)
	}

	switch d.Network {
	case "":
		if npa.Network == nil {
			return nil, "", nil, fmt.Errorf("no network selected")
		}
	default:
		npa.Network = cfg.Networks.All[d.Network]
		if npa.Network == nil {
			return nil, "", nil, fmt.Errorf("network '%s' does not exist", d.Network)
		}
		npa.NetworkName = d.Network
	}
	switch d.ParaTime {
	case "":
		npa.MustHaveParaTime()
	default:
		npa.ParaTime = npa.Network.ParaTimes.All[d.ParaTime]
		if npa.ParaTime == nil {
			return nil, "", nil, fmt.Errorf("paratime '%s' does not exist", d.ParaTime)
		}
		npa.ParaTimeName = d.ParaTime
	}
	switch {
	case d.Admin == "":
		// No admin in manifest, leave unchanged.
	case npa.AccountSetExplicitly:
		// Account explicitly overridden on the command line, leave unchanged.
	default:
		accCfg, err := common.LoadAccountConfig(cfg, d.Admin)
		switch {
		case err == nil:
			npa.Account = accCfg
			npa.AccountName = d.Admin
		case opts != nil && opts.NeedAdmin:
			return nil, "", nil, err
		default:
			// Admin account is not valid, but it is also not required, so do not override.
		}
	}
	return manifest, deployment, d, nil
}

func printAvailableDeployments(manifest *rofl.Manifest) {
	if len(manifest.Deployments) == 0 {
		return
	}
	fmt.Println("The following deployments are configured in the app manifest:")
	for name := range manifest.Deployments {
		fmt.Printf("  - %s\n", name)
	}
}

// GetOrcFilename generates a filename based on the project name and deployment.
func GetOrcFilename(manifest *rofl.Manifest, deploymentName string) string {
	normalizedName := strings.ToLower(manifest.Name)
	normalizedName = strings.TrimSpace(normalizedName)
	normalizedName = orcFilenameDisallowedChars.ReplaceAllString(normalizedName, "-")
	normalizedName = orcFilenameRepeatedChars.ReplaceAllString(normalizedName, "$1")
	return fmt.Sprintf("%s.%s.orc", normalizedName, deploymentName)
}

// normalizeVersion strips the leading 'v' prefix from a version string for parsing.
func normalizeVersion(s string) string {
	return strings.TrimPrefix(s, "v")
}

// checkToolingVersion warns if the manifest was created with a different CLI version.
func checkToolingVersion(manifest *rofl.Manifest) {
	if manifest.Tooling == nil || manifest.Tooling.Version == "" {
		common.Warnf("WARNING: Manifest has no tooling version. Consider running `oasis rofl upgrade`.")
		return
	}

	manifestVer, err := version.FromString(normalizeVersion(manifest.Tooling.Version))
	if err != nil {
		common.Warnf("WARNING: Failed to parse manifest tooling version '%s'. Consider running `oasis rofl upgrade`.", manifest.Tooling.Version)
		return
	}
	currentVer, err := version.FromString(normalizeVersion(cliVersion.Software))
	if err != nil {
		panic(fmt.Sprintf("failed to parse CLI version '%s': %v", cliVersion.Software, err))
	}

	if manifestVer.ToU64() < currentVer.ToU64() {
		common.Warnf("WARNING: Manifest was created with an older CLI version (%s < %s). Consider running `oasis rofl upgrade`.", manifest.Tooling.Version, cliVersion.Software)
	} else if manifestVer.ToU64() > currentVer.ToU64() {
		common.Warnf("WARNING: Manifest was created with a newer CLI version (%s > %s). Consider upgrading the CLI.", manifest.Tooling.Version, cliVersion.Software)
	}
}
