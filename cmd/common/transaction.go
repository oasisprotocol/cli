package common

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	coreSignature "github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	consensusPretty "github.com/oasisprotocol/oasis-core/go/common/prettyprint"
	"github.com/oasisprotocol/oasis-core/go/common/quantity"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	consensusTx "github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"

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
	txEncrypted  bool
	txYes        bool
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
	// YesFlag corresponds to the yes-to-all flag.
	YesFlag *flag.FlagSet

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

// SignConsensusTransaction signs a consensus transaction.
func SignConsensusTransaction(
	ctx context.Context,
	npa *NPASelection,
	wallet wallet.Account,
	conn connection.Connection,
	tx *consensusTx.Transaction,
) (interface{}, error) {
	// Require consensus signer.
	signer := wallet.ConsensusSigner()
	if signer == nil {
		return nil, fmt.Errorf("account does not support signing consensus transactions")
	}

	if txEncrypted {
		return nil, fmt.Errorf("--encrypted not supported for consensus transactions")
	}

	// Default to passed values and do online estimation when possible.
	tx.Nonce = txNonce
	if tx.Fee == nil {
		tx.Fee = &consensusTx.Fee{}
	}
	tx.Fee.Gas = consensusTx.Gas(txGasLimit)

	gasPrice := quantity.NewQuantity()
	if txGasPrice != "" {
		var err error
		gasPrice, err = helpers.ParseConsensusDenomination(npa.Network, txGasPrice)
		if err != nil {
			return nil, fmt.Errorf("bad gas price: %w", err)
		}
	}

	if !txOffline { //nolint: nestif
		// Query nonce if not specified.
		if tx.Nonce == invalidNonce {
			nonce, err := conn.Consensus().GetSignerNonce(ctx, &consensus.GetSignerNonceRequest{
				AccountAddress: wallet.Address().ConsensusAddress(),
				Height:         consensus.HeightLatest,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to query nonce: %w", err)
			}
			tx.Nonce = nonce
		}

		// Gas estimation if not specified.
		if tx.Fee.Gas == invalidGasLimit {
			gas, err := conn.Consensus().EstimateGas(ctx, &consensus.EstimateGasRequest{
				Signer:      signer.Public(),
				Transaction: tx,
			})
			if err != nil {
				return nil, fmt.Errorf("failed to estimate gas: %w", err)
			}
			tx.Fee.Gas = gas
		}
	}

	// If we are using offline mode and either nonce or gas limit is not specified, abort.
	if tx.Nonce == invalidNonce || tx.Fee.Gas == invalidGasLimit {
		return nil, fmt.Errorf("nonce and/or gas limit must be specified in offline mode")
	}

	// Compute fee amount based on gas price.
	if err := gasPrice.Mul(quantity.NewFromUint64(uint64(tx.Fee.Gas))); err != nil {
		return nil, err
	}
	tx.Fee.Amount = *gasPrice

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
	if npa.ParaTime == nil {
		return nil, nil, fmt.Errorf("no ParaTime configured for ParaTime transaction signing")
	}

	// Determine whether the signer information for a transaction has already been set.
	var hasSignerInfo bool
	accountAddressSpec := account.SignatureAddressSpec()
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

	// Default to passed values and do online estimation when possible.
	if tx.AuthInfo.Fee.Gas == 0 {
		tx.AuthInfo.Fee.Gas = txGasLimit
	}

	gasPrice := &types.BaseUnits{}
	if txGasPrice != "" {
		// TODO: Support different denominations for gas fees.
		var err error
		gasPrice, err = helpers.ParseParaTimeDenomination(npa.ParaTime, txGasPrice, types.NativeDenomination)
		if err != nil {
			return nil, nil, fmt.Errorf("bad gas price: %w", err)
		}
	}

	if !hasSignerInfo {
		nonce := txNonce

		// Query nonce if not specified.
		if !txOffline && nonce == invalidNonce {
			var err error
			nonce, err = conn.Runtime(npa.ParaTime).Accounts.Nonce(ctx, client.RoundLatest, account.Address())
			if err != nil {
				return nil, nil, fmt.Errorf("failed to query nonce: %w", err)
			}
		}

		if nonce == invalidNonce {
			return nil, nil, fmt.Errorf("nonce must be specified in offline mode")
		}

		// Prepare the transaction before (optional) gas estimation to ensure correct estimation.
		tx.AppendAuthSignature(account.SignatureAddressSpec(), nonce)
	}

	if !txOffline { //nolint: nestif
		// Gas estimation if not specified.
		if tx.AuthInfo.Fee.Gas == invalidGasLimit {
			var err error
			tx.AuthInfo.Fee.Gas, err = conn.Runtime(npa.ParaTime).Core.EstimateGas(ctx, client.RoundLatest, tx, false)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to estimate gas: %w", err)
			}
		}

		// Gas price determination if not specified.
		if txGasPrice == "" {
			mgp, err := conn.Runtime(npa.ParaTime).Core.MinGasPrice(ctx)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to query minimum gas price: %w", err)
			}

			// TODO: Support different denominations for gas fees.
			denom := types.NativeDenomination
			*gasPrice = types.NewBaseUnits(mgp[denom], denom)
		}
	}

	// If we are using offline mode and gas limit is not specified, abort.
	if tx.AuthInfo.Fee.Gas == invalidGasLimit {
		return nil, nil, fmt.Errorf("gas limit must be specified in offline mode")
	}

	// Compute fee amount based on gas price.
	if err := gasPrice.Amount.Mul(quantity.NewFromUint64(tx.AuthInfo.Fee.Gas)); err != nil {
		return nil, nil, err
	}
	tx.AuthInfo.Fee.Amount.Amount = gasPrice.Amount
	tx.AuthInfo.Fee.Amount.Denomination = gasPrice.Denomination

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

// PrintTransaction prints the transaction which can be either signed or unsigned.
func PrintTransactionRaw(npa *NPASelection, tx interface{}) {
	switch rtx := tx.(type) {
	case consensusPretty.PrettyPrinter:
		// Signed or unsigned consensus or runtime transaction.
		var ns common.Namespace
		if npa.ParaTime != nil {
			ns = npa.ParaTime.Namespace()
		}
		sigCtx := signature.RichContext{
			RuntimeID:    ns,
			ChainContext: npa.Network.ChainContext,
			Base:         types.SignatureContextBase,
		}
		ctx := context.Background()
		ctx = context.WithValue(ctx, consensusPretty.ContextKeyTokenSymbol, npa.Network.Denomination.Symbol)
		ctx = context.WithValue(ctx, consensusPretty.ContextKeyTokenValueExponent, npa.Network.Denomination.Decimals)
		if npa.ParaTime != nil {
			ctx = context.WithValue(ctx, config.ContextKeyParaTimeCfg, npa.ParaTime)
		}
		ctx = context.WithValue(ctx, signature.ContextKeySigContext, &sigCtx)
		ctx = context.WithValue(ctx, types.ContextKeyAccountNames, GenAccountNames())

		// Set up chain context for signature verification during pretty-printing.
		coreSignature.UnsafeResetChainContext()
		coreSignature.SetChainContext(npa.Network.ChainContext)
		rtx.PrettyPrint(ctx, "", os.Stdout)
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
	pt *config.ParaTime,
	conn connection.Connection,
	tx interface{},
	meta interface{},
	result interface{},
) bool {
	if shouldExportTransaction() {
		ExportTransaction(tx)
		return false
	}

	BroadcastTransaction(ctx, pt, conn, tx, meta, result)
	return true
}

// BroadcastTransaction broadcasts a transaction.
//
// When in offline mode, it outputs the transaction instead.
func BroadcastTransaction(
	ctx context.Context,
	pt *config.ParaTime,
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
		if pt == nil {
			cobra.CheckErr("no ParaTime configured for ParaTime transaction submission")
		}

		fmt.Printf("Broadcasting transaction...\n")
		rawMeta, err := conn.Runtime(pt).SubmitTxRawMeta(ctx, sigTx)
		cobra.CheckErr(err)

		if rawMeta.CheckTxError != nil {
			cobra.CheckErr(fmt.Sprintf("Transaction check failed with error: module: %s code: %d message: %s",
				rawMeta.CheckTxError.Module,
				rawMeta.CheckTxError.Code,
				rawMeta.CheckTxError.Message,
			))
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
			cobra.CheckErr(fmt.Sprintf("Execution failed with error: %s", decResult.Failed.Error()))
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

func init() {
	YesFlag = flag.NewFlagSet("", flag.ContinueOnError)
	YesFlag.BoolVarP(&txYes, "yes", "y", false, "answer yes to all questions")

	RuntimeTxFlags = flag.NewFlagSet("", flag.ContinueOnError)
	RuntimeTxFlags.BoolVar(&txOffline, "offline", false, "do not perform any operations requiring network access")
	RuntimeTxFlags.Uint64Var(&txNonce, "nonce", invalidNonce, "override nonce to use")
	RuntimeTxFlags.Uint64Var(&txGasLimit, "gas-limit", invalidGasLimit, "override gas limit to use (disable estimation)")
	RuntimeTxFlags.StringVar(&txGasPrice, "gas-price", "", "override gas price to use")
	RuntimeTxFlags.BoolVar(&txEncrypted, "encrypted", false, "encrypt transaction call data (requires online mode)")
	RuntimeTxFlags.AddFlagSet(YesFlag)
	RuntimeTxFlags.BoolVar(&txUnsigned, "unsigned", false, "do not sign transaction")
	RuntimeTxFlags.StringVar(&txFormat, "format", "json", "transaction output format (for offline/unsigned modes) [json, cbor]")
	RuntimeTxFlags.StringVarP(&txOutputFile, "output-file", "o", "", "output transaction into specified file instead of broadcasting")

	TxFlags = flag.NewFlagSet("", flag.ContinueOnError)
	TxFlags.BoolVar(&txOffline, "offline", false, "do not perform any operations requiring network access")
	TxFlags.Uint64Var(&txNonce, "nonce", invalidNonce, "override nonce to use")
	TxFlags.Uint64Var(&txGasLimit, "gas-limit", invalidGasLimit, "override gas limit to use (disable estimation)")
	TxFlags.StringVar(&txGasPrice, "gas-price", "", "override gas price to use")
	TxFlags.AddFlagSet(YesFlag)
	TxFlags.BoolVar(&txUnsigned, "unsigned", false, "do not sign transaction")
	TxFlags.StringVar(&txFormat, "format", "json", "transaction output format (for offline/unsigned modes) [json, cbor]")
	TxFlags.StringVarP(&txOutputFile, "output-file", "o", "", "output transaction into specified file instead of broadcasting")
}
