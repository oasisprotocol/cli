package scheduler

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
)

// NewHTTPClient creates an HTTP client to communicate with the given scheduler.
func NewHTTPClient(dsc *rofl.Registration) (*http.Client, error) {
	schedulerTLSPk, ok := dsc.Metadata[MetadataKeyTLSPk]
	if !ok {
		return nil, fmt.Errorf("scheduler does not publish its TLS public key")
	}
	expectedSubjectPublicKeyInfo, err := base64.StdEncoding.DecodeString(schedulerTLSPk)
	if err != nil {
		return nil, fmt.Errorf("malformed scheduler TLS public key: %w", err)
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			VerifyConnection: func(cs tls.ConnectionState) error {
				// Ensure certificate verification runs for every connection,
				// including resumed TLS sessions, which bypass function
				// VerifyPeerCertificate.
				if len(cs.PeerCertificates) == 0 {
					return fmt.Errorf("server did not send a certificate")
				}

				cert := cs.PeerCertificates[0]

				if !bytes.Equal(cert.RawSubjectPublicKeyInfo, expectedSubjectPublicKeyInfo) {
					return fmt.Errorf("server certificate public key does not match expected value")
				}
				return nil
			},
		},
	}
	client := &http.Client{
		Transport: transport,
	}
	return client, nil
}
