package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/oasisprotocol/oasis-core/go/common"
	coreSignature "github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	consensusPretty "github.com/oasisprotocol/oasis-core/go/common/prettyprint"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/contracts"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
)

// PrettyJSONMarshal returns pretty-printed JSON encoding of v.
func PrettyJSONMarshal(v interface{}) ([]byte, error) {
	return PrettyJSONMarshalIndent(v, "", "  ")
}

// PrettyJSONMarshal returns pretty-printed JSON encoding of v.
func PrettyJSONMarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	formatted, err := json.MarshalIndent(v, prefix, indent)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal to pretty JSON: %w", err)
	}
	return formatted, nil
}

// JSONMarshalKey encodes k as UTF-8 string if valid, or Base64 otherwise.
func JSONMarshalKey(k interface{}) (keyJSON []byte, err error) {
	keyBytes, ok := k.([]byte)
	if ok && utf8.Valid(keyBytes) {
		// Marshal valid UTF-8 string.
		keyJSON, err = json.Marshal(string(keyBytes))
	} else {
		// Marshal string or Base64 otherwise.
		keyJSON, err = json.Marshal(k)
	}
	return keyJSON, err
}

// JSONPrettyPrintRoflAppConfig is a wrapper around rofl.AppConfig that implements custom JSON marshaling.
type JSONPrettyPrintRoflAppConfig rofl.AppConfig

// MarshalJSON implements custom JSON marshaling.
func (a JSONPrettyPrintRoflAppConfig) MarshalJSON() ([]byte, error) {
	type alias JSONPrettyPrintRoflAppConfig
	out := struct {
		alias
		// Overrides alias field so JSON marshaler uses this base64 string.
		SEK string `json:"sek"`
	}{
		alias: alias(a),
		SEK:   base64.StdEncoding.EncodeToString(a.SEK[:]),
	}
	return json.Marshal(out)
}

// JSONPrettyPrintRoflRegistration is a wrapper around rofl.Registration that implements custom JSON marshaling.
type JSONPrettyPrintRoflRegistration rofl.Registration

// MarshalJSON implements custom JSON marshaling.
func (r JSONPrettyPrintRoflRegistration) MarshalJSON() ([]byte, error) {
	type alias JSONPrettyPrintRoflRegistration
	out := struct {
		alias
		// Overrides alias field so JSON marshaler uses this base64 string.
		REK string `json:"rek"`
	}{
		alias: alias(r),
		REK:   base64.StdEncoding.EncodeToString(r.REK[:]),
	}
	return json.Marshal(out)
}

// JSONPrintKeyValueTuple traverses potentially large number of items and prints JSON representation
// of them.
//
// Marshalling is done externally without holding resulting JSON string in-memory.
// Cbor decoding of each value is tried first. If it fails, the binary content is preserved.
// Universal marshalling of map[interface{}]interface{} types is also supported.
// Each key is encoded as string if it contains valid UTF-8 value. Otherwise, Base64 is used.
func JSONPrintKeyValueTuple(items []contracts.InstanceStorageKeyValue) {
	first := true
	fmt.Printf("{")
	for _, kv := range items {
		if !first {
			fmt.Printf(",")
		}
		first = false
		var val interface{}
		err := cbor.Unmarshal(kv.Value, &val)
		if err != nil {
			// Value is not CBOR, use raw value instead.
			val = kv.Value
		}
		keyJSON, err := JSONMarshalKey(kv.Key)
		cobra.CheckErr(err)

		valJSON := JSONMarshalUniversalValue(val)
		fmt.Printf("%s:%s", keyJSON, valJSON)
	}
	fmt.Printf("}\n")
}

// JSONMarshalUniversalValue is a wrapper for the built-in JSON encoder which adds support for
// marshalling map[interface{}]interface{}.
//
// Each key is encoded as string if it contains valid UTF-8 value. Otherwise, Base64 is used.
func JSONMarshalUniversalValue(v interface{}) []byte {
	// Try array.
	if valTest, ok := v.([]interface{}); ok {
		e := make([]string, 0, len(valTest))
		for _, v := range valTest {
			valJSON := JSONMarshalUniversalValue(v)
			e = append(e, string(valJSON))
		}
		return []byte(fmt.Sprintf("[%s]", strings.Join(e, ",")))
	}

	// Try universal map.
	if valTest, ok := v.(map[interface{}]interface{}); ok {
		e := make([]string, 0, len(valTest))
		for k, v := range valTest {
			keyJSON, err := JSONMarshalKey(k)
			cobra.CheckErr(err)

			valJSON := JSONMarshalUniversalValue(v)

			e = append(e, fmt.Sprintf("%s:%s", keyJSON, valJSON))
		}
		return []byte(fmt.Sprintf("{%s}", strings.Join(e, ",")))
	}

	// Primitive type - use built-in JSON encoder.
	vJSON, err := json.Marshal(v)
	cobra.CheckErr(err)
	return vJSON
}

// PrettyPrint transforms generic JSON-formatted data into a pretty-printed string.
// For types implementing consensusPretty.PrettyPrinter, it uses the custom pretty printer.
// For other types, it does basic JSON indentation and cleanup of common delimiters.
func PrettyPrint(npa *NPASelection, prefix string, blob interface{}) string {
	return PrettyPrintWithTxDetails(npa, prefix, blob, nil)
}

// PrettyPrintWithTxDetails is like PrettyPrint but passes txDetails to the signature context.
func PrettyPrintWithTxDetails(npa *NPASelection, prefix string, blob interface{}, txDetails *signature.TxDetails) string {
	ret := ""
	switch rtx := blob.(type) {
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
			TxDetails:    txDetails,
		}
		ctx := context.Background()
		ctx = context.WithValue(ctx, consensusPretty.ContextKeyTokenSymbol, npa.Network.Denomination.Symbol)
		ctx = context.WithValue(ctx, consensusPretty.ContextKeyTokenValueExponent, npa.Network.Denomination.Decimals)
		if npa.ParaTime != nil {
			ctx = context.WithValue(ctx, config.ContextKeyParaTimeCfg, npa.ParaTime)
		}
		ctx = context.WithValue(ctx, signature.ContextKeySigContext, &sigCtx)

		// Provide locally-known names and native->ETH mapping for address formatting.
		addrCtx := GenAddressFormatContext()

		// Inject the original Ethereum "To" address into the eth map so that
		// FormatNamedAddressWith can prefer it over the native representation.
		if txDetails != nil && txDetails.OrigTo != nil {
			native := types.NewAddressFromEth(txDetails.OrigTo.Bytes()).String()
			addrCtx.Eth[native] = txDetails.OrigTo.Hex()
		}

		ctx = context.WithValue(ctx, types.ContextKeyAccountNames, addrCtx.Names)
		ctx = context.WithValue(ctx, types.ContextKeyAccountEthMap, addrCtx.Eth)

		// Set up chain context for signature verification during pretty-printing.
		coreSignature.UnsafeResetChainContext()
		coreSignature.SetChainContext(npa.Network.ChainContext)
		var pp strings.Builder
		rtx.PrettyPrint(ctx, prefix, &pp)
		ret = pp.String()
	default:
		pp, err := PrettyJSONMarshalIndent(blob, "", prefix)
		cobra.CheckErr(err)

		out := string(pp)
		out = strings.ReplaceAll(out, "{", "")
		out = strings.ReplaceAll(out, "}", "")
		out = strings.ReplaceAll(out, "[", "")
		out = strings.ReplaceAll(out, "]", "")
		out = strings.ReplaceAll(out, ",", "")
		out = strings.ReplaceAll(out, "\"", "")

		for _, line := range strings.Split(out, "\n") {
			line = strings.TrimRight(line, " \n")
			if len(line) == 0 {
				continue
			}
			ret += line + "\n"
		}
		ret = strings.TrimRight(ret, "\n")
	}

	return ret
}
