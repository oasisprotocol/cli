package test

import (
	"encoding/base64"
	"encoding/hex"

	ethCommon "github.com/ethereum/go-ethereum/common"
	coreSignature "github.com/oasisprotocol/oasis-core/go/common/crypto/signature"

	"github.com/oasisprotocol/cli/wallet"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/testing"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

const (
	// Kind is the account kind for the test accounts.
	Kind = "test"
)

type testAccount struct {
	testKey testing.TestKey
}

func NewTestAccount(testKey testing.TestKey) (wallet.Account, error) {
	return &testAccount{testKey: testKey}, nil
}

func (a *testAccount) ConsensusSigner() coreSignature.Signer {
	type wrappedSigner interface {
		Unwrap() coreSignature.Signer
	}

	if ws, ok := a.testKey.Signer.(wrappedSigner); ok {
		return ws.Unwrap()
	}
	return nil
}

func (a *testAccount) Signer() signature.Signer {
	return a.testKey.Signer
}

func (a *testAccount) Address() types.Address {
	return a.testKey.Address
}

func (a *testAccount) EthAddress() *ethCommon.Address {
	if a.testKey.SigSpec.Secp256k1Eth != nil {
		return &a.testKey.EthAddress
	}
	return nil
}

func (a *testAccount) SignatureAddressSpec() types.SignatureAddressSpec {
	return a.testKey.SigSpec
}

func (a *testAccount) UnsafeExport() string {
	if a.testKey.SigSpec.Secp256k1Eth != nil {
		return hex.EncodeToString(a.testKey.SecretKey)
	}
	return base64.StdEncoding.EncodeToString(a.testKey.SecretKey)
}
