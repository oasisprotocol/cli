package file

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"fmt"

	"github.com/tyler-smith/go-bip39"

	"github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	"github.com/oasisprotocol/curve25519-voi/primitives/sr25519"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/sakg"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature/signers/memory"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/slip10"
	sdkSignature "github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	sdkSr25519 "github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/sr25519"
)

// Sr25519FromMnemonic derives a signer using ADR-8 from given mnemonic.
func Sr25519FromMnemonic(mnemonic string, number uint32) (sdkSignature.Signer, []byte, error) {
	if number > sakg.MaxAccountKeyNumber {
		return nil, nil, fmt.Errorf(
			"sakg: invalid key number: %d (maximum: %d)",
			number,
			sakg.MaxAccountKeyNumber,
		)
	}

	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, nil, fmt.Errorf("sakg: invalid mnemonic")
	}

	seed := bip39.NewSeed(mnemonic, "")

	_, chainCode, skBinary, skCanBinary, err := newMasterKey(seed)
	if err != nil {
		return nil, nil, fmt.Errorf("sakg: error deriving master key: %w", err)
	}

	pathStr := fmt.Sprintf("%s/%d'", sakg.BIP32PathPrefix, number)
	path, err := sakg.NewBIP32Path(pathStr)
	if err != nil {
		return nil, nil, fmt.Errorf("sakg: error creating BIP-0032 path %s: %w", pathStr, err)
	}

	var signer sdkSignature.Signer
	for _, index := range path {
		signer, chainCode, skBinary, skCanBinary, err = newChildKey(skBinary, chainCode, index)
		if err != nil {
			return nil, nil, fmt.Errorf("sakg: error deriving child key: %w", err)
		}
	}

	return signer, skCanBinary, nil
}

func newMasterKey(seed []byte) (sdkSignature.Signer, slip10.ChainCode, []byte, []byte, error) {
	// Let S be a seed byte sequence of 128 to 512 bits in length.
	if sLen := len(seed); sLen < slip10.SeedMinSize || sLen > slip10.SeedMaxSize {
		return nil, slip10.ChainCode{}, nil, nil, fmt.Errorf("slip10: invalid seed")
	}

	// 1. Calculate I = HMAC-SHA512(Key = Curve, Data = S)
	mac := hmac.New(sha512.New, []byte("ed25519 seed"))
	_, _ = mac.Write(seed)
	I := mac.Sum(nil)

	// 2. Split I into two 32-byte sequences, IL and IR.
	// 3. Use parse256(IL) as master secret key, and IR as master chain code.
	return splitDigest(I)
}

func newChildKey(kPar []byte, cPar slip10.ChainCode, index uint32) (sdkSignature.Signer, slip10.ChainCode, []byte, []byte, error) {
	if len(kPar) < memory.SeedSize {
		return nil, slip10.ChainCode{}, nil, nil, fmt.Errorf("slip10: invalid parent key")
	}

	// 1. Check whether i >= 2^31 (whether the child is a hardened key).
	if index < 1<<31 {
		// If not (normal child):
		// If curve is ed25519: return failure.
		return nil, slip10.ChainCode{}, nil, nil, fmt.Errorf("slip10: non-hardened keys not supported")
	}

	// If so (hardened child):
	// let I = HMAC-SHA512(Key = cpar, Data = 0x00 || ser256(kpar) || ser32(i)).
	// (Note: The 0x00 pads the private key to make it 33 bytes long.)
	var b [4]byte
	mac := hmac.New(sha512.New, cPar[:])
	_, _ = mac.Write(b[0:1])                 // 0x00
	_, _ = mac.Write(kPar[:memory.SeedSize]) // ser256(kPar)
	binary.BigEndian.PutUint32(b[:], index)  // Note: The spec neglects to define ser32.
	_, _ = mac.Write(b[:])                   // ser32(i)
	I := mac.Sum(nil)

	// 2. Split I into two 32-byte sequences, IL and IR.
	// 3. The returned chain code ci is IR.
	// 4. If curve is ed25519: The returned child key ki is parse256(IL).
	return splitDigest(I)
}

func splitDigest(digest []byte) (sdkSignature.Signer, slip10.ChainCode, []byte, []byte, error) {
	IL, IR := digest[:32], digest[32:]

	var chainCode slip10.ChainCode

	edSk := ed25519.NewKeyFromSeed(IL) // Needed for the SLIP10 scheme.
	msk, err := sr25519.NewMiniSecretKeyFromBytes(IL)
	if err != nil {
		return nil, chainCode, nil, nil, err
	}
	sk := msk.ExpandUniform()

	signer := sdkSr25519.NewSignerFromKeyPair(sk.KeyPair())
	copy(chainCode[:], IR)

	skCanBinary, err := sk.MarshalBinary()
	if err != nil {
		return nil, chainCode, nil, nil, err
	}

	return signer, chainCode, edSk[:], skCanBinary, nil
}
