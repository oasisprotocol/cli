package ledger

import (
	"fmt"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	ethCommon "github.com/ethereum/go-ethereum/common"
	ledger_go "github.com/zondax/ledger-go"
	"golang.org/x/crypto/sha3"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/ed25519"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/sr25519"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/cli/wallet"
)

// NOTE: Some of this is lifted from https://github.com/oasisprotocol/oasis-core-ledger but updated
//       to conform to the ADR 0008 derivation scheme.

const (
	userMessageChunkSize = 250

	claConsumer = 0x05

	insGetVersion       = 0
	insGetAddrEd25519   = 1
	insSignEd25519      = 2
	insGetAddrSr25519   = 3
	insGetAddrSecp256k1 = 4
	insSignRtEd25519    = 5
	insSignRtSr25519    = 6
	insSignRtSecp256k1  = 7

	payloadChunkInit = 0
	payloadChunkAdd  = 1
	payloadChunkLast = 2

	errMsgInvalidParameters = "[APDU_CODE_BAD_KEY_HANDLE] The parameters in the data field are incorrect"
	errMsgInvalidated       = "[APDU_CODE_DATA_INVALID] Referenced data reversibly blocked (invalidated)"
	errMsgRejected          = "[APDU_CODE_COMMAND_NOT_ALLOWED] Sign request rejected"
)

type VersionInfo struct {
	Major uint8
	Minor uint8
	Patch uint8
}

type ledgerDevice struct {
	raw ledger_go.LedgerDevice
}

func (ld *ledgerDevice) Close() error {
	return ld.raw.Close()
}

// GetVersion returns the current version of the Oasis user app.
func (ld *ledgerDevice) GetVersion() (*VersionInfo, error) {
	message := []byte{claConsumer, insGetVersion, 0, 0, 0}
	response, err := ld.raw.Exchange(message)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed GetVersion request: %w", err)
	}

	if len(response) < 4 {
		return nil, fmt.Errorf("ledger: truncated GetVersion response")
	}

	return &VersionInfo{
		Major: response[1],
		Minor: response[2],
		Patch: response[3],
	}, nil
}

// GetPublicKey25519 returns the Ed25519 or Sr25519 public key associated with the given derivation path.
// If the requireConfirmation flag is set, this will require confirmation from the user.
func (ld *ledgerDevice) GetPublicKey25519(path []uint32, algorithm string, requireConfirmation bool) ([]byte, error) {
	pathBytes, err := getSerializedPath(path)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed to get serialized path bytes: %w", err)
	}

	var ins byte
	switch algorithm {
	case wallet.AlgorithmEd25519Adr8:
		ins = insGetAddrEd25519
	case wallet.AlgorithmSr25519Adr8:
		ins = insGetAddrSr25519
	default:
		return nil, fmt.Errorf("ledger: unknown provided algorithm %s", algorithm)
	}

	response, err := ld.getPublicKeyRaw(pathBytes, ins, requireConfirmation)
	if err != nil {
		return nil, err
	}
	// 32-byte public key + Bech32-encoded address.
	if len(response) < 78 {
		return nil, fmt.Errorf("ledger: truncated GetAddr*25519 response")
	}

	rawPubkey := response[0:32]
	rawAddr := string(response[32:])

	// Sanity check, if the public key matches the expected address shown on Ledger.
	var addrSpec types.SignatureAddressSpec
	switch algorithm {
	case wallet.AlgorithmEd25519Adr8:
		addrSpec.Ed25519 = &ed25519.PublicKey{}
		if err = addrSpec.Ed25519.UnmarshalBinary(rawPubkey); err != nil {
			return nil, fmt.Errorf("ledger: device returned malformed public key: %w", err)
		}
	case wallet.AlgorithmSr25519Adr8:
		addrSpec.Sr25519 = &sr25519.PublicKey{}
		if err = addrSpec.Sr25519.UnmarshalBinary(rawPubkey); err != nil {
			return nil, fmt.Errorf("ledger: device returned malformed public key: %w", err)
		}
	default:
		return nil, fmt.Errorf("ledger: unknown provided algorithm %s", algorithm)
	}

	var addrFromDevice types.Address
	if err = addrFromDevice.UnmarshalText([]byte(rawAddr)); err != nil {
		return nil, fmt.Errorf("ledger: device returned malformed account address: %w", err)
	}
	addrFromPubkey := types.NewAddress(addrSpec)
	if !addrFromDevice.Equal(addrFromPubkey) {
		return nil, fmt.Errorf(
			"ledger: account address computed on device (%s) doesn't match internally computed account address (%s)",
			addrFromDevice,
			addrFromPubkey,
		)
	}

	return rawPubkey, nil
}

// GetPublicKeySecp256k1 returns the Secp256k1 public key associated with the given derivation path.
// If the requireConfirmation flag is set, this will require confirmation from the user.
func (ld *ledgerDevice) GetPublicKeySecp256k1(path []uint32, requireConfirmation bool) ([]byte, error) {
	pathBytes, err := getSerializedBip44Path(path)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed to get serialized BIP44 path bytes: %w", err)
	}

	response, err := ld.getPublicKeyRaw(pathBytes, insGetAddrSecp256k1, requireConfirmation)
	if err != nil {
		return nil, err
	}
	// 33-byte public key + 20-byte address
	if len(response) < 53 {
		return nil, fmt.Errorf("ledger: truncated GetAddrSecp256k1 response")
	}

	rawPubkey := response[0:33]
	rawAddr := string(response[33:])

	pubkey, err := btcec.ParsePubKey(rawPubkey)
	if err != nil {
		return nil, fmt.Errorf("ledger: device returned malformed public key: %w", err)
	}

	addrFromDevice := ethCommon.HexToAddress(rawAddr)

	h := sha3.NewLegacyKeccak256()
	h.Write(pubkey.SerializeUncompressed()[1:])
	hash := h.Sum(nil)
	addrFromPubkey := ethCommon.BytesToAddress(hash[32-20:])
	if addrFromDevice.String() != addrFromPubkey.String() {
		return nil, fmt.Errorf(
			"ledger: account address computed on device (%s) doesn't match internally computed account address (%s)",
			addrFromDevice,
			addrFromPubkey,
		)
	}

	return rawPubkey, nil
}

func (ld *ledgerDevice) getPublicKeyRaw(pathBytes []byte, ins byte, requireConfirmation bool) ([]byte, error) {
	p1 := byte(0)
	if requireConfirmation {
		p1 = byte(1)
	}

	// Prepare message
	header := []byte{claConsumer, ins, p1, 0, 0}
	message := append([]byte{}, header...)
	message = append(message, pathBytes...)
	message[4] = byte(len(message) - len(header)) // update length

	response, err := ld.raw.Exchange(message)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed to request public key: %w", err)
	}
	return response, nil
}

// SignEd25519 asks the device to sign the given domain-separated message with the key derived from
// the given derivation path.
func (ld *ledgerDevice) SignEd25519(path []uint32, context, message []byte) ([]byte, error) {
	pathBytes, err := getSerializedPath(path)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed to get serialized path bytes: %w", err)
	}

	chunks, err := prepareConsensusChunks(pathBytes, context, message, userMessageChunkSize)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed to prepare chunks: %w", err)
	}

	var finalResponse []byte
	for idx, chunk := range chunks {
		payloadLen := byte(len(chunk))

		var payloadDesc byte
		switch idx {
		case 0:
			payloadDesc = payloadChunkInit
		case len(chunks) - 1:
			payloadDesc = payloadChunkLast
		default:
			payloadDesc = payloadChunkAdd
		}

		message := []byte{claConsumer, insSignEd25519, payloadDesc, 0, payloadLen}
		message = append(message, chunk...)

		response, err := ld.raw.Exchange(message)
		if err != nil {
			switch err.Error() {
			case errMsgInvalidParameters, errMsgInvalidated:
				return nil, fmt.Errorf("ledger: failed to sign: %s", string(response))
			case errMsgRejected:
				return nil, fmt.Errorf("ledger: signing request rejected by user")
			}
			return nil, fmt.Errorf("ledger: failed to sign: %w", err)
		}

		finalResponse = response
	}

	// XXX: Work-around for Oasis App issue of currently not being capable of
	// signing two transactions immediately one after another:
	// https://github.com/Zondax/ledger-oasis/issues/68.
	time.Sleep(100 * time.Millisecond)

	return finalResponse, nil
}

// SignRtEd25519 asks the device to sign the given message and metadata with the Ed25519 key derived from
// the given hardened path.
func (ld *ledgerDevice) SignRtEd25519(path []uint32, sigCtx signature.Context, message []byte) ([]byte, error) {
	return ld.signRt25519(path, sigCtx, message, insSignRtEd25519)
}

// SignRtSr25519 asks the device to sign the given message and metadata with the Sr25519 key derived from
// the given hardened path.
func (ld *ledgerDevice) SignRtSr25519(path []uint32, sigCtx signature.Context, message []byte) ([]byte, error) {
	return ld.signRt25519(path, sigCtx, message, insSignRtSr25519)
}

// SignRtSecp256k1 asks the device to sign the given message and metadata with the Secp256k1 key derived from
// the given BIP44 path.
func (ld *ledgerDevice) SignRtSecp256k1(bip44Path []uint32, sigCtx signature.Context, message []byte) ([]byte, error) {
	pathBytes, err := getSerializedBip44Path(bip44Path)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed to get serialized BIP44 path bytes: %w", err)
	}
	rsvSig, err := ld.signRt(pathBytes, sigCtx, message, insSignRtSecp256k1)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed to perform secp256k1 signature: %w", err)
	}
	if len(rsvSig) < 64 {
		return nil, fmt.Errorf("ledger: secp256k1 signature in RS format should be at least 64-bytes long")
	}
	var r, s btcec.ModNScalar
	r.SetByteSlice(rsvSig[0:32])
	s.SetByteSlice(rsvSig[32:64])
	return ecdsa.NewSignature(&r, &s).Serialize(), nil
}

func (ld *ledgerDevice) signRt25519(path []uint32, sigCtx signature.Context, message []byte, ins byte) ([]byte, error) {
	pathBytes, err := getSerializedPath(path)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed to get serialized path bytes: %w", err)
	}
	return ld.signRt(pathBytes, sigCtx, message, ins)
}

func (ld *ledgerDevice) signRt(pathBytes []byte, sigCtx signature.Context, message []byte, instruction byte) ([]byte, error) {
	richSigCtx, ok := sigCtx.(*signature.RichContext)
	if !ok {
		return nil, fmt.Errorf("ledger: signature context is not RichContext")
	}

	meta := signature.NewHwContext(richSigCtx)
	metadataBytes := cbor.Marshal(meta)
	chunks, err := prepareRuntimeChunks(pathBytes, metadataBytes, message, userMessageChunkSize)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed to prepare chunks: %w", err)
	}

	var finalResponse []byte
	for idx, chunk := range chunks {
		payloadLen := byte(len(chunk))

		var payloadDesc byte
		switch idx {
		case 0:
			payloadDesc = payloadChunkInit
		case len(chunks) - 1:
			payloadDesc = payloadChunkLast
		default:
			payloadDesc = payloadChunkAdd
		}

		message := []byte{claConsumer, instruction, payloadDesc, 0, payloadLen}
		message = append(message, chunk...)

		response, err := ld.raw.Exchange(message)
		if err != nil {
			switch err.Error() {
			case errMsgInvalidParameters, errMsgInvalidated:
				return nil, fmt.Errorf("ledger: failed to sign: %s", string(response))
			case errMsgRejected:
				return nil, fmt.Errorf("ledger: signing request rejected by user")
			}
			return nil, fmt.Errorf("ledger: failed to sign: %w", err)
		}

		finalResponse = response
	}

	// XXX: Work-around for Oasis App issue of currently not being capable of
	// signing two transactions immediately one after another:
	// https://github.com/Zondax/ledger-oasis/issues/68.
	time.Sleep(100 * time.Millisecond)

	return finalResponse, nil
}

// connectToDevice connects to the first connected Ledger device.
func connectToDevice() (*ledgerDevice, error) {
	ledgerAdmin := ledger_go.NewLedgerAdmin()

	// TODO: Support multiple devices.
	numDevices := ledgerAdmin.CountDevices()
	switch {
	case numDevices == 0:
		return nil, fmt.Errorf("ledger: no devices connected")
	case numDevices > 1:
		return nil, fmt.Errorf("ledger: multiple devices not supported")
	default:
	}

	raw, err := ledgerAdmin.Connect(0)
	if err != nil {
		return nil, fmt.Errorf("ledger: failed to connect to device: %w", err)
	}

	return &ledgerDevice{raw}, nil
}
