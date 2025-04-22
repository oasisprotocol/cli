package common

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/accounts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/core"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	coreSignature "github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	consensusPretty "github.com/oasisprotocol/oasis-core/go/common/prettyprint"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	consensusTx "github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/callformat"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/client"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/connection"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/helpers"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/wallet"
)

var (
	txOffline    bool
	txNonce      uint64
	txGasLimit   uint64
	txGasPrice   string
	txFeeDenom   string
	txEncrypted  bool
	txUnsigned   bool
	txFormat     string
	txOutputFile string
)

const (
	invalidNonce    = math.MaxUint64
	invalidGasLimit = math.MaxUint64

	formatJSON = "json"
	formatCBOR = "cbor"
)

var (
	// TxFlags contains the common consensus transaction flags.
	TxFlags *flag.FlagSet

	// RuntimeTxFlags contains the common runtime transaction flags.
	RuntimeTxFlags *flag.FlagSet
)

// TransactionConfig contains the transaction-related configuration from flags.
type TransactionConfig struct {
	// Offline is a flag indicating that no online queries are allowed.
	Offline bool

	// Export is a flag indicating that the transaction should be exported instead of broadcast.
	Export bool
}

// GetTransactionConfig returns the transaction-related configuration from flags.
func GetTransactionConfig() *TransactionConfig {
	return &TransactionConfig{
		Offline: txOffline,
		Export:  shouldExportTransaction(),
	}
}

// shouldExportTransaction returns true if the transaction should be exported instead of broadcast.
func shouldExportTransaction() bool {
	return txOffline || txUnsigned || txOutputFile != ""
}

// isRuntimeTx returns true, if given object is a signed or unsigned runtime transaction.
func isRuntimeTx(tx interface{}) bool {
	_, isRuntimeTx := tx.(*types.Transaction)
	if !isRuntimeTx {
		_, isRuntimeTx = tx.(*types.UnverifiedTransaction)
	}
	return isRuntimeTx
}

// PrepareConsensusTransaction initialized nonce and gas fields of the
// consensus transaction and estimates gas.
//
// Returns the estimated gas limit and total fee amount.
func PrepareConsensusTransaction(ctx context.Context, npa *NPASelection, signer coreSignature.Signer, conn connection.Connection, tx *consensusTx.Transaction) (consensusTx.Gas, *quantity.Quantity, error) {
	// Nonce is required for correct gas estimation.
	if tx.Nonce == 0 {
		tx.Nonce = txNonce
	}

	// Default to passed values and do online estimation when possible.
	if tx.Fee == nil {
		tx.Fee = &consensusTx.Fee{}
	}
	tx.Fee.Gas = consensusTx.Gas(txGasLimit)

	// Gas price estimation if not specified.
	gasPrice := quantity.NewQuantity()
	var err error
	if txGasPrice != "" {
		gasPrice, err = helpers.ParseConsensusDenomination(npa.Network, txGasPrice)
		if err != nil {
			return 0, nil, fmt.Errorf("bad gas price: %w", err)
		}
	}

	// Gas limit estimation if not specified.
	gas := consensusTx.Gas(txGasLimit)
	if !txOffline && gas == invalidGasLimit {
		gas, err = conn.Consensus().EstimateGas(ctx, &consensus.EstimateGasRequest{
			Signer:      signer.Public(),
			Transaction: tx,
		})
		if err != nil {
			return 0, nil, fmt.Errorf("failed to estimate gas: %w", err)
		}
	}

	// Compute the fee.
	fee := gasPrice.Clone()
	if err = fee.Mul(quantity.NewFromUint64(uint64(gas))); err != nil {
		return 0, nil, fmt.Errorf("failed to compute gas fee: %w", err)
	}
	return gas, fee, nil
}

// SignConsensusTransaction signs a consensus transaction.
func SignConsensusTransaction(
	ctx context.Context,
	npa *NPASelection,
	account wallet.Account,
	conn connection.Connection,
	tx *consensusTx.Transaction,
) (interface{}, error) {
	// Sanity checks.
	signer := account.ConsensusSigner()
	if signer == nil {
		return nil, fmt.Errorf("account does not support signing consensus transactions")
	}
	if txEncrypted {
		return nil, fmt.Errorf("--encrypted not supported for consensus transactions")
	}
	if txFeeDenom != "" {
		return nil, fmt.Errorf("consensus layer only supports the native denomination for paying fees")
	}

	gas, fee, err := PrepareConsensusTransaction(ctx, npa, signer, conn, tx)
	if err != nil {
		return nil, err
	}

	if tx.Fee.Gas == invalidGasLimit {
		tx.Fee.Gas = gas
		tx.Fee.Amount = *fee
	}

	// Query nonce if not specified.
	if !txOffline && tx.Nonce == invalidNonce {
		var nonce uint64
		nonce, err = conn.Consensus().GetSignerNonce(ctx, &consensus.GetSignerNonceRequest{
			AccountAddress: account.Address().ConsensusAddress(),
			Height:         consensus.HeightLatest,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to query nonce: %w", err)
		}
		tx.Nonce = nonce
	}

	// If we are using offline mode and either nonce or gas limit is not specified, abort.
	if tx.Nonce == invalidNonce || tx.Fee.Gas == invalidGasLimit {
		return nil, fmt.Errorf("nonce and/or gas limit must be specified in offline mode")
	}
	if txUnsigned {
		// Return an unsigned transaction.
		return tx, nil
	}

	PrintTransactionBeforeSigning(npa, tx)

	// Sign the transaction.
	// NOTE: We build our own domain separation context here as we need to support multiple chain
	//       contexts at the same time. Would be great if chainContextSeparator was exposed in core.
	sigCtx := coreSignature.Context([]byte(fmt.Sprintf("%s for chain %s", consensusTx.SignatureContext, npa.Network.ChainContext)))
	signed, err := coreSignature.SignSigned(signer, sigCtx, tx)
	if err != nil {
		return nil, err
	}

	return &consensusTx.SignedTransaction{Signed: *signed}, nil
}

// PrepareParatimeTransaction initializes nonce and gas fields of the ParaTime
// transaction and estimates gas.
//
// Returns the estimated gas limit, total fee amount and fee denominator.
func PrepareParatimeTransaction(ctx context.Context, npa *NPASelection, account wallet.Account, conn connection.Connection, tx *types.Transaction) (uint64, *quantity.Quantity, types.Denomination, error) {
	// Determine whether the signer information for a transaction has already been set.
	accountAddressSpec := account.SignatureAddressSpec()
	var hasSignerInfo bool
	for _, si := range tx.AuthInfo.SignerInfo {
		if si.AddressSpec.Signature == nil {
			continue
		}
		if !si.AddressSpec.Signature.PublicKey().Equal(accountAddressSpec.PublicKey()) {
			continue
		}
		hasSignerInfo = true
		break
	}

	var err error
	if !hasSignerInfo {
		nonce := txNonce
		// Query nonce if not specified.
		if !txOffline && nonce == invalidNonce {
			nonce, err = conn.Runtime(npa.ParaTime).Accounts.Nonce(ctx, client.RoundLatest, account.Address())
			if err != nil {
				return 0, nil, "", fmt.Errorf("failed to query nonce: %w", err)
			}
		}

		if nonce == invalidNonce {
			return 0, nil, "", fmt.Errorf("nonce must be specified in offline mode")
		}

		// Prepare the transaction before (optional) gas estimation to ensure correct estimation.
		tx.AppendAuthSignature(accountAddressSpec, nonce)
	}

	// Gas price estimation if not specified.
	gasPrice := &types.BaseUnits{}
	feeDenom := types.Denomination(txFeeDenom)
	if txGasPrice != "" {
		gasPrice, err = helpers.ParseParaTimeDenomination(npa.ParaTime, txGasPrice, feeDenom)
		if err != nil {
			return 0, nil, "", fmt.Errorf("bad gas price: %w", err)
		}
	} else if !txOffline {
		var mgp map[types.Denomination]types.Quantity
		mgp, err = conn.Runtime(npa.ParaTime).Core.MinGasPrice(ctx)
		if err != nil {
			return 0, nil, "", fmt.Errorf("failed to query minimum gas price: %w", err)
		}
		*gasPrice = types.NewBaseUnits(mgp[feeDenom], feeDenom)
	}

	// Gas limit estimation if not specified.
	gas := txGasLimit
	if gas == invalidGasLimit && !txOffline {
		gas, err = conn.Runtime(npa.ParaTime).Core.EstimateGas(ctx, client.RoundLatest, tx, false)
		if err != nil {
			return 0, nil, "", fmt.Errorf("failed to estimate gas: %w", err)
		}

		// Inflate the estimate by 20% for good measure.
		gas = (120 * gas) / 100
	}

	// Compute fee.
	fee := gasPrice.Amount.Clone()
	if err = fee.Mul(quantity.NewFromUint64(gas)); err != nil {
		return 0, nil, "", err
	}
	return gas, fee, feeDenom, nil
}

// SignParaTimeTransaction signs a ParaTime transaction.
//
// Returns the signed transaction and call format-specific metadata for result decoding.
func SignParaTimeTransaction(
	ctx context.Context,
	npa *NPASelection,
	account wallet.Account,
	conn connection.Connection,
	tx *types.Transaction,
	txDetails *signature.TxDetails,
) (interface{}, interface{}, error) {
	npa.MustHaveParaTime()

	gas, fee, feeDenom, err := PrepareParatimeTransaction(ctx, npa, account, conn, tx)
	if err != nil {
		return nil, nil, err
	}

	if tx.AuthInfo.Fee.Gas == 0 {
		tx.AuthInfo.Fee.Gas = gas
		tx.AuthInfo.Fee.Amount.Amount = *fee
		tx.AuthInfo.Fee.Amount.Denomination = feeDenom
	}

	// If we are using offline mode and gas limit is not specified, abort.
	if tx.AuthInfo.Fee.Gas == invalidGasLimit {
		return nil, nil, fmt.Errorf("gas limit must be specified in offline mode")
	}

	// Handle confidential transactions.
	var meta interface{}
	if txEncrypted {
		// Only online mode is supported for now.
		if txOffline {
			return nil, nil, fmt.Errorf("encrypted transactions are not available in offline mode")
		}

		// Request public key from the runtime.
		pk, err := conn.Runtime(npa.ParaTime).Core.CallDataPublicKey(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get runtime's call data public key: %w", err)
		}

		cfg := callformat.EncodeConfig{
			PublicKey: &pk.PublicKey,
		}
		var encCall *types.Call
		encCall, meta, err = callformat.EncodeCall(&tx.Call, types.CallFormatEncryptedX25519DeoxysII, &cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to encrypt call: %w", err)
		}

		tx.Call = *encCall
	}

	if txUnsigned {
		// Return an unsigned transaction.
		return tx, meta, nil
	}

	PrintTransactionBeforeSigning(npa, tx)

	// Sign the transaction.
	ts := tx.PrepareForSigning()
	sigCtx := &signature.RichContext{
		RuntimeID:    npa.ParaTime.Namespace(),
		ChainContext: npa.Network.ChainContext,
		Base:         types.SignatureContextBase,
		TxDetails:    txDetails,
	}
	if err := ts.AppendSign(sigCtx, account.Signer()); err != nil {
		return nil, nil, fmt.Errorf("failed to sign transaction: %w", err)
	}
	return ts.UnverifiedTransaction(), meta, nil
}

// PrintTransactionRaw prints the transaction which can be either signed or unsigned.
func PrintTransactionRaw(npa *NPASelection, tx interface{}) {
	switch tx.(type) {
	case consensusPretty.PrettyPrinter:
		fmt.Print(PrettyPrint(npa, "", tx))
	default:
		fmt.Printf("[unsupported transaction type: %T]\n", tx)
	}
}

// PrintTransaction prints the transaction which can be either signed or unsigned together with
// information about the selected network/ParaTime.
func PrintTransaction(npa *NPASelection, tx interface{}) {
	PrintTransactionRaw(npa, tx)

	fmt.Println()
	fmt.Printf("Network:  %s", npa.PrettyPrintNetwork())

	fmt.Println()
	if isRuntimeTx(tx) && npa.ParaTime != nil {
		fmt.Printf("ParaTime: %s", npa.ParaTimeName)
		if len(npa.ParaTime.Description) > 0 {
			fmt.Printf(" (%s)", npa.ParaTime.Description)
		}
		fmt.Println()
	} else {
		fmt.Println("ParaTime: none (consensus layer)")
	}
}

// PrintTransactionBeforeSigning prints the transaction and asks the user for confirmation.
func PrintTransactionBeforeSigning(npa *NPASelection, tx interface{}) {
	fmt.Printf("You are about to sign the following transaction:\n")

	PrintTransaction(npa, tx)

	fmt.Printf("Account:  %s", npa.AccountName)
	if len(npa.Account.Description) > 0 {
		fmt.Printf(" (%s)", npa.Account.Description)
	}
	fmt.Println()

	// Ask the user to confirm signing this transaction.
	Confirm("Sign this transaction?", "signing aborted")

	fmt.Println("(In case you are using a hardware-based signer you may need to confirm on device.)")
}

// ExportTransaction exports a (signed) transaction based on configuration.
func ExportTransaction(sigTx interface{}) {
	// Determine output destination.
	var err error
	outputFile := os.Stdout
	if txOutputFile != "" {
		outputFile, err = os.Create(txOutputFile)
		if err != nil {
			cobra.CheckErr(fmt.Errorf("failed to open output file: %w", err))
		}
		defer outputFile.Close()
	}

	// Determine output format.
	var data []byte
	switch txFormat {
	case formatJSON:
		data, err = json.MarshalIndent(sigTx, "", "  ")
		cobra.CheckErr(err)
	case formatCBOR:
		data = cbor.Marshal(sigTx)
	default:
		cobra.CheckErr(fmt.Errorf("unknown transaction format: %s", txFormat))
	}

	_, err = outputFile.Write(data)
	if err != nil {
		cobra.CheckErr(fmt.Errorf("failed to write output: %w", err))
	}
}

// BroadcastOrExportTransaction broadcasts or exports a transaction based on configuration.
//
// When in offline or unsigned mode, it exports the transaction and returns false. Otherwise
// it broadcasts the transaction and returns true.
func BroadcastOrExportTransaction(
	ctx context.Context,
	npa *NPASelection,
	conn connection.Connection,
	tx interface{},
	meta interface{},
	result interface{},
) bool {
	if shouldExportTransaction() {
		ExportTransaction(tx)
		return false
	}

	BroadcastTransaction(ctx, npa, conn, tx, meta, result)
	return true
}

// BroadcastTransaction broadcasts a transaction.
//
// When in offline mode, it outputs the transaction instead.
func BroadcastTransaction(
	ctx context.Context,
	npa *NPASelection,
	conn connection.Connection,
	tx interface{},
	meta interface{},
	result interface{},
) {
	switch sigTx := tx.(type) {
	case *consensusTx.SignedTransaction:
		// Consensus transaction.
		fmt.Printf("Broadcasting transaction...\n")
		err := conn.Consensus().SubmitTx(ctx, sigTx)
		cobra.CheckErr(err)

		fmt.Printf("Transaction executed successfully.\n")
		fmt.Printf("Transaction hash: %s\n", sigTx.Hash())
	case *types.UnverifiedTransaction:
		// ParaTime transaction.
		if npa == nil || npa.ParaTime == nil {
			cobra.CheckErr("no ParaTime configured for ParaTime transaction submission")
		}

		fmt.Printf("Broadcasting transaction...\n")
		rawMeta, err := conn.Runtime(npa.ParaTime).SubmitTxRawMeta(ctx, sigTx)
		cobra.CheckErr(err)

		if rawMeta.CheckTxError != nil {
			cobra.CheckErr(fmt.Sprintf("Transaction check failed with error: %s",
				PrettyErrorHints(ctx, npa, conn, tx, meta, &types.FailedCallResult{
					Module:  rawMeta.CheckTxError.Module,
					Code:    rawMeta.CheckTxError.Code,
					Message: rawMeta.CheckTxError.Message,
				})))
		}

		fmt.Printf("Transaction included in block successfully.\n")
		fmt.Printf("Round:            %d\n", rawMeta.Round)
		fmt.Printf("Transaction hash: %s\n", sigTx.Hash())

		if rawMeta.Result.IsUnknown() {
			fmt.Printf("                  (Transaction result is encrypted.)\n")
		}

		decResult, err := callformat.DecodeResult(&rawMeta.Result, meta)
		cobra.CheckErr(err)

		switch {
		case decResult.IsUnknown():
			// This should never happen as the inner result should not be unknown.
			cobra.CheckErr(fmt.Sprintf("Execution result unknown: %X", decResult.Unknown))
		case decResult.IsSuccess():
			fmt.Printf("Execution successful.\n")

			if result != nil {
				err = cbor.Unmarshal(decResult.Ok, result)
				cobra.CheckErr(err)
			}
		default:
			cobra.CheckErr(fmt.Sprintf("Execution failed with error: %s", PrettyErrorHints(ctx, npa, conn, tx, meta, decResult.Failed)))
		}
	default:
		panic(fmt.Errorf("unsupported transaction kind: %T", tx))
	}
}

// WaitForEvent waits for a specific ParaTime event.
//
// If no mapFn is specified, the returned channel will contain DecodedEvents, otherwise it will
// contain whatever mapFn returns.
//
// If mapFn is specified it should return a non-nil value when encountering a matching event.
func WaitForEvent(
	ctx context.Context,
	pt *config.ParaTime,
	conn connection.Connection,
	decoder client.EventDecoder,
	mapFn func(client.DecodedEvent) interface{},
) <-chan interface{} {
	ctx, cancel := context.WithCancel(ctx)

	// Start watching events.
	ch, err := conn.Runtime(pt).WatchEvents(ctx, []client.EventDecoder{decoder}, false)
	cobra.CheckErr(err)

	// Start processing events.
	resultCh := make(chan interface{})
	go func() {
		defer close(resultCh)
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				return
			case bev := <-ch:
				if bev == nil {
					return
				}

				for _, ev := range bev.Events {
					if result := mapFn(ev); result != nil {
						resultCh <- result
						return
					}
				}

				// TODO: Timeout.
			}
		}
	}()

	return resultCh
}

// PrettyErrorHints adds any hints based on the error and the transaction context.
func PrettyErrorHints(
	_ context.Context,
	npa *NPASelection,
	_ connection.Connection,
	_ interface{},
	_ interface{},
	failedRes *types.FailedCallResult,
) string {
	errMsg := failedRes.Error()
	if npa != nil && npa.ParaTime != nil &&
		npa.Network.ChainContext == config.DefaultNetworks.All["testnet"].ChainContext &&
		npa.ParaTime.ID == config.DefaultNetworks.All["testnet"].ParaTimes.All["sapphire"].ID &&
		(failedRes.Module == accounts.ModuleName && failedRes.Code == 2 || failedRes.Module == core.ModuleName && failedRes.Code == 5) {
		errMsg += "\nTip: You can get TEST tokens at https://faucet.testnet.oasis.io or #dev-central at https://oasis.io/discord."
	}
	if failedRes.Module == staking.ModuleName {
		if failedRes.Code == 5 {
			errMsg += "\nTip: Did you forget to run `oasis account allow`?"
		} else if failedRes.Code == 9 {
			errMsg += "\nTip: You can see minimum staking transfer amount by running `oasis network show parameters`"
		}
	}
	return errMsg
}

func init() {
	RuntimeTxFlags = flag.NewFlagSet("", flag.ContinueOnError)
	RuntimeTxFlags.BoolVar(&txOffline, "offline", false, "do not perform any operations requiring network access")
	RuntimeTxFlags.Uint64Var(&txNonce, "nonce", invalidNonce, "override nonce to use")
	RuntimeTxFlags.Uint64Var(&txGasLimit, "gas-limit", invalidGasLimit, "override gas limit to use (disable estimation)")
	RuntimeTxFlags.StringVar(&txGasPrice, "gas-price", "", "override gas price to use")
	RuntimeTxFlags.StringVar(&txFeeDenom, "fee-denom", "", "override fee denomination (defaults to native)")
	RuntimeTxFlags.BoolVar(&txEncrypted, "encrypted", false, "encrypt transaction call data (requires online mode)")
	RuntimeTxFlags.AddFlagSet(AnswerYesFlag)
	RuntimeTxFlags.BoolVar(&txUnsigned, "unsigned", false, "do not sign transaction")
	RuntimeTxFlags.StringVar(&txFormat, "format", "json", "transaction output format (for offline/unsigned modes) [json, cbor]")
	RuntimeTxFlags.StringVarP(&txOutputFile, "output-file", "o", "", "output transaction into specified file instead of broadcasting")

	TxFlags = flag.NewFlagSet("", flag.ContinueOnError)
	TxFlags.BoolVar(&txOffline, "offline", false, "do not perform any operations requiring network access")
	TxFlags.Uint64Var(&txNonce, "nonce", invalidNonce, "override nonce to use")
	TxFlags.Uint64Var(&txGasLimit, "gas-limit", invalidGasLimit, "override gas limit to use (disable estimation)")
	TxFlags.StringVar(&txGasPrice, "gas-price", "", "override gas price to use")
	TxFlags.AddFlagSet(AnswerYesFlag)
	TxFlags.BoolVar(&txUnsigned, "unsigned", false, "do not sign transaction")
	TxFlags.StringVar(&txFormat, "format", "json", "transaction output format (for offline/unsigned modes) [json, cbor]")
	TxFlags.StringVarP(&txOutputFile, "output-file", "o", "", "output transaction into specified file instead of broadcasting")
}
