package common

import (
	"strings"
	"testing"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-core/go/common/quantity"

	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	sdkSignature "github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

func TestPrettyPrintWithTxDetails_PreservesUnnamedEthTo(t *testing.T) {
	require := require.New(t)

	pt := &sdkConfig.ParaTime{
		ID: strings.Repeat("0", 64),
		Denominations: map[string]*sdkConfig.DenominationInfo{
			sdkConfig.NativeDenominationKey: {
				Symbol:   "TEST",
				Decimals: 18,
			},
		},
	}

	npa := &NPASelection{
		NetworkName: "testnet",
		Network: &sdkConfig.Network{
			ChainContext: "test-chain-context",
			Denomination: sdkConfig.DenominationInfo{
				Symbol:   "TEST",
				Decimals: 9,
			},
		},
		ParaTimeName: "test-paratime",
		ParaTime:     pt,
	}

	ethAddr := ethCommon.HexToAddress("0x1111111111111111111111111111111111111111")
	to := types.NewAddressFromEth(ethAddr.Bytes())
	amt := types.NewBaseUnits(*quantity.NewFromUint64(0), types.NativeDenomination)
	tx := accounts.NewTransferTx(nil, &accounts.Transfer{
		To:     to,
		Amount: amt,
	})

	out := PrettyPrintWithTxDetails(npa, "", tx, &sdkSignature.TxDetails{OrigTo: &ethAddr})

	require.Contains(out, "To: "+ethAddr.Hex()+" ("+to.String()+")")
}
