package rofl

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"github.com/oasisprotocol/curve25519-voi/primitives/x25519"
	"github.com/oasisprotocol/deoxysii"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	mraeDeoxysii "github.com/oasisprotocol/oasis-core/go/common/crypto/mrae/deoxysii"
)

// SecretConfig is the configuration of a given secret.
type SecretConfig struct {
	// Name is the name of the secret.
	Name string `yaml:"name" json:"name"`
	// PublicName is the public name of the secret. It will be visible to everyone on-chain, but is
	// otherwise ignored.
	PublicName string `yaml:"public_name,omitempty" json:"public_name,omitempty"`
	// Value is the Base64-encoded encrypted value.
	Value string `yaml:"value" json:"value"`
}

// Validate validates the secret configuration for correctness.
func (s *SecretConfig) Validate() error {
	if s == nil {
		return fmt.Errorf("secret cannot be nil")
	}
	if s.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if len(s.Value) == 0 {
		return fmt.Errorf("value cannot be empty")
	}
	if _, err := base64.StdEncoding.DecodeString(s.Value); err != nil {
		return fmt.Errorf("malformed value: %w", err)
	}
	return nil
}

// SecretEnvelope is the envelope used for storing encrypted secrets.
type SecretEnvelope struct {
	// Pk is the ephemeral public key used for X25519.
	Pk x25519.PublicKey `json:"pk"`
	// Nonce.
	Nonce [deoxysii.NonceSize]byte `json:"nonce"`
	// Name is the encrypted secret name.
	Name []byte `json:"name"`
	// Value is the encrypted secret value.
	Value []byte `json:"value"`
}

// EncryptSecret encrypts the given secret given its plain-text name and value together with the
// secrets encryption key (SEK) obtained for the given application. Returns the Base64-encoded
// value that can be used in the configuration.
func EncryptSecret(name string, value []byte, sek x25519.PublicKey) (string, error) {
	// Generate ephemeral X25519 key pair.
	pk, sk, err := x25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to generate ephemeral X25519 key pair: %w", err)
	}

	// Generate random nonce.
	var nonce [deoxysii.NonceSize]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", fmt.Errorf("failed to generate random nonce: %w", err)
	}

	// Seal plain-text secret name and value.
	encName := mraeDeoxysii.Box.Seal(nil, nonce[:], []byte(name), []byte("name"), &sek, sk)
	encValue := mraeDeoxysii.Box.Seal(nil, nonce[:], value, []byte("value"), &sek, sk)

	envelope := SecretEnvelope{
		Pk:    *pk,
		Nonce: nonce,
		Name:  encName,
		Value: encValue,
	}
	data := cbor.Marshal(envelope)
	return base64.StdEncoding.EncodeToString(data), nil
}

// PrepareSecrets transforms the secrets configuration into a format suitable for updating the ROFL
// app configuration.
//
// Panics in case the configuration is malformed.
func PrepareSecrets(cfg []*SecretConfig) map[string][]byte {
	if len(cfg) == 0 {
		return nil
	}

	out := make(map[string][]byte)
	for _, sc := range cfg {
		name := sc.Name
		if sc.PublicName != "" {
			name = sc.PublicName
		}

		data, err := base64.StdEncoding.DecodeString(sc.Value)
		if err != nil {
			panic(err) // Should not happen as the configuration has been validated.
		}
		out[name] = data
	}
	return out
}
