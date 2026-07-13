package common

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	ethAbi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

// subcallAddress is the address of the "Subcall" precompile that dispatches
// ABI-encoded (method, CBOR body) calldata as an SDK runtime call. See
// https://github.com/oasisprotocol/oasis-sdk/blob/main/runtime-sdk/modules/evm/src/precompile/subcall.rs.
const subcallAddress = "0x0100000000000000000000000000000000000103"

// safeTxBuilderTransaction is a single call within a Safe Transaction Builder batch.
type safeTxBuilderTransaction struct {
	To                   string      `json:"to"`
	Value                string      `json:"value"`
	Data                 string      `json:"data"`
	ContractMethod       interface{} `json:"contractMethod"`
	ContractInputsValues interface{} `json:"contractInputsValues"`
}

// safeTxBuilderBatch is the file format understood by Safe's Transaction Builder app.
// See https://help.safe.global/en/articles/40841-transaction-builder.
type safeTxBuilderBatch struct {
	Version   string `json:"version"`
	ChainID   string `json:"chainId"`
	CreatedAt int64  `json:"createdAt"`
	Meta      struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"meta"`
	Transactions []safeTxBuilderTransaction `json:"transactions"`
}

// exportSafe wraps an unsigned Oasis transaction into a calldata for the
// Subcall precompile, and packages it for Safe.
func exportSafe(pt *config.ParaTime, sigTx interface{}) ([]byte, error) {
	tx, ok := sigTx.(*types.Transaction)
	if !ok || pt == nil {
		return nil, fmt.Errorf("--format safe is only supported for unsigned Sapphire transactions")
	}
	var chainID uint64
	switch pt.ID {
	case config.DefaultNetworks.All["mainnet"].ParaTimes.All["sapphire"].ID:
		chainID = 23294 // Sapphire Mainnet
	case config.DefaultNetworks.All["testnet"].ParaTimes.All["sapphire"].ID:
		chainID = 23295 // Sapphire Testnet
	default:
		return nil, fmt.Errorf("--format safe is only supported on Sapphire")
	}
	stringType, err := ethAbi.NewType("string", "", nil)
	if err != nil {
		return nil, err
	}
	bytesType, err := ethAbi.NewType("bytes", "", nil)
	if err != nil {
		return nil, err
	}
	calldata, err := ethAbi.Arguments{{Type: stringType}, {Type: bytesType}}.Pack(string(tx.Call.Method), []byte(tx.Call.Body))
	if err != nil {
		return nil, fmt.Errorf("failed to ABI-encode subcall: %w", err)
	}

	batch := safeTxBuilderBatch{
		Version:   "1.0",
		ChainID:   strconv.FormatUint(chainID, 10),
		CreatedAt: time.Now().UnixMilli(),
		Transactions: []safeTxBuilderTransaction{{
			To:    subcallAddress,
			Value: "0",
			Data:  "0x" + hex.EncodeToString(calldata),
		}},
	}
	batch.Meta.Name = string(tx.Call.Method)
	batch.Meta.Description = fmt.Sprintf("Oasis CLI-generated Subcall wrapping '%s' for proposal via Safe.", tx.Call.Method)

	return json.MarshalIndent(batch, "", "  ")
}

// DecodeSafe attempts to decode raw bytes as a Safe Transaction Builder batch
// and unwraps Subcall precompile calldata back into an unsigned ParaTime
// transaction.
//
// The resulting transaction only has Call.Method and Call.Body populated, since
// the Safe batch does not carry sender, nonce, or fee information.
func DecodeSafe(raw []byte) (*types.Transaction, error) {
	var batch safeTxBuilderBatch
	if err := json.Unmarshal(raw, &batch); err != nil {
		return nil, err
	}
	if batch.Version == "" || len(batch.Transactions) != 1 {
		return nil, fmt.Errorf("file not a Safe Transaction Builder batch with a single transaction")
	}

	txn := batch.Transactions[0]
	if !strings.EqualFold(txn.To, subcallAddress) {
		return nil, fmt.Errorf("transaction does not call the Subcall precompile at %s", subcallAddress)
	}

	calldata, err := hex.DecodeString(strings.TrimPrefix(txn.Data, "0x"))
	if err != nil {
		return nil, fmt.Errorf("malformed calldata: %w", err)
	}

	stringTy, err := ethAbi.NewType("string", "", nil)
	if err != nil {
		return nil, err
	}
	bytesTy, err := ethAbi.NewType("bytes", "", nil)
	if err != nil {
		return nil, err
	}
	vals, err := (ethAbi.Arguments{{Type: stringTy}, {Type: bytesTy}}).Unpack(calldata)
	if err != nil {
		return nil, fmt.Errorf("failed to ABI-decode subcall data: %w", err)
	}

	tx := &types.Transaction{Versioned: cbor.NewVersioned(types.LatestTransactionVersion)}
	tx.Call.Method = types.MethodName(vals[0].(string))
	tx.Call.Body = vals[1].([]byte)
	tx.AuthInfo.Fee.Amount = types.NewBaseUnits(*quantity.NewFromUint64(0), types.NativeDenomination)
	return tx, nil
}
