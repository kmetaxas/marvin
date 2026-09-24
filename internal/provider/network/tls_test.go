package network

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestCert(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey, isClient bool, san ...string) (certPEM, keyPEM string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	if isClient {
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		template.Subject.CommonName = "client"
	}
	if len(san) > 0 {
		template.DNSNames = san
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, ca, &priv.PublicKey, caKey)
	require.NoError(t, err)

	certPEMB := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	keyPEMB := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return string(certPEMB), string(keyPEMB)
}

func generateTestCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey, string) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	certPEMB := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	return cert, priv, string(certPEMB)
}

func startTestTLSServer(t *testing.T, certPEM, keyPEM string) (string, func()) {
	t.Helper()
	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	require.NoError(t, err)

	config := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}

	ln, err := tls.Listen("tcp4", "127.0.0.1:0", config)
	require.NoError(t, err)

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				// Just perform handshake and close
				if tc, ok := c.(*tls.Conn); ok {
					_ = tc.Handshake()
				}
			}(conn)
		}
	}()

	return ln.Addr().String(), func() { ln.Close() }
}

func TestTLSHandshakeSuccess(t *testing.T) {
	t.Parallel()

	caCert, caKey, caCertPEM := generateTestCA(t)
	serverCertPEM, serverKeyPEM := generateTestCert(t, caCert, caKey, false, "localhost")

	addr, cleanup := startTestTLSServer(t, serverCertPEM, serverKeyPEM)
	defer cleanup()

	tr := &tlsTask{}
	res, err := tr.Execute(context.Background(), map[string]any{
		"target":  addr,
		"ca_cert": caCertPEM,
	})
	require.NoError(t, err)
	require.True(t, res.Success, "unexpected error: %s", res.Error)

	data, ok := res.Data.(map[string]any)
	require.True(t, ok)
	assert.True(t, data["handshake_ok"].(bool))
	assert.True(t, data["validation_ok"].(bool))
	assert.NotEmpty(t, data["certificates"])
}

func TestTLSHandshakeInsecure(t *testing.T) {
	t.Parallel()

	caCert, caKey, _ := generateTestCA(t)
	serverCertPEM, serverKeyPEM := generateTestCert(t, caCert, caKey, false)

	addr, cleanup := startTestTLSServer(t, serverCertPEM, serverKeyPEM)
	defer cleanup()

	tr := &tlsTask{}
	res, err := tr.Execute(context.Background(), map[string]any{
		"target":               addr,
		"insecure_skip_verify": true,
	})
	require.NoError(t, err)
	assert.True(t, res.Success)

	data, ok := res.Data.(map[string]any)
	require.True(t, ok)
	assert.True(t, data["handshake_ok"].(bool))
	assert.True(t, data["validation_ok"].(bool)) // skipped, so we report true
}

func TestTLSMissingTarget(t *testing.T) {
	t.Parallel()

	tr := &tlsTask{}
	res, err := tr.Execute(context.Background(), map[string]any{})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "target")
}

func TestTLSValidationFails(t *testing.T) {
	t.Parallel()

	// Generate two different CAs
	_, _, wrongCACertPEM := generateTestCA(t)
	caCert, caKey, _ := generateTestCA(t)
	serverCertPEM, serverKeyPEM := generateTestCert(t, caCert, caKey, false)

	addr, cleanup := startTestTLSServer(t, serverCertPEM, serverKeyPEM)
	defer cleanup()

	tr := &tlsTask{}
	res, err := tr.Execute(context.Background(), map[string]any{
		"target":  addr,
		"ca_cert": wrongCACertPEM,
	})
	require.NoError(t, err)
	assert.True(t, res.Success) // Handshake succeeded

	data, ok := res.Data.(map[string]any)
	require.True(t, ok)
	assert.True(t, data["handshake_ok"].(bool))
	assert.False(t, data["validation_ok"].(bool))
	assert.NotEmpty(t, data["validation_error"])
}

func TestTLSmTLS(t *testing.T) {
	t.Parallel()

	caCert, caKey, caCertPEM := generateTestCA(t)
	serverCertPEM, serverKeyPEM := generateTestCert(t, caCert, caKey, false, "localhost")
	clientCertPEM, clientKeyPEM := generateTestCert(t, caCert, caKey, true)

	// Server that requests client cert
	serverCert, err := tls.X509KeyPair([]byte(serverCertPEM), []byte(serverKeyPEM))
	require.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(caCertPEM))
	config := &tls.Config{
		Certificates:       []tls.Certificate{serverCert},
		ClientAuth:         tls.RequireAndVerifyClientCert,
		ClientCAs:          pool,
		InsecureSkipVerify: true,
	}

	ln, err := tls.Listen("tcp4", "127.0.0.1:0", config)
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				if tc, ok := c.(*tls.Conn); ok {
					_ = tc.Handshake()
				}
			}(conn)
		}
	}()

	tr := &tlsTask{}
	res, err := tr.Execute(context.Background(), map[string]any{
		"target":      ln.Addr().String(),
		"ca_cert":     caCertPEM,
		"client_cert": clientCertPEM,
		"client_key":  clientKeyPEM,
	})
	require.NoError(t, err)
	assert.True(t, res.Success, "unexpected error: %s", res.Error)

	data, ok := res.Data.(map[string]any)
	require.True(t, ok)
	assert.True(t, data["handshake_ok"].(bool))
	assert.True(t, data["validation_ok"].(bool))
}
