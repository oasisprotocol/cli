package scheduler

import (
	"encoding/base64"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/tuplehash"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/roflmarket"
)

const (
	/// Domain verification token context.
	DomainVerificationTokenContext = "rofl-scheduler/proxy: domain verification token" //nolint:gosec
)

// DomainVerificationToken derives the verification token for a given domain.
//
// The token is derived using a cryptographic hash function and is used to verify
// that the domain is owned by the instance. More specifically, the token is derived
// as follows:
//
//	TupleHash[DOMAIN_VERIFICATION_TOKEN_CONTEXT](
//	  deployment.app_id,
//	  instance.provider,
//	  instance.id,
//	  domain
//	)
//
// The result is then encoded using base64.
func DomainVerificationToken(instance *roflmarket.Instance, appID rofl.AppID, domain string) string {
	h := tuplehash.New256(32, []byte(DomainVerificationTokenContext))
	_, _ = h.Write(appID[:])
	_, _ = h.Write(instance.Provider[:])
	_, _ = h.Write(instance.ID[:])
	_, _ = h.Write([]byte(domain))
	output := h.Sum(nil)

	return base64.StdEncoding.EncodeToString(output)
}
