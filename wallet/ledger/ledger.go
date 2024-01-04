package ledger

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	ethCommon "github.com/ethereum/go-ethereum/common"
	flag "github.com/spf13/pflag"
	"golang.org/x/crypto/sha3"

	coreSignature "github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/ed25519"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/secp256k1"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/sr25519"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/wallet"
)

const (
	// Kind is the account kind for the ledger-backed accounts.
	Kind = "ledger"

	cfgAlgorithm = "ledger.algorithm"
	cfgNumber    = "ledger.number"
)

type ledgerAccountFactory struct {
	flags *flag.FlagSet
}

func (af *ledgerAccountFactory) Kind() string {
	return Kind
}

func (af *ledgerAccountFactory) PrettyKind(rawCfg map[string]interface{}) string {
	var cfg wallet.AccountConfig
	if err := cfg.UnmarshalMap(rawCfg); err != nil {
		return ""
	}

	// Show legacy, if algorithm not set.
	algorithm := cfg.Algorithm
	if algorithm == "" {
		algorithm = wallet.AlgorithmEd25519Adr8
	}
	return fmt.Sprintf("%s (%s:%d)", af.Kind(), algorithm, cfg.Number)
}

func (af *ledgerAccountFactory) Flags() *flag.FlagSet {
	return af.flags
}

func (af *ledgerAccountFactory) GetConfigFromFlags() (map[string]interface{}, error) {
	cfg := make(map[string]interface{})
	cfg["algorithm"], _ = af.flags.GetString(cfgAlgorithm)
	cfg["number"], _ = af.flags.GetUint32(cfgNumber)
	return cfg, nil
}

func (af *ledgerAccountFactory) GetConfigFromSurvey(_ *wallet.ImportKind) (map[string]interface{}, error) {
	return nil, fmt.Errorf("ledger: import not supported")
}

func (af *ledgerAccountFactory) DataPrompt(_ wallet.ImportKind, _ map[string]interface{}) survey.Prompt {
	return nil
}

func (af *ledgerAccountFactory) DataValidator(_ wallet.ImportKind, _ map[string]interface{}) survey.Validator {
	return nil
}

func (af *ledgerAccountFactory) RequiresPassphrase() bool {
	return false
}

func (af *ledgerAccountFactory) SupportedImportKinds() []wallet.ImportKind {
	return []wallet.ImportKind{}
}

func (af *ledgerAccountFactory) HasConsensusSigner(rawCfg map[string]interface{}) bool {
	var cfg wallet.AccountConfig
	if err := cfg.UnmarshalMap(rawCfg); err != nil {
		return false
	}

	switch cfg.Algorithm {
	case wallet.AlgorithmEd25519Legacy, wallet.AlgorithmEd25519Adr8:
		return true
	}
	return false
}

// Migrate migrates the given config ledger account entry to the latest version of the config and
// returns true, if any changes were needed.
func (af *ledgerAccountFactory) Migrate(rawCfg map[string]interface{}) bool {
	var changed bool

	// CONFIG MIGRATION 1 (add legacy derivation support): Set default derivation to ADR 8, if not set.
	if rawCfg["derivation"] == nil && rawCfg["algorithm"] == nil {
		rawCfg["derivation"] = "adr8"

		changed = true
	}

	// CONFIG MIGRATION 2 (convert derivation -> algorithm).
	if val, ok := rawCfg["derivation"]; ok {
		rawCfg["algorithm"] = fmt.Sprintf("ed25519-%s", val)
		delete(rawCfg, "derivation")

		changed = true
	}

	return changed
}

func (af *ledgerAccountFactory) Create(_ string, _ string, rawCfg map[string]interface{}) (wallet.Account, error) {
	var cfg wallet.AccountConfig
	if err := cfg.UnmarshalMap(rawCfg); err != nil {
		return nil, err
	}

	return newAccount(&cfg)
}

func (af *ledgerAccountFactory) Load(_ string, _ string, rawCfg map[string]interface{}) (wallet.Account, error) {
	var cfg wallet.AccountConfig
	if err := cfg.UnmarshalMap(rawCfg); err != nil {
		return nil, err
	}

	return newAccount(&cfg)
}

func (af *ledgerAccountFactory) Remove(_ string, _ map[string]interface{}) error {
	return nil
}

func (af *ledgerAccountFactory) Rename(_, _ string, _ map[string]interface{}) error {
	return nil
}

func (af *ledgerAccountFactory) Import(_ string, _ string, _ map[string]interface{}, _ *wallet.ImportSource) (wallet.Account, error) {
	return nil, fmt.Errorf("ledger: import not supported")
}

type ledgerAccount struct {
	cfg        *wallet.AccountConfig
	signer     *ledgerSigner
	coreSigner *ledgerCoreSigner
}

func newAccount(cfg *wallet.AccountConfig) (wallet.Account, error) {
	// Connect to device.
	dev, err := connectToDevice()
	if err != nil {
		return nil, err
	}

	// Retrieve public key.
	var path []uint32
	var pk signature.PublicKey
	var coreSigner *ledgerCoreSigner
	switch cfg.Algorithm {
	case wallet.AlgorithmEd25519Adr8, wallet.AlgorithmEd25519Legacy, "":
		path = getAdr0008Path(cfg.Number)
		if cfg.Algorithm == wallet.AlgorithmEd25519Legacy {
			path = getLegacyPath(cfg.Number)
		}
		rawPk, err := dev.GetPublicKey25519(path, wallet.AlgorithmEd25519Adr8, false)
		if err != nil {
			_ = dev.Close()
			return nil, err
		}
		// Create consensus layer signer.
		coreSigner = &ledgerCoreSigner{
			path: path,
			dev:  dev,
		}
		if err = coreSigner.pk.UnmarshalBinary(rawPk); err != nil {
			_ = dev.Close()
			return nil, fmt.Errorf("ledger: got malformed public key: %w", err)
		}
		var ed25519pk ed25519.PublicKey
		if err := ed25519pk.UnmarshalBinary(rawPk); err != nil {
			return nil, err
		}
		pk = ed25519pk
	case wallet.AlgorithmSecp256k1Bip44:
		path = getBip44Path(cfg.Number)
		rawPk, err := dev.GetPublicKeySecp256k1(path, false)
		if err != nil {
			_ = dev.Close()
			return nil, err
		}
		var secp256k1pk secp256k1.PublicKey
		if err := secp256k1pk.UnmarshalBinary(rawPk); err != nil {
			return nil, err
		}
		pk = secp256k1pk
	case wallet.AlgorithmSr25519Adr8:
		path = getAdr0008Path(cfg.Number)
		rawPk, err := dev.GetPublicKey25519(path, wallet.AlgorithmSr25519Adr8, false)
		if err != nil {
			_ = dev.Close()
			return nil, err
		}
		var sr25519pk sr25519.PublicKey
		if err := sr25519pk.UnmarshalBinary(rawPk); err != nil {
			return nil, err
		}
		pk = sr25519pk
	default:
		return nil, fmt.Errorf("unsupported algorithm %s", cfg.Algorithm)
	}

	// Create runtime signer.
	signer := &ledgerSigner{
		algorithm: cfg.Algorithm,
		path:      path,
		pk:        pk,
		dev:       dev,
	}

	return &ledgerAccount{
		cfg:        cfg,
		signer:     signer,
		coreSigner: coreSigner,
	}, nil
}

func (a *ledgerAccount) ConsensusSigner() coreSignature.Signer {
	switch a.cfg.Algorithm {
	case wallet.AlgorithmEd25519Adr8, wallet.AlgorithmEd25519Legacy:
		return a.coreSigner
	}

	return nil
}

func (a *ledgerAccount) Signer() signature.Signer {
	return a.signer
}

func (a *ledgerAccount) Address() types.Address {
	return types.NewAddress(a.SignatureAddressSpec())
}

func (a *ledgerAccount) EthAddress() *ethCommon.Address {
	if a.cfg.Algorithm == wallet.AlgorithmSecp256k1Bip44 {
		h := sha3.NewLegacyKeccak256()
		untaggedPk, _ := a.Signer().Public().(secp256k1.PublicKey).MarshalBinaryUncompressedUntagged()
		h.Write(untaggedPk)
		hash := h.Sum(nil)
		addr := ethCommon.BytesToAddress(hash[32-20:])
		return &addr
	}

	return nil
}

func (a *ledgerAccount) SignatureAddressSpec() types.SignatureAddressSpec {
	switch a.cfg.Algorithm {
	case "", wallet.AlgorithmEd25519Legacy, wallet.AlgorithmEd25519Adr8:
		return types.NewSignatureAddressSpecEd25519(a.Signer().Public().(ed25519.PublicKey))
	case wallet.AlgorithmSecp256k1Bip44:
		return types.NewSignatureAddressSpecSecp256k1Eth(a.Signer().Public().(secp256k1.PublicKey))
	case wallet.AlgorithmSr25519Adr8:
		return types.NewSignatureAddressSpecSr25519(a.Signer().Public().(sr25519.PublicKey))
	}
	return types.SignatureAddressSpec{}
}

func (a *ledgerAccount) UnsafeExport() (string, string) {
	// Secret is stored on the device.
	return "", ""
}

func init() {
	flags := flag.NewFlagSet("", flag.ContinueOnError)
	flags.String(cfgAlgorithm, wallet.AlgorithmEd25519Legacy, fmt.Sprintf("Cryptographic algorithm to use for this account [%s, %s, %s, %s]", wallet.AlgorithmEd25519Legacy, wallet.AlgorithmEd25519Adr8, wallet.AlgorithmSecp256k1Bip44, wallet.AlgorithmSr25519Adr8))
	flags.Uint32(cfgNumber, 0, "Key number to use in the derivation scheme")

	wallet.Register(&ledgerAccountFactory{
		flags: flags,
	})
}
