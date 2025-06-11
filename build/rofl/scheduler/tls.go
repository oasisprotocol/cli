package scheduler

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
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
			VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
				if len(rawCerts) == 0 {
					return fmt.Errorf("server did not send a certificate")
				}

				cert, err := x509.ParseCertificate(rawCerts[0])
				if err != nil {
					return fmt.Errorf("bad X509 certificate: %w", err)
				}

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
