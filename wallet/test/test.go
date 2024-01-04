package test

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	ethCommon "github.com/ethereum/go-ethereum/common"
	flag "github.com/spf13/pflag"

	coreSignature "github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/testing"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/wallet"
)

const (
	// Kind is the account kind for the test accounts.
	Kind = "test"
)

type testAccountFactory struct{}

func (af *testAccountFactory) Kind() string {
	return Kind
}

func (af *testAccountFactory) PrettyKind(rawCfg map[string]interface{}) string {
	var cfg wallet.AccountConfig
	if err := cfg.UnmarshalMap(rawCfg); err != nil {
		return ""
	}

	return fmt.Sprintf("test (%s)", cfg.Algorithm)
}

func (af *testAccountFactory) Flags() *flag.FlagSet {
	return nil
}

func (af *testAccountFactory) GetConfigFromFlags() (map[string]interface{}, error) {
	return nil, nil
}

func (af *testAccountFactory) GetConfigFromSurvey(_ *wallet.ImportKind) (map[string]interface{}, error) {
	return nil, fmt.Errorf("test: account is built-in")
}

func (af *testAccountFactory) DataPrompt(_ wallet.ImportKind, _ map[string]interface{}) survey.Prompt {
	return nil
}

func (af *testAccountFactory) DataValidator(_ wallet.ImportKind, _ map[string]interface{}) survey.Validator {
	return nil
}

func (af *testAccountFactory) RequiresPassphrase() bool {
	return false
}

func (af *testAccountFactory) SupportedImportKinds() []wallet.ImportKind {
	return []wallet.ImportKind{}
}

func (af *testAccountFactory) HasConsensusSigner(rawCfg map[string]interface{}) bool {
	var cfg wallet.AccountConfig
	if err := cfg.UnmarshalMap(rawCfg); err != nil {
		return false
	}

	return cfg.Algorithm == wallet.AlgorithmEd25519Raw
}

func (af *testAccountFactory) Migrate(_ map[string]interface{}) bool {
	return false
}

func (af *testAccountFactory) Create(_ string, _ string, _ map[string]interface{}) (wallet.Account, error) {
	return nil, fmt.Errorf("test: account is built-in")
}

func (af *testAccountFactory) Load(_ string, _ string, _ map[string]interface{}) (wallet.Account, error) {
	return nil, fmt.Errorf("test: account is built-in")
}

func (af *testAccountFactory) Remove(_ string, _ map[string]interface{}) error {
	return fmt.Errorf("test: account is built-in")
}

func (af *testAccountFactory) Rename(_, _ string, _ map[string]interface{}) error {
	return fmt.Errorf("test: account is built-in")
}

func (af *testAccountFactory) Import(_ string, _ string, _ map[string]interface{}, _ *wallet.ImportSource) (wallet.Account, error) {
	return nil, fmt.Errorf("test: import not supported")
}

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
	return a.testKey.EthAddress
}

func (a *testAccount) SignatureAddressSpec() types.SignatureAddressSpec {
	return a.testKey.SigSpec
}

func (a *testAccount) UnsafeExport() (string, string) {
	if a.testKey.SigSpec.Secp256k1Eth != nil {
		return hex.EncodeToString(a.testKey.SecretKey), ""
	}
	return base64.StdEncoding.EncodeToString(a.testKey.SecretKey), ""
}

func init() {
	wallet.Register(&testAccountFactory{})
}
