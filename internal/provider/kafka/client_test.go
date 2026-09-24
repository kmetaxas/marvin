package kafka

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jcmturner/gokrb5/v8/iana/etypeID"
	"github.com/jcmturner/gokrb5/v8/keytab"
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTLSConfigInlineCAData(t *testing.T) {
	t.Parallel()

	caPEM := testCertificatePEM(t)
	tlsConfig, err := buildTLSConfig(config.KafkaTLSConfig{
		Enabled: true,
		CAData:  base64.StdEncoding.EncodeToString(caPEM),
	})

	require.NoError(t, err)
	require.NotNil(t, tlsConfig.RootCAs)
	assert.Equal(t, uint16(0x0303), tlsConfig.MinVersion)
}

func TestBuildSASLMechanismGSSAPI(t *testing.T) {
	t.Parallel()

	keytabData := testKeytabData(t, "alice", "EXAMPLE.COM")
	krb5ConfData := []byte(`[libdefaults]
	default_realm = EXAMPLE.COM
	dns_lookup_kdc = false
	dns_lookup_realm = false

[realms]
	EXAMPLE.COM = {
		kdc = localhost:88
	}
`)

	var tempPaths []string
	mechanism, err := buildSASLMechanismWithTempFiles(config.KafkaSASLConfig{
		Mechanism:           "GSSAPI",
		Username:            "alice",
		KerberosRealm:       "EXAMPLE.COM",
		KerberosServiceName: "kafka",
		KeytabData:          base64.StdEncoding.EncodeToString(keytabData),
		Krb5ConfData:        base64.StdEncoding.EncodeToString(krb5ConfData),
	}, &tempPaths)
	t.Cleanup(func() {
		for _, path := range tempPaths {
			_ = os.RemoveAll(path)
		}
	})

	require.NoError(t, err)
	assert.NotNil(t, mechanism)
	assert.Len(t, tempPaths, 2)
}

func TestClientCleanupTempFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tempDir := filepath.Join(dir, "marvin-krb5-test")
	require.NoError(t, os.Mkdir(tempDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "client.keytab"), []byte("secret"), 0o600))

	client := &clientImpl{tempPaths: []string{tempDir}}
	client.cleanupTempFiles()

	_, err := os.Stat(tempDir)
	require.ErrorIs(t, err, os.ErrNotExist)
	assert.Empty(t, client.tempPaths)
}

func testCertificatePEM(t *testing.T) []byte {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "marvin-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
}

func testKeytabData(t *testing.T, principal, realm string) []byte {
	t.Helper()

	kt := keytab.New()
	require.NoError(t, kt.AddEntry(principal, realm, "password", time.Now(), 1, etypeID.AES256_CTS_HMAC_SHA1_96))
	data, err := kt.Marshal()
	require.NoError(t, err)
	return data
}
