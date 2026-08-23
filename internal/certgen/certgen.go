// Package certgen generates development TLS certificates: a local CA plus a
// server certificate signed by it. It backs the server's `gencert` subcommand
// and is reused by tests to mint ephemeral in-memory certs (no files on disk).
//
// These are for development and internal/private deployments. Production
// deployments should use certificates from your organization's CA / PKI.
package certgen

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// Bundle is a generated CA and a server cert/key signed by it, all PEM-encoded.
type Bundle struct {
	CACertPEM     []byte
	ServerCertPEM []byte
	ServerKeyPEM  []byte
}

// Generate produces a CA and a server certificate valid for the given hosts
// (DNS names and/or IP addresses). validFor sets the certificate lifetime.
func Generate(hosts []string, validFor time.Duration) (*Bundle, error) {
	if len(hosts) == 0 {
		hosts = []string{"localhost", "127.0.0.1"}
	}
	notBefore := time.Now().Add(-time.Hour)
	notAfter := time.Now().Add(validFor)

	// --- CA ---
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	caSerial, err := randSerial()
	if err != nil {
		return nil, err
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          caSerial,
		Subject:               pkix.Name{Organization: []string{"KoraDB Dev CA"}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, err
	}

	// --- server cert signed by CA ---
	srvKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	srvSerial, err := randSerial()
	if err != nil {
		return nil, err
	}
	srvTmpl := &x509.Certificate{
		SerialNumber: srvSerial,
		Subject:      pkix.Name{Organization: []string{"KoraDB Server"}},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			srvTmpl.IPAddresses = append(srvTmpl.IPAddresses, ip)
		} else {
			srvTmpl.DNSNames = append(srvTmpl.DNSNames, h)
		}
	}
	srvDER, err := x509.CreateCertificate(rand.Reader, srvTmpl, caCert, &srvKey.PublicKey, caKey)
	if err != nil {
		return nil, err
	}
	srvKeyDER, err := x509.MarshalPKCS8PrivateKey(srvKey)
	if err != nil {
		return nil, err
	}

	return &Bundle{
		CACertPEM:     pemBlock("CERTIFICATE", caDER),
		ServerCertPEM: pemBlock("CERTIFICATE", srvDER),
		ServerKeyPEM:  pemBlock("PRIVATE KEY", srvKeyDER),
	}, nil
}

func pemBlock(typ string, der []byte) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der})
}

func randSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("certgen: serial: %w", err)
	}
	return n, nil
}
