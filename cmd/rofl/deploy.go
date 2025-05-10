package rofl

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
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
	roflCommon "github.com/oasisprotocol/cli/cmd/rofl/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	deployProvider   string
	deployOffer      string
	deployMachine    string
	deployTerm       string
	deployTermCount  uint64
	deployForce      bool
	deployShowOffers bool

	deployCmd = &cobra.Command{
		Use:   "deploy",
		Short: "Deploy ROFL to a specified machine",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			if txCfg.Offline {
				cobra.CheckErr("offline mode currently not supported")
			}

			manifest, deployment := roflCommon.LoadManifestAndSetNPA(cfg, npa, deploymentName, &roflCommon.ManifestOptions{
				NeedAppID: true,
				NeedAdmin: true,
			})

			var appID rofl.AppID
			if err := appID.UnmarshalText([]byte(deployment.AppID)); err != nil {
				cobra.CheckErr(fmt.Sprintf("malformed app id: %s", err))
			}

			ctx := context.Background()
			conn, err := connection.Connect(ctx, npa.Network)
			cobra.CheckErr(err)

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
					if npa.ParaTime.ID == config.DefaultNetworks.All["testnet"].ParaTimes.All["sapphire"].ID {
						deployProvider = provider.DefaultRoflServices[npa.ParaTime.ID].Provider
					} else {
						cobra.CheckErr(fmt.Sprintf("Provider not configured for deployment '%s' machine '%s'. Please specify --provider.", deploymentName, deployMachine))
					}
				}

				machine.Provider = deployProvider
			default:
				// Already set, require the provider to be omitted or equal.
				if deployProvider != "" && deployProvider != machine.Provider {
					cobra.CheckErr(fmt.Sprintf("Provider '%s' conflicts with existing provider '%s' for deployment '%s' machine '%s'. Omit --provider.", deployProvider, machine.Provider, deploymentName, deployMachine))
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

			// Push ORC to OCI repository.
			if deployment.OCIRepository == "" {
				// Use default OCI repository with random name and timestamp tag.
				deployment.OCIRepository = fmt.Sprintf(
					"%s/%s:%d", buildRofl.DefaultOCIRegistry, uuid.New(), time.Now().Unix(),
				)
			}
			fmt.Printf("Pushing ROFL app to OCI repository '%s'...\n", deployment.OCIRepository)

			orcFilename := roflCommon.GetOrcFilename(manifest, deploymentName)
			ociDigest, manifestHash, err := buildRofl.PushBundleToOciRepository(orcFilename, deployment.OCIRepository)
			switch {
			case err == nil:
				// Save the OCI repository field to the configuration file so we avoid multiple uploads.
				if err := manifest.Save(); err != nil {
					cobra.CheckErr(fmt.Sprintf("Failed to update manifest with OCI repository: %s", err))
				}
			case errors.Is(err, os.ErrNotExist):
				cobra.CheckErr(fmt.Sprintf("ROFL app bundle '%s' not found. Run `oasis rofl build` first.", orcFilename))
			default:
				cobra.CheckErr(fmt.Sprintf("Failed to push ROFL app to OCI repository: %s", err))
			}

			// Define the deployment.
			machineDeployment := roflmarket.Deployment{
				AppID:        appID,
				ManifestHash: manifestHash,
				Metadata: map[string]string{
					"net.oasis.deployment.orc.ref": fmt.Sprintf("%s@%s", deployment.OCIRepository, ociDigest),
				},
			}

			switch machine.ID {
			case "":
				// When machine is not set, we need to obtain one.
				fmt.Printf("No pre-existing machine configured, creating a new one...\n")

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
					cobra.CheckErr(fmt.Sprintf("Offer '%s' not found for provider '%s'.\n", machine.Offer, providerAddr))
					return
				}

				fmt.Printf("Taking offer: %s [%s]\n", machine.Offer, offer.ID)

				term := detectTerm(offer)
				if deployTermCount < 1 {
					cobra.CheckErr("Number of terms must be at least 1.")
				}

				// Prepare transaction.
				tx := roflmarket.NewInstanceCreateTx(nil, &roflmarket.InstanceCreate{
					Provider:   *providerAddr,
					Offer:      offer.ID,
					Deployment: &machineDeployment,
					Term:       term,
					TermCount:  deployTermCount,
				})

				acc := common.LoadAccount(cfg, npa.AccountName)
				var sigTx, meta any
				sigTx, meta, err = common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
				cobra.CheckErr(err)

				var machineID roflmarket.InstanceID
				if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, &machineID) {
					return
				}

				rawMachineID, _ := machineID.MarshalText()
				cobra.CheckErr(err)
				machine.ID = string(rawMachineID)

				fmt.Printf("Created machine: %s\n", machine.ID)
			default:
				// Deploy into existing machine.
				var machineID roflmarket.InstanceID
				if err = machineID.UnmarshalText([]byte(machine.ID)); err != nil {
					cobra.CheckErr(fmt.Errorf("malformed machine ID: %w", err))
				}

				fmt.Printf("Deploying into existing machine: %s\n", machine.ID)

				// Sanity check the deployment.
				var insDsc *roflmarket.Instance
				insDsc, err = conn.Runtime(npa.ParaTime).ROFLMarket.Instance(ctx, client.RoundLatest, *providerAddr, machineID)
				cobra.CheckErr(err)
				if insDsc.Deployment != nil && insDsc.Deployment.AppID != machineDeployment.AppID && !deployForce {
					fmt.Printf("Machine already contains a deployment of ROFL app '%s'.\n", insDsc.Deployment.AppID)
					fmt.Printf("You are trying to replace it with ROFL app '%s'.\n", machineDeployment.AppID)
					cobra.CheckErr("Refusing to change existing ROFL app. Use --force to override.")
				}

				// Prepare transaction.
				tx := roflmarket.NewInstanceExecuteCmdsTx(nil, &roflmarket.InstanceExecuteCmds{
					Provider: *providerAddr,
					ID:       machineID,
					Cmds: [][]byte{cbor.Marshal(scheduler.Command{
						Method: scheduler.MethodDeploy,
						Args: cbor.Marshal(scheduler.DeployRequest{
							Deployment:  machineDeployment,
							WipeStorage: false,
						}),
					})},
				})

				acc := common.LoadAccount(cfg, npa.AccountName)
				var sigTx, meta any
				sigTx, meta, err = common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
				cobra.CheckErr(err)

				if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
					return
				}
			}

			fmt.Printf("Deployment into machine scheduled.\n")
			fmt.Printf("Use `oasis rofl machine show` to see status.\n")

			if err = manifest.Save(); err != nil {
				cobra.CheckErr(fmt.Errorf("failed to update manifest: %w", err))
			}
		},
	}
)

// detectTerm returns the preferred (longest) period of the given offer.
func detectTerm(offer *roflmarket.Offer) (term roflmarket.Term) {
	if offer == nil {
		cobra.CheckErr(fmt.Errorf("no offers exist to determine payment term"))
		return // Linter complains otherwise.
	}
	if offer.Payment.Native == nil {
		cobra.CheckErr(fmt.Errorf("no payment terms available for offer '%s'", offer.ID))
	}

	if deployTerm != "" {
		// Custom deploy term.
		term = roflCommon.ParseMachineTerm(deployTerm)
		if _, ok := offer.Payment.Native.Terms[term]; !ok {
			cobra.CheckErr(fmt.Errorf("term '%s' is not available for offer '%s'", deployTerm, offer.ID))
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
		showProviderOffer(offer)
		if idx != len(offers)-1 {
			fmt.Println()
		}
	}
	fmt.Println()
}

func showProviderOffer(offer *roflmarket.Offer) {
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

	// TODO: Show pricing.
}

func init() {
	providerFlags := flag.NewFlagSet("", flag.ContinueOnError)
	// Default to Testnet playground provider.
	providerFlags.StringVar(&deployProvider, "provider", "", "set the provider address")
	providerFlags.StringVar(&deployOffer, "offer", "", "set the provider's offer identifier")
	providerFlags.StringVar(&deployMachine, "machine", buildRofl.DefaultMachineName, "machine to deploy into")
	providerFlags.StringVar(&deployTerm, "term", "", "term to pay for in advance")
	providerFlags.Uint64Var(&deployTermCount, "term-count", 1, "number of terms to pay for in advance")
	providerFlags.BoolVar(&deployForce, "force", false, "force deployment")
	providerFlags.BoolVar(&deployShowOffers, "show-offers", false, "show all provider offers and quit")

	deployCmd.Flags().AddFlagSet(common.SelectorFlags)
	deployCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
	deployCmd.Flags().AddFlagSet(deploymentFlags)
	deployCmd.Flags().AddFlagSet(providerFlags)
}
