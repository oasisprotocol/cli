package scheduler

import (
	"encoding/base64"
	"time"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
)

// StdAuthContextBase is the base authentication context.
const StdAuthContextBase = "rofl-scheduler/auth: v1"

// AuthLoginRequest is the request to login.
type AuthLoginRequest struct {
	Method string `json:"method"`
	Data   any    `json:"data"`
}

// AuthLoginResponse is the response from the login request.
type AuthLoginResponse struct {
	Token  string `json:"token"`
	Expiry uint64 `json:"expiry"`
}

// StdAuthLoginRequest is the body for the standard authentication method.
type StdAuthLoginRequest struct {
	Body      string `json:"body"`
	Signature string `json:"signature"`
}

// StdAuthBody is the standard authentication body.
type StdAuthBody struct {
	Version   uint16          `json:"v"`
	Domain    string          `json:"domain"`
	Provider  types.Address   `json:"provider"`
	Signer    types.PublicKey `json:"signer"`
	Nonce     string          `json:"nonce"`
	NotBefore uint64          `json:"not_before"`
	NotAfter  uint64          `json:"not_after"`
}

// SignLogin creates a new login request for the given provider and signs it.
func SignLogin(sigCtx signature.Context, signer signature.Signer, domain string, provider types.Address) (*AuthLoginRequest, error) {
	notBefore := time.Now().Add(-10 * time.Second) // Allow for some time drift.
	notAfter := time.Now().Add(30 * time.Second)   // Short expiry, just needed for login.

	body := StdAuthBody{
		Version:   1,
		Domain:    domain,
		Provider:  provider,
		Signer:    types.PublicKey{PublicKey: signer.Public()},
		Nonce:     "",
		NotBefore: uint64(notBefore.Unix()), //nolint: gosec
		NotAfter:  uint64(notAfter.Unix()),  //nolint: gosec
	}
	rawBody := cbor.Marshal(body)
	sig, err := signer.ContextSign(sigCtx, rawBody)
	if err != nil {
		return nil, err
	}

	rq := &AuthLoginRequest{
		Method: "std",
		Data: StdAuthLoginRequest{
			Body:      base64.StdEncoding.EncodeToString(rawBody),
			Signature: base64.StdEncoding.EncodeToString(sig),
		},
	}
	return rq, nil
}
