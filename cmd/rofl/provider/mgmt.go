package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wI2L/jsondiff"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"

	"github.com/oasisprotocol/cli/build/rofl/provider"
	"github.com/oasisprotocol/cli/cmd/common"
	cliConfig "github.com/oasisprotocol/cli/config"
)

var (
	initCmd = &cobra.Command{
		Use:   "init",
		Short: "Initialize a ROFL provider manifest",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)

			// Fail in case there is an existing manifest.
			if provider.ManifestExists() {
				cobra.CheckErr("refusing to overwrite existing manifest")
			}

			var schedulerApp rofl.AppID
			rawSchedulerApp := provider.DefaultRoflServices[npa.ParaTime.ID].Scheduler
			if rawSchedulerApp != "" {
				err := schedulerApp.UnmarshalText([]byte(rawSchedulerApp))
				cobra.CheckErr(err)
			}

			fmt.Printf("Scheduler app: %s\n", schedulerApp)

			// Create a default manifest.
			manifest := provider.Manifest{
				Network:        npa.NetworkName,
				ParaTime:       npa.ParaTimeName,
				Provider:       npa.AccountName,
				SchedulerApp:   schedulerApp,
				PaymentAddress: npa.AccountName,
			}
			err := manifest.Validate()
			cobra.CheckErr(err)

			// Serialize manifest and write it to file.
			err = manifest.Save()
			if err != nil {
				cobra.CheckErr(fmt.Errorf("failed to write manifest: %w", err))
			}

			fmt.Printf("Created manifest in '%s'.\n", manifest.SourceFileName())
			fmt.Printf("Edit the manifest to add desired configuration.\n")
			fmt.Printf("Then run `oasis rofl provider create` to register your ROFL provider.\n")
		},
	}

	createCmd = &cobra.Command{
		Use:   "create",
		Short: "Create a new ROFL provider",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			manifest := loadManifestAndSetNPA(cfg, npa)

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			// Prepare provider create transaction.
			create := roflmarket.ProviderCreate{
				Nodes:        manifest.Nodes,
				SchedulerApp: manifest.SchedulerApp,
				Metadata:     manifest.GetMetadata(),
			}

			// Resolve payment address.
			sdkAddr, ethAddr, err := common.ResolveLocalAccountOrAddress(npa.Network, manifest.PaymentAddress)
			switch {
			case err != nil:
				cobra.CheckErr(fmt.Errorf("invalid payment address: %w", err))
			case ethAddr != nil:
				var rawAddr [20]byte
				copy(rawAddr[:], ethAddr.Bytes())
				create.PaymentAddress.Eth = &rawAddr
			default:
				create.PaymentAddress.Native = sdkAddr
			}

			// Offers.
			for idx, offerCfg := range manifest.Offers {
				var offer *roflmarket.Offer
				offer, err = offerCfg.AsDescriptor(npa.ParaTime)
				if err != nil {
					cobra.CheckErr(fmt.Errorf("bad offer configuration %d: %w", idx, err))
				}

				create.Offers = append(create.Offers, *offer)
			}

			tx := roflmarket.NewProviderCreateTx(nil, &create)
			acc := common.LoadAccount(cfg, npa.AccountName)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
				return
			}
		},
	}

	updateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update a ROFL provider",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			manifest := loadManifestAndSetNPA(cfg, npa)

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			acc := common.LoadAccount(cfg, npa.AccountName)

			// Prepare provider update transaction.
			update := roflmarket.ProviderUpdate{
				Provider:     acc.Address(),
				Nodes:        manifest.Nodes,
				SchedulerApp: manifest.SchedulerApp,
				Metadata:     manifest.GetMetadata(),
			}

			// Resolve payment address.
			sdkAddr, ethAddr, err := common.ResolveLocalAccountOrAddress(npa.Network, manifest.PaymentAddress)
			switch {
			case err != nil:
				cobra.CheckErr(fmt.Errorf("invalid payment address: %w", err))
			case ethAddr != nil:
				var rawAddr [20]byte
				copy(rawAddr[:], ethAddr.Bytes())
				update.PaymentAddress.Eth = &rawAddr
			default:
				update.PaymentAddress.Native = sdkAddr
			}

			tx := roflmarket.NewProviderUpdateTx(nil, &update)
			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
				return
			}
		},
	}

	// TODO: Update offers. Diff and then add/update/remove.
	updateOffersCmd = &cobra.Command{
		Use:   "update-offers",
		Short: "Update offers of a ROFL provider",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			if txCfg.Offline {
				cobra.CheckErr("offline mode currently not supported")
			}

			manifest := loadManifestAndSetNPA(cfg, npa)

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			acc := common.LoadAccount(cfg, npa.AccountName)

			// Retrieve existing offers.
			existingOffers, err := conn.Runtime(npa.ParaTime).ROFLMarket.Offers(ctx, client.RoundLatest, acc.Address())
			cobra.CheckErr(err)
			existingOfferMap := make(map[string]*roflmarket.Offer)
			for _, offer := range existingOffers {
				offerID, ok := offer.Metadata[provider.SchedulerMetadataOfferKey]
				if !ok {
					fmt.Printf("WARNING: On-chain offer '%s' is missing '%s' metadata, ignoring.\n", offer.ID, provider.SchedulerMetadataOfferKey)
					continue
				}

				existingOfferMap[offerID] = offer
			}
			removeOffers := maps.Clone(existingOfferMap)

			// Determine what should be added, updated and/or removed.
			var (
				add    []roflmarket.Offer
				update []roflmarket.Offer
				remove []roflmarket.OfferID
			)
			for _, offer := range manifest.Offers {
				var offerDsc *roflmarket.Offer
				offerDsc, err = offer.AsDescriptor(npa.ParaTime)
				cobra.CheckErr(err)

				existingOffer, ok := existingOfferMap[offer.ID]
				switch ok {
				case true:
					// Offer already exists, check if it needs to be updated.
					offerDsc.ID = existingOffer.ID
					if !reflect.DeepEqual(existingOffer, offerDsc) {
						update = append(update, *offerDsc)
					}

					delete(removeOffers, offer.ID)
				case false:
					// Offer does not yet exist and should be added.
					add = append(add, *offerDsc)
				}

			}
			// Remove any offers that don't exist in the manifest.
			for _, offer := range removeOffers {
				remove = append(remove, offer.ID)
			}

			// Show a summary of changes.
			fmt.Printf("Going to perform the following updates:\n")

			fmt.Printf("Add offers:\n")
			if len(add) > 0 {
				for _, offer := range add {
					fmt.Printf("  - %s\n", offer.Metadata[provider.SchedulerMetadataOfferKey])
				}
			} else {
				fmt.Printf("  <none>\n")
			}

			fmt.Printf("Update offers:\n")
			if len(update) > 0 {
				for _, offer := range update {
					offerID := offer.Metadata[provider.SchedulerMetadataOfferKey]
					fmt.Printf("  - %s\n", offerID)

					oldOffer, _ := json.Marshal(existingOfferMap[offerID])
					newOffer, _ := json.Marshal(offer)

					var patch jsondiff.Patch
					patch, err = jsondiff.CompareJSON(oldOffer, newOffer)
					if err == nil {
						for _, p := range patch {
							path := strings.ReplaceAll(p.Path, "/", ".")
							newValue, _ := json.Marshal(p.Value)

							switch p.Type {
							case jsondiff.OperationAdd:
								fmt.Printf("    {add} %s = %s", path, newValue)
							case jsondiff.OperationReplace:
								fmt.Printf("    {replace} %s = %s", path, newValue)
							case jsondiff.OperationRemove:
								fmt.Printf("    {remove} %s", path)
							default:
								continue
							}
							fmt.Println()
						}
					}
				}
			} else {
				fmt.Printf("  <none>\n")
			}

			fmt.Printf("Remove offers:\n")
			if len(remove) > 0 {
				for _, offerID := range remove {
					fmt.Printf("  - %s\n", offerID)
				}
			} else {
				fmt.Printf("  <none>\n")
			}

			if len(add) == 0 && len(update) == 0 && len(remove) == 0 {
				fmt.Printf("Nothing to update.\n")
				return
			}

			// Prepare provider update offers transaction.
			tx := roflmarket.NewProviderUpdateOffersTx(nil, &roflmarket.ProviderUpdateOffers{
				Provider: acc.Address(),
				Add:      add,
				Update:   update,
				Remove:   remove,
			})

			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
				return
			}
		},
	}

	removeCmd = &cobra.Command{
		Use:   "remove",
		Short: "Remove a ROFL provider",
		Args:  cobra.NoArgs,
		Run: func(_ *cobra.Command, _ []string) {
			cfg := cliConfig.Global()
			npa := common.GetNPASelection(cfg)
			txCfg := common.GetTransactionConfig()

			loadManifestAndSetNPA(cfg, npa)

			// When not in offline mode, connect to the given network endpoint.
			ctx := context.Background()
			var conn connection.Connection
			if !txCfg.Offline {
				var err error
				conn, err = connection.Connect(ctx, npa.Network)
				cobra.CheckErr(err)
			}

			// Prepare provider remove transaction.
			acc := common.LoadAccount(cfg, npa.AccountName)
			tx := roflmarket.NewProviderRemoveTx(nil, &roflmarket.ProviderRemove{
				Provider: acc.Address(),
			})

			sigTx, meta, err := common.SignParaTimeTransaction(ctx, npa, acc, conn, tx, nil)
			cobra.CheckErr(err)

			if !common.BroadcastOrExportTransaction(ctx, npa, conn, sigTx, meta, nil) {
				return
			}
		},
	}
)

// loadManifestAndSetNPA loads the ROFL provider manifest and reconfigures the
// network/paratime/account selection.
//
// In case there is an error in loading the manifest, it aborts the application.
func loadManifestAndSetNPA(cfg *cliConfig.Config, npa *common.NPASelection) *provider.Manifest {
	manifest, err := maybeLoadManifestAndSetNPA(cfg, npa)
	cobra.CheckErr(err)
	return manifest
}

// maybeLoadManifestAndSetNPA loads the ROFL provider manifest and reconfigures the
// network/paratime/account selection.
//
// In case there is an error in loading the manifest, it is returned.
func maybeLoadManifestAndSetNPA(cfg *cliConfig.Config, npa *common.NPASelection) (*provider.Manifest, error) {
	manifest, err := provider.LoadManifest()
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
		npa.MustHaveParaTime()
	default:
		npa.ParaTime = npa.Network.ParaTimes.All[manifest.ParaTime]
		if npa.ParaTime == nil {
			return nil, fmt.Errorf("paratime '%s' does not exist", manifest.ParaTime)
		}
		npa.ParaTimeName = manifest.ParaTime
	}
	switch manifest.Provider {
	case "":
	default:
		accCfg, err := common.LoadAccountConfig(cfg, manifest.Provider)
		if err != nil {
			return nil, err
		}
		npa.Account = accCfg
		npa.AccountName = manifest.Provider
	}
	return manifest, nil
}

func init() {
	initCmd.Flags().AddFlagSet(common.SelectorFlags)

	createCmd.Flags().AddFlagSet(common.SelectorFlags)
	createCmd.Flags().AddFlagSet(common.RuntimeTxFlags)

	updateCmd.Flags().AddFlagSet(common.SelectorFlags)
	updateCmd.Flags().AddFlagSet(common.RuntimeTxFlags)

	updateOffersCmd.Flags().AddFlagSet(common.SelectorFlags)
	updateOffersCmd.Flags().AddFlagSet(common.RuntimeTxFlags)

	removeCmd.Flags().AddFlagSet(common.SelectorFlags)
	removeCmd.Flags().AddFlagSet(common.RuntimeTxFlags)
}
