package provider

import (
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"gopkg.in/yaml.v3"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

// ManifestFileNames are the manifest file names that are tried when loading the manifest.
var ManifestFileNames = []string{
	"rofl-provider.yaml",
	"rofl-provider.yml",
}

// Manifest is the manifest describing a provider.
type Manifest struct {
	// Name is the optional name of the provider.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// Homepage is the optional homepage URL of the provider.
	Homepage string `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	// Description is the optional description of the provider.
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Network is the identifier of the network to deploy to.
	Network string `yaml:"network" json:"network"`
	// ParaTime is the identifier of the paratime to deploy to.
	ParaTime string `yaml:"paratime" json:"paratime"`
	// Provider is the identifier of the provider account.
	Provider string `yaml:"provider,omitempty" json:"provider,omitempty"`

	// Nodes are the public keys of nodes authorized to act on behalf of provider.
	Nodes []signature.PublicKey `yaml:"nodes" json:"nodes"`
	// ScheduperApp is the authorized scheduper app for this provider.
	SchedulerApp rofl.AppID `yaml:"scheduler_app" json:"scheduler_app"`
	// PaymentAddress is the payment address.
	PaymentAddress string `yaml:"payment_address" json:"payment_address"`
	// Offers is a list of offers available from this provider.
	Offers []*Offer `yaml:"offers,omitempty" json:"offers,omitempty"`
	// Metadata is arbitrary metadata (key-value pairs) assigned by the provider.
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`

	// sourceFn is the filename from which the manifest has been loaded.
	sourceFn string
}

// ManifestExists checks whether a manifest file exist. No attempt is made to load, parse or
// validate any of the found manifest files.
func ManifestExists() bool {
	for _, fn := range ManifestFileNames {
		_, err := os.Stat(fn)
		switch {
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return true
		}
	}
	return false
}

// LoadManifest attempts to find and load the ROFL app manifest from a local file.
func LoadManifest() (*Manifest, error) {
	for _, fn := range ManifestFileNames {
		f, err := os.Open(fn)
		switch {
		case err == nil:
		case errors.Is(err, os.ErrNotExist):
			continue
		default:
			return nil, fmt.Errorf("failed to load manifest from '%s': %w", fn, err)
		}

		var m Manifest
		dec := yaml.NewDecoder(f)
		if err = dec.Decode(&m); err != nil {
			f.Close()
			return nil, fmt.Errorf("malformed manifest '%s': %w", fn, err)
		}
		if err = m.Validate(); err != nil {
			f.Close()
			return nil, fmt.Errorf("invalid manifest '%s': %w", fn, err)
		}
		m.sourceFn, _ = filepath.Abs(f.Name()) // Record source filename.

		f.Close()
		return &m, nil
	}
	return nil, fmt.Errorf("no ROFL provider manifest found (tried: %s)", strings.Join(ManifestFileNames, ", "))
}

// Validate validates the provider manifest.
func (m *Manifest) Validate() error {
	if _, err := url.Parse(m.Homepage); err != nil && m.Homepage != "" {
		return fmt.Errorf("malformed homepage URL: %w", err)
	}

	for idx, offer := range m.Offers {
		if err := offer.Validate(); err != nil {
			return fmt.Errorf("invalid offer %d: %w", idx, err)
		}
	}
	return nil
}

// providerMetadataPrefix is the prefix used for all provider metadata.
const providerMetadataPrefix = "net.oasis.provider."

// GetMetadata derives metadata from the attributes defined in the manifest and combines it with
// the specified metadata.
func (m *Manifest) GetMetadata() map[string]string {
	meta := make(map[string]string)
	for _, md := range []struct {
		name  string
		value string
	}{
		{"name", m.Name},
		{"homepage", m.Homepage},
		{"description", m.Description},
	} {
		if md.value == "" {
			continue
		}
		meta[providerMetadataPrefix+md.name] = md.value
	}

	maps.Copy(meta, m.Metadata)
	return meta
}

// SourceFileName returns the filename of the manifest file from which the manifest was loaded or
// an empty string in case the filename is not available.
func (m *Manifest) SourceFileName() string {
	return m.sourceFn
}

// Save serializes the manifest and writes it to the file returned by `SourceFileName`, overwriting
// any previous manifest.
//
// If no previous source filename is available, a default one is set.
func (m *Manifest) Save() error {
	if m.sourceFn == "" {
		m.sourceFn = ManifestFileNames[0]
	}

	f, err := os.Create(m.sourceFn)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	return enc.Encode(m)
}

// Offer is a provider's offer.
type Offer struct {
	// ID is the unique offer identifier used by the scheduler.
	ID string `yaml:"id" json:"id"`
	// Resources are the offered resources.
	Resources Resources `yaml:"resources" json:"resources"`
	// Payment is the payment for this offer.
	Payment Payment `yaml:"payment" json:"payment"`
	// Capacity is the amount of available instances. Setting this to zero will disallow
	// provisioning of new instances for this offer. Each accepted instance will automatically
	// decrement capacity.
	Capacity uint64 `yaml:"capacity" json:"capacity"`
	// Metadata is arbitrary metadata (key-value pairs) assigned by the provider.
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// Validate validates the offer.
func (o *Offer) Validate() error {
	if o.ID == "" {
		return fmt.Errorf("missing offer identifier")
	}
	if err := o.Resources.Validate(); err != nil {
		return fmt.Errorf("invalid resources: %w", err)
	}
	if err := o.Payment.Validate(); err != nil {
		return fmt.Errorf("invalid payment specifier: %w", err)
	}
	return nil
}

// schedulerMetadataPrefix is the prefix used for all scheduler metadata.
const schedulerMetadataPrefix = "net.oasis.scheduler."

// SchedulerMetadataOfferKey is the metadata key used for the offer name.
const SchedulerMetadataOfferKey = schedulerMetadataPrefix + "offer"

// GetMetadata derives metadata from the attributes defined in the offer and combines it with the
// specified metadata.
func (o *Offer) GetMetadata() map[string]string {
	meta := make(map[string]string)
	for _, md := range []struct {
		name  string
		value string
	}{
		{"offer", o.ID},
	} {
		if md.value == "" {
			continue
		}
		meta[schedulerMetadataPrefix+md.name] = md.value
	}

	maps.Copy(meta, o.Metadata)
	return meta
}

// AsDescriptor returns the configuration as an on-chain descriptor.
func (o *Offer) AsDescriptor(pt *config.ParaTime) (*roflmarket.Offer, error) {
	offer := roflmarket.Offer{
		Resources: *o.Resources.AsDescriptor(),
		Capacity:  o.Capacity,
		Metadata:  o.GetMetadata(),
	}

	payment, err := o.Payment.AsDescriptor(pt)
	if err != nil {
		return nil, err
	}
	offer.Payment = *payment

	return &offer, nil
}

const (
	TermKeyHour  = "hourly"
	TermKeyMonth = "monthly"
	TermKeyYear  = "yearly"
)

// Payment is payment configuration for an offer.
type Payment struct {
	Native *struct {
		Denomination string            `yaml:"denomination" json:"denomination"`
		Terms        map[string]string `yaml:"terms" json:"terms"`
	} `yaml:"native,omitempty" json:"native,omitempty"`

	EvmContract *struct {
		Address string `json:"address"`
		Data    string `json:"data"`
	} `yaml:"evm,omitempty" json:"evm,omitempty"`
}

// Validate validates the payment configuration.
func (p *Payment) Validate() error {
	if p.Native != nil && p.EvmContract != nil {
		return fmt.Errorf("only one payment method may be specified")
	}
	switch {
	case p.Native != nil:
		for term := range p.Native.Terms {
			switch term {
			case TermKeyHour, TermKeyMonth, TermKeyYear:
			default:
				return fmt.Errorf("invalid term: %s", term)
			}
		}
	case p.EvmContract != nil:
		if !ethCommon.IsHexAddress(p.EvmContract.Address) {
			return fmt.Errorf("malformed Ethereum address: %s", p.EvmContract.Address)
		}
		_, err := hex.DecodeString(p.EvmContract.Data)
		if err != nil {
			return fmt.Errorf("malformed EVM contract data: %w", err)
		}
	default:
		return fmt.Errorf("missing payment configuration")
	}
	return nil
}

// AsDescriptor returns the configuration as an on-chain descriptor.
func (p *Payment) AsDescriptor(pt *config.ParaTime) (*roflmarket.Payment, error) {
	var dsc roflmarket.Payment
	switch {
	case p.Native != nil:
		dsc.Native = &roflmarket.NativePayment{
			Terms: make(map[roflmarket.Term]quantity.Quantity),
		}

		for termCfg, amount := range p.Native.Terms {
			var term roflmarket.Term
			switch termCfg {
			case TermKeyHour:
				term = roflmarket.TermHour
			case TermKeyMonth:
				term = roflmarket.TermMonth
			case TermKeyYear:
				term = roflmarket.TermYear
			default:
				return nil, fmt.Errorf("invalid term: %s", termCfg)
			}

			bu, err := helpers.ParseParaTimeDenomination(pt, amount, types.Denomination(p.Native.Denomination))
			if err != nil {
				return nil, fmt.Errorf("invalid amount: %w", err)
			}

			dsc.Native.Terms[term] = bu.Amount
			dsc.Native.Denomination = bu.Denomination
		}
	case p.EvmContract != nil:
		var addr [20]byte
		copy(addr[:], ethCommon.HexToAddress(p.EvmContract.Address).Bytes())

		data, err := hex.DecodeString(p.EvmContract.Data)
		if err != nil {
			return nil, fmt.Errorf("malformed EVM contract data: %w", err)
		}

		dsc.EvmContract = &roflmarket.EvmContractPayment{
			Address: addr,
			Data:    data,
		}
	}
	return &dsc, nil
}

// Resources are describe the offered resources.
type Resources struct {
	// TEE is the type of TEE hardware.
	TEE string `yaml:"tee" json:"tee"`
	// Memory is the amount of memory in megabytes.
	Memory uint64 `yaml:"memory" json:"memory"`
	// CPUCount is the amount of vCPUs.
	CPUCount uint16 `yaml:"cpus" json:"cpus"`
	// Storage is the amount of storage ine megabytes.
	Storage uint64 `yaml:"storage" json:"storage"`
	// GPU is the optional GPU resource.
	GPU *GPUResource `yaml:"gpu,omitempty" json:"gpu,omitempty"`
}

// Validate validates the resources.
func (r *Resources) Validate() error {
	switch r.TEE {
	case "sgx", "tdx":
	default:
		return fmt.Errorf("invalid TEE: %s", r.TEE)
	}

	if r.GPU != nil {
		err := r.GPU.Validate()
		if err != nil {
			return fmt.Errorf("invalid GPU resource: %w", err)
		}
	}
	return nil
}

// AsDescriptor returns the configuration as an on-chain descriptor.
func (r *Resources) AsDescriptor() *roflmarket.Resources {
	dsc := roflmarket.Resources{
		Memory:   r.Memory,
		CPUCount: r.CPUCount,
		Storage:  r.Storage,
	}

	switch r.TEE {
	case "sgx":
		dsc.TEE = roflmarket.TeeTypeSGX
	case "tdx":
		dsc.TEE = roflmarket.TeeTypeTDX
	default:
	}

	if r.GPU != nil {
		dsc.GPU = &roflmarket.GPUResource{
			Model: r.GPU.Model,
			Count: r.GPU.Count,
		}
	}

	return &dsc
}

// GPUResource is the offered GPU resource.
type GPUResource struct {
	// Model is the optional GPU model.
	Model string `yaml:"model,omitempty" json:"model,omitempty"`
	// Count is the number of GPUs requested.
	Count uint8 `yaml:"count" json:"count"`
}

// Validate validates the GPU resource.
func (g *GPUResource) Validate() error {
	return nil
}
