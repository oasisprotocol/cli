package ledger

import (
	"fmt"

	coreSignature "github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"

	"github.com/oasisprotocol/cli/wallet"
)

type ledgerCoreSigner struct {
	path []uint32
	pk   coreSignature.PublicKey
	dev  *ledgerDevice
}

func (ls *ledgerCoreSigner) Public() coreSignature.PublicKey {
	return ls.pk
}

func (ls *ledgerCoreSigner) ContextSign(context coreSignature.Context, message []byte) ([]byte, error) {
	preparedContext, err := coreSignature.PrepareSignerContext(context)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed to prepare signing context: %w", err)
	}

	signature, err := ls.dev.SignEd25519(ls.path, preparedContext, message)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed to sign message: %w", err)
	}
	return signature, nil
}

func (ls *ledgerCoreSigner) String() string {
	return fmt.Sprintf("[ledger consensus signer: %s]", ls.pk)
}

func (ls *ledgerCoreSigner) Reset() {
	_ = ls.dev.Close()
}

type ledgerSigner struct {
	algorithm string
	path      []uint32
	pk        signature.PublicKey
	dev       *ledgerDevice
}

func (ls *ledgerSigner) Public() (pk signature.PublicKey) {
	return ls.pk
}

func (ls *ledgerSigner) ContextSign(metadata signature.Context, message []byte) ([]byte, error) {
	switch ls.algorithm {
	case wallet.AlgorithmEd25519Adr8, wallet.AlgorithmEd25519Legacy:
		return ls.dev.SignRtEd25519(ls.path, metadata, message)
	case wallet.AlgorithmSecp256k1Bip44:
		return ls.dev.SignRtSecp256k1(ls.path, metadata, message)
	case wallet.AlgorithmSr25519Adr8:
		return ls.dev.SignRtSr25519(ls.path, metadata, message)
	}

	return nil, fmt.Errorf("ledger: algorithm %s not supported", ls.algorithm)
}

func (ls *ledgerSigner) Sign(_ []byte) ([]byte, error) {
	return nil, fmt.Errorf("ledger: signing without context not supported")
}

func (ls *ledgerSigner) String() string {
	return fmt.Sprintf("[ledger runtime signer: %s]", ls.pk)
}

func (ls *ledgerSigner) Reset() {
	_ = ls.dev.Close()
}
