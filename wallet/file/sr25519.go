package file

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/sakg"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/slip10"
	sdkSignature "github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/sr25519"
	"github.com/tyler-smith/go-bip39"
)

// Sr25519FromMnemonic derives a signer using ADR-8 from given mnemonic.
func Sr25519FromMnemonic(mnemonic string, number uint32) (sdkSignature.Signer, error) {
	if number > sakg.MaxAccountKeyNumber {
		return nil, fmt.Errorf(
			"sakg: invalid key number: %d (maximum: %d)",
			number,
			sakg.MaxAccountKeyNumber,
		)
	}

	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("sakg: invalid mnemonic")
	}

	seed := bip39.NewSeed(mnemonic, "")

	signer, chainCode, err := slip10.NewMasterKey(seed)
	if err != nil {
		return nil, fmt.Errorf("sakg: error deriving master key: %w", err)
	}

	pathStr := fmt.Sprintf("%s/%d'", sakg.BIP32PathPrefix, number)
	path, err := sakg.NewBIP32Path(pathStr)
	if err != nil {
		return nil, fmt.Errorf("sakg: error creating BIP-0032 path %s: %w", pathStr, err)
	}

	for _, index := range path {
		signer, chainCode, err = slip10.NewChildKey(signer, chainCode, index)
		if err != nil {
			return nil, fmt.Errorf("sakg: error deriving child key: %w", err)
		}
	}

	sr25519signer, err := sr25519.NewSigner(signer.(signature.UnsafeSigner).UnsafeBytes())
	if err != nil {
		return nil, err
	}
	return sr25519signer, nil
}
