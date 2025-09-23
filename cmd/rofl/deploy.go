package rofl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	"github.com/oasisprotocol/oasis-core/go/common/sgx"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	buildRofl "github.com/oasisprotocol/cli/build/rofl"
	"github.com/oasisprotocol/cli/build/rofl/provider"
	"github.com/oasisprotocol/cli/build/rofl/scheduler"
	"github.com/oasisprotocol/cli/cmd/common"
	roflCmdBuild "github.com/oasisprotocol/cli/cmd/rofl/build"
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	deployProvider       string
	deployOffer          string
	deployMachine        string
	deployForce          bool
	deployShowOffers     bool
	deployReplaceMachine bool

	deployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Deploy ROFL to a specified machine",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			txCfg := common.GetTransactionConfig()

			if txCfg.Offline {
				cobra.CheckErr("offline mode currently not supported")
			}

			manifest, deployment, npa := roflCommon.LoadManifestAndSetNPA(&roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: true,
			})

			var appID rofl.AppID
			if err := appID.UnmarshalText([]byte(deployment.AppID)); err != nil {
				cobra.CheckErr(fmt.Sprintf("malformed app id: %s", err))
			}

			extraCfg, err := roflCmdBuild.ValidateApp(manifest, roflCmdBuild.ValidationOpts{
				Offline: true,
			})
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("failed to validate app: %s", err))
			}

			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

			// This is required for the pretty printer to work...
			if npa.ParaTime != nil {
				ctx = context.WithValue(ctx, config.ContextKeyParaTimeCfg, npa.ParaTime)
			}

			manifestEnclaves := make(map[sgx.EnclaveIdentity]struct{})
			for _, eid := range deployment.Policy.Enclaves {
				manifestEnclaves[eid.ID] = struct{}{}
			}

			cfgEnclaves, err := roflCommon.GetRegisteredEnclaves(ctx, deployment.AppID, npa)
			cobra.CheckErr(err)

			if !maps.Equal(manifestEnclaves, cfgEnclaves) && !deployForce {
				// TODO: Generate and run Update TX automatically.
				cobra.CheckErr("Local enclave identities DIFFER from on-chain enclave identities! Run `oasis rofl update` first")
			}

			machine, ok := deployment.Machines[deployMachine]
			if !ok {
				machine = new(buildRofl.Machine)
				if deployment.Machines == nil {
					deployment.Machines = make(map[string]*buildRofl.Machine)
				}
				deployment.Machines[deployMachine] = machine
			}

			switch machine.Provider {
			case "":
				// Not yet set, obtain a new machine from the provider.
				if deployProvider == "" {
					if roflServices, ok := provider.DefaultRoflServices[npa.ParaTime.ID]; ok {
						deployProvider = roflServices.Provider
					} else {
						cobra.CheckErr(fmt.Sprintf("Provider not configured for deployment '%s' machine '%s'. Please specify --provider.", roflCommon.DeploymentName, deployMachine))
					}
				}

				machine.Provider = deployProvider
			default:
				// Already set, require the provider to be omitted or equal.
				if deployProvider != "" && deployProvider != machine.Provider {
					cobra.CheckErr(fmt.Sprintf("Provider '%s' conflicts with existing provider '%s' for deployment '%s' machine '%s'. Omit --provider.", deployProvider, machine.Provider, roflCommon.DeploymentName, deployMachine))
				}
			}

			// Resolve provider address.
			providerAddr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, machine.Provider)
			if err != nil {
				cobra.CheckErr(fmt.Sprintf("Invalid provider address: %s", err))
			}

			fmt.Printf("Using provider: %s (%s)\n", machine.Provider, providerAddr)

			if deployShowOffers {
				// Display all offers supported by the provider.
				showProviderOffers(ctx, npa, conn, *providerAddr)
				return
			}

			acc := common.LoadAccount(cliConfig.Global(), npa.AccountName)

			ociRepository := ociRepository(deployment)
			orcFilename := roflCommon.GetOrcFilename(manifest, roflCommon.DeploymentName)
			fmt.Printf("Pushing ROFL app to OCI repository '%s'...\n", ociRepository)
			ociDigest, manifestHash := pushBundleToOciRepository(orcFilename, ociRepository)
			// Save the OCI repository field to the configuration file so we avoid multiple uploads.
			deployment.OCIRepository = ociRepository
			if err := manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Sprintf("Failed to update manifest with OCI repository: %s", err))
			}

			// Define the deployment.
			machineDeployment := roflmarket.Deployment{
				AppID:        appID,
				ManifestHash: manifestHash,
				Metadata: map[string]string{
					scheduler.MetadataKeyORCReference: fmt.Sprintf("%s@%s", deployment.OCIRepository, ociDigest),
				},
			}
			if len(machine.Permissions) > 0 {
				perms, err := resolveAndMarshalPermissions(npa, machine.Permissions)
				if err != nil {
					cobra.CheckErr(fmt.Sprintf("Failed to marshal permissions: %s", err))
				}
				machineDeployment.Metadata[scheduler.MetadataKeyPermissions] = perms
			}
			if customDomains := extractCustomDomains(extraCfg); customDomains != "" {
				machineDeployment.Metadata[scheduler.MetadataKeyProxyCustomDomains] = customDomains
			}

			obtainMachine := func() (*buildRofl.Machine, *roflmarket.Instance, error) {
				if deployOffer != "" {
					machine.Offer = deployOffer
				}

				// Resolve offer.
				offers, err := fetchProviderOffers(ctx, npa, conn, *providerAddr)
				cobra.CheckErr(err)
				var offer *roflmarket.Offer
				for _, of := range offers {
					if of.Metadata[provider.SchedulerMetadataOfferKey] == machine.Offer || machine.Offer == "" {
						machine.Offer = of.Metadata[provider.SchedulerMetadataOfferKey]
						offer = of
						break
					}
				}
				if offer == nil {
					showProviderOffers(ctx, npa, conn, *providerAddr)
					return nil, nil, fmt.Errorf("offer '%s' not found for provider '%s'", machine.Offer, providerAddr)
				}

				fmt.Printf("Taking offer:\n")
				showProviderOffer(ctx, offer)

				term := detectTerm(offer)
				if roflCommon.TermCount < 1 {
					return nil, nil, fmt.Errorf("number of terms must be at least 1")
				}

				// Calculate total price.
				qTermCount := quantity.NewFromUint64(roflCommon.TermCount)
				totalPrice, ok := offer.Payment.Native.Terms[term]
				if !ok {
					cobra.CheckErr("internal error: previously selected term not found in offer terms")
				}
				cobra.CheckErr(totalPrice.Mul(qTermCount))
				tp := types.NewBaseUnits(totalPrice, offer.Payment.Native.Denomination)
				fmt.Printf("Selected per-%s pricing term, total price is ", term2str(term))
				tp.PrettyPrint(ctx, "", os.Stdout)
				fmt.Println(".")
				// Warn the user about the non-refundable rental policy before first renting.
				roflCommon.PrintRentRefundWarning()

				// Prepare transaction.
				tx := roflmarket.NewInstanceCreateTx(nil, &roflmarket.InstanceCreate{
					Provider:   *providerAddr,
					Offer:      offer.ID,
					Deployment: &machineDeployment,
					Term:       term,
					TermCount:  roflCommon.TermCount,
				})

				var sigTx, meta any
				sigTx, meta, err = common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
				cobra.CheckErr(err)

				var machineID roflmarket.InstanceID
				if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, &machineID) {
					return nil, nil, nil
				}

				rawMachineID, _ := machineID.MarshalText()
				machine.ID = string(rawMachineID)
				deployment.Machines[deployMachine] = machine

				fmt.Printf("Created machine: %s\n", machine.ID)

				var insDsc *roflmarket.Instance
				insDsc, err = conn.Runtime(npa.ParaTime).ROFLMarket.Instance(ctx, client.RoundLatest, *providerAddr, machineID)
				cobra.CheckErr(err)

				return machine, insDsc, nil
			}

			doDeployMachine := func(machine *buildRofl.Machine) (*roflmarket.Instance, error) {
				// Deploy into existing machine.
				var machineID roflmarket.InstanceID
				if err = machineID.UnmarshalText([]byte(machine.ID)); err != nil {
					return nil, fmt.Errorf("malformed machine ID: %w", err)
				}

				fmt.Printf("Deploying into existing machine: %s\n", machine.ID)

				// Sanity check the deployment.
				var insDsc *roflmarket.Instance
				insDsc, err = conn.Runtime(npa.ParaTime).ROFLMarket.Instance(ctx, client.RoundLatest, *providerAddr, machineID)

				// The "instance not found" error originates from Rust code,
				// so we can't compare it nicely here.
				if err != nil && strings.Contains(err.Error(), "instance not found") {
					if deployReplaceMachine {
						fmt.Printf("Machine instance not found. Obtaining new one...")
						machine.ID = ""
						_, insDsc, err = obtainMachine()
						return insDsc, err
					}

					cobra.CheckErr("Machine instance not found.\nTip: If your instance expired, run this command with --replace-machine flag to replace it with a new machine.")
				}
				cobra.CheckErr(err)

				if insDsc.Deployment != nil && insDsc.Deployment.AppID != machineDeployment.AppID && !deployForce {
					fmt.Printf("Machine already contains a deployment of ROFL app '%s'.\n", insDsc.Deployment.AppID)
					fmt.Printf("You are trying to replace it with ROFL app '%s'.\n", machineDeployment.AppID)
					return nil, fmt.Errorf("refusing to change existing ROFL app, use --force to override")
				}

				// Prepare transaction.
				tx := roflmarket.NewInstanceExecuteCmdsTx(nil, &roflmarket.InstanceExecuteCmds{
					Provider: *providerAddr,
					ID:       machineID,
					Cmds: [][]byte{cbor.Marshal(scheduler.Command{
						Method: scheduler.MethodDeploy,
						Args: cbor.Marshal(scheduler.DeployRequest{
							Deployment:  machineDeployment,
							WipeStorage: roflCommon.WipeStorage,
						}),
					})},
				})

				var sigTx, meta any
				sigTx, meta, err = common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
				cobra.CheckErr(err)

				if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
					return nil, nil
				}

				return insDsc, nil
			}

			var insDsc *roflmarket.Instance
			if machine.ID == "" {
				// When machine is not set, we need to obtain one.
				fmt.Printf("No pre-existing machine configured, creating a new one...\n")
				machine, insDsc, err = obtainMachine()
				cobra.CheckErr(err)
			} else {
				insDsc, err = doDeployMachine(machine)
				cobra.CheckErr(err)
			}

			if txCfg.Export {
				return
			}

			fmt.Printf("Deployment into machine scheduled.\n")
			fmt.Printf("This machine expires on %s. Use `oasis rofl machine top-up` to extend it.\n", time.Unix(int64(insDsc.PaidUntil), 0))
			fmt.Printf("Use `oasis rofl machine show` to check status.\n")

			if err = manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}
		},
	}
)

// ociRepository returns the OCI repository for the deployment (or a default one if not set).
func ociRepository(deployment *buildRofl.Deployment) string {
	if deployment.OCIRepository == "" {
		return fmt.Sprintf(
			"%s/%s:%d", buildRofl.DefaultOCIRegistry, uuid.New(), time.Now().Unix(),
		)
	}
	return deployment.OCIRepository
}

func pushBundleToOciRepository(orcFilename string, ociRepository string) (string, hash.Hash) {
	ociDigest, manifestHash, err := buildRofl.PushBundleToOciRepository(orcFilename, ociRepository)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		cobra.CheckErr(fmt.Sprintf("ROFL app bundle '%s' not found. Run `oasis rofl build` first.", orcFilename))
	default:
		cobra.CheckErr(fmt.Sprintf("Failed to push ROFL app to OCI repository: %s", err))
	}

	return ociDigest, manifestHash
}

// detectTerm returns the preferred (longest) period of the given offer.
func detectTerm(offer *roflmarket.Offer) (term roflmarket.Term) {
	if offer == nil {
		cobra.CheckErr(fmt.Errorf("no offers exist to determine payment term"))
		return // Linter complains otherwise.
	}
	if offer.Payment.Native == nil {
		cobra.CheckErr(fmt.Errorf("no payment terms available for offer '%s'", offer.ID))
	}

	if roflCommon.Term != "" {
		// Custom deploy term.
		term = roflCommon.ParseMachineTerm(roflCommon.Term)
		if _, ok := offer.Payment.Native.Terms[term]; !ok {
			cobra.CheckErr(fmt.Errorf("term '%s' is not available for offer '%s'", roflCommon.Term, offer.ID))
		}
		return
	}

	// Take the longest payment period.
	// TODO: Sort by actual periods (e.g. seconds) instead of internal roflmarket.Term index.
	for t := range offer.Payment.Native.Terms {
		if t > term {
			term = t
		}
	}
	return
}

func fetchProviderOffers(ctx context.Context, npa *common.NPASelection, conn connection.Connection, provider types.Address) (offers []*roflmarket.Offer, err error) {
	offers, err = conn.Runtime(npa.ParaTime).ROFLMarket.Offers(ctx, client.RoundLatest, provider)
	if err != nil {
		err = fmt.Errorf("failed to query provider: %s", err)
		return
	}
	// Order offers, newer first.
	sort.Slice(offers, func(i, j int) bool {
		return bytes.Compare(offers[i].ID[:], offers[j].ID[:]) > 0
	})
	return
}

func showProviderOffers(ctx context.Context, npa *common.NPASelection, conn connection.Connection, provider types.Address) {
	offers, err := fetchProviderOffers(ctx, npa, conn, provider)
	cobra.CheckErr(err)

	fmt.Println()
	fmt.Printf("Offers available from the selected provider:\n")
	for idx, offer := range offers {
		showProviderOffer(ctx, offer)
		if idx != len(offers)-1 {
			fmt.Println()
		}
	}
	fmt.Println()
}

func showProviderOffer(ctx context.Context, offer *roflmarket.Offer) {
	name, ok := offer.Metadata[provider.SchedulerMetadataOfferKey]
	if !ok {
		name = "<unnamed>"
	}

	var tee string
	switch offer.Resources.TEE {
	case roflmarket.TeeTypeSGX:
		tee = "sgx"
	case roflmarket.TeeTypeTDX:
		tee = "tdx"
	default:
		tee = "<unknown>"
	}

	fmt.Printf("- %s [%s]\n", name, offer.ID)
	fmt.Printf("  TEE: %s | Memory: %d MiB | vCPUs: %d | Storage: %.2f GiB\n",
		tee,
		offer.Resources.Memory,
		offer.Resources.CPUCount,
		float64(offer.Resources.Storage)/1024.,
	)
	if _, ok := offer.Metadata[provider.NoteMetadataKey]; ok {
		fmt.Printf("  Note: %s\n", offer.Metadata[provider.NoteMetadataKey])
	}
	if _, ok := offer.Metadata[provider.DescriptionMetadataKey]; ok {
		fmt.Printf("  Description:\n    %s\n", strings.ReplaceAll(offer.Metadata[provider.DescriptionMetadataKey], "\n", "\n    "))
	}
	if offer.Payment.Native != nil {
		if len(offer.Payment.Native.Terms) == 0 {
			return
		}

		// Specify sorting order for terms.
		terms := []roflmarket.Term{roflmarket.TermHour, roflmarket.TermMonth, roflmarket.TermYear}

		// Go through provided payment terms and print price.
		fmt.Printf("  Price: ")
		var gotPrev bool
		for _, term := range terms {
			price, exists := offer.Payment.Native.Terms[term]
			if !exists {
				continue
			}

			if gotPrev {
				fmt.Printf(" | ")
			}

			bu := types.NewBaseUnits(price, offer.Payment.Native.Denomination)
			bu.PrettyPrint(ctx, "", os.Stdout)
			fmt.Printf("/%s", term2str(term))
			gotPrev = true
		}
		fmt.Println()
	}
}

// Helper to convert roflmarket term into string.
func term2str(term roflmarket.Term) string {
	switch term {
	case roflmarket.TermHour:
		return "hour"
	case roflmarket.TermMonth:
		return "month"
	case roflmarket.TermYear:
		return "year"
	default:
		return "<unknown>"
	}
}

func resolveAndMarshalPermissions(npa *common.NPASelection, permissions map[string][]string) (string, error) {
	perms := make(scheduler.Permissions)
	for action, addresses := range permissions {
		perms[action] = make([]types.Address, len(addresses))
		for i, rawAddr := range addresses {
			addr, _, err := common.ResolveLocalAccountOrAddress(npa.Network, rawAddr)
			if err != nil {
				return "", err
			}

			perms[action][i] = *addr
		}
	}
	return scheduler.MarshalPermissions(perms), nil
}

func extractCustomDomains(extraCfg *roflCmdBuild.AppExtraConfig) string {
	if extraCfg == nil || len(extraCfg.Ports) == 0 {
		return ""
	}

	customDomains := make([]string, 0, len(extraCfg.Ports))
	for _, p := range extraCfg.Ports {
		if p.CustomDomain == "" {
			continue
		}
		customDomains = append(customDomains, p.CustomDomain)
	}
	return strings.Join(customDomains, " ")
}

func init() {
	providerFlags := flag.NewFlagSet("", flag.ContinueOnError)
	// Default to Testnet playground provider.
	providerFlags.StringVar(&deployProvider, "provider", "", "set the provider address")
	providerFlags.StringVar(&deployOffer, "offer", "", "set the provider's offer identifier")
	providerFlags.StringVar(&deployMachine, "machine", buildRofl.DefaultMachineName, "machine to deploy into")
	providerFlags.BoolVar(&deployForce, "force", false, "force deployment")
	providerFlags.BoolVar(&deployShowOffers, "show-offers", false, "show all provider offers and quit")
	providerFlags.BoolVar(&deployReplaceMachine, "replace-machine", false, "rent a new machine if the provided one expired")

	deployCmd.Flags().AddFlagSet(common.AccountFlag)
	deployCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	deployCmd.Flags().AddFlagSet(providerFlags)
	deployCmd.Flags().AddFlagSet(roflCommon.DeploymentFlags)
	deployCmd.Flags().AddFlagSet(roflCommon.WipeFlags)
	deployCmd.Flags().AddFlagSet(roflCommon.TermFlags)
}
