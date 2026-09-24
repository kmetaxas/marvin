package testutil

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"software.sslmate.com/src/go-pkcs12"
)

// clusterID is a fixed KRaft cluster id used across all Kafka containers. It
// is arbitrary but must be a valid base64-encoded 16-byte value.
const clusterID = "MkU3OEVBNTcwNTJENDM2Qk"

// keystorePassword is the password used for all generated PKCS12 keystores and
// truststores. It matches the value used by the Confluent demo images.
const keystorePassword = "confluent"

// freePort binds an ephemeral TCP port on the loopback interface, records the
// assigned port number, and immediately releases it. The returned port is used
// to bind a container port to a fixed host port so that Kafka's advertised
// listener (which must be reachable from the test process) matches the actual
// host port the container is published on.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "failed to allocate a free port")
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// fixedPortBinding returns a HostConfigModifier that publishes the given
// container port on a fixed host port. This is required for Kafka because the
// broker advertises a listener address that the client must be able to reach;
// a random ephemeral mapping would break that contract.
func fixedPortBinding(containerPort string, hostPort int) func(*container.HostConfig) {
	return func(hc *container.HostConfig) {
		hc.PortBindings = network.PortMap{
			network.MustParsePort(containerPort): {
				{HostPort: strconv.Itoa(hostPort)},
			},
		}
	}
}

// terminateFn returns a function that terminates a single container. It is
// used to build the combined cleanup function returned by each helper.
func terminateFn(c testcontainers.Container) func() {
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = c.Terminate(ctx)
	}
}

// newCleanup builds an idempotent cleanup function that runs all the provided
// terminate functions exactly once, registers it with t.Cleanup, and returns
// it so callers can also invoke it explicitly.
func newCleanup(t *testing.T, fns ...func()) func() {
	t.Helper()
	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			for _, fn := range fns {
				fn()
			}
		})
	}
	t.Cleanup(cleanup)
	return cleanup
}

// startContainer starts a single container, registers its termination via
// t.Cleanup, and returns the running container.
func startContainer(t *testing.T, req testcontainers.ContainerRequest) testcontainers.Container {
	t.Helper()
	ctx := context.Background()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "failed to start container %s", req.Image)
	t.Cleanup(terminateFn(c))
	return c
}

// createNetwork creates a named Docker network, registers its removal via
// t.Cleanup, and returns the network name. Containers that need to talk to
// each other (Kafka + Schema Registry, Kafka + KDC) are attached to it.
func createNetwork(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	name := fmt.Sprintf("marvin-kafka-test-%d", time.Now().UnixNano())
	netw, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name:           name,
			CheckDuplicate: false,
		},
	})
	require.NoError(t, err, "failed to create network")
	t.Cleanup(func() {
		_ = netw.Remove(ctx)
	})
	return name
}

// StartKafkaPlaintext starts a single-node Kafka broker in KRaft mode with a
// PLAINTEXT listener. It returns the externally reachable bootstrap servers
// (a single "localhost:<port>" address) and a cleanup function.
//
// The broker advertises "localhost:<hostPort>" where hostPort is a fixed,
// pre-allocated host port, so the returned bootstrap address is directly
// usable by a Kafka client running in the test process.
func StartKafkaPlaintext(t *testing.T) (bootstrapServers []string, cleanup func()) {
	t.Helper()

	hostPort := freePort(t)

	req := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-kafka:latest",
		ExposedPorts: []string{"9092/tcp"},
		Env: map[string]string{
			"KAFKA_NODE_ID":                          "1",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":   "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT",
			"KAFKA_ADVERTISED_LISTENERS":             fmt.Sprintf("PLAINTEXT://localhost:%d", hostPort),
			"KAFKA_PROCESS_ROLES":                    "broker,controller",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":         "1@localhost:29093",
			"KAFKA_LISTENERS":                        "PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:29093",
			"KAFKA_INTER_BROKER_LISTENER_NAME":       "PLAINTEXT",
			"KAFKA_CONTROLLER_LISTENER_NAMES":        "CONTROLLER",
			"KAFKA_LOG_DIRS":                         "/tmp/kraft-combined-logs",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
			"CLUSTER_ID":                             clusterID,
		},
		HostConfigModifier: fixedPortBinding("9092/tcp", hostPort),
		WaitingFor:         wait.ForLog("Kafka Server started").WithStartupTimeout(90 * time.Second),
	}

	c := startContainer(t, req)

	return []string{fmt.Sprintf("localhost:%d", hostPort)}, terminateFn(c)
}

// StartKafkaWithSCRAM starts a single-node Kafka broker in KRaft mode with a
// SASL_PLAINTEXT listener secured by SCRAM-SHA-256. A single user
// (admin / admin-secret) is provisioned during storage formatting.
//
// It returns the bootstrap servers, a cleanup function, and the SCRAM
// credentials that were provisioned.
func StartKafkaWithSCRAM(t *testing.T) (bootstrapServers []string, cleanup func(), scramConfig ScramConfig) {
	t.Helper()

	hostPort := freePort(t)

	req := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-kafka:latest",
		ExposedPorts: []string{"9092/tcp"},
		Env: map[string]string{
			"KAFKA_NODE_ID":                          "1",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":   "CONTROLLER:PLAINTEXT,OUTSIDE:SASL_PLAINTEXT,INTERNAL:PLAINTEXT",
			"KAFKA_ADVERTISED_LISTENERS":             fmt.Sprintf("OUTSIDE://localhost:%d,INTERNAL://localhost:9093", hostPort),
			"KAFKA_PROCESS_ROLES":                    "broker,controller",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":         "1@localhost:29093",
			"KAFKA_LISTENERS":                        "OUTSIDE://0.0.0.0:9092,CONTROLLER://0.0.0.0:29093,INTERNAL://0.0.0.0:9093",
			"KAFKA_INTER_BROKER_LISTENER_NAME":       "INTERNAL",
			"KAFKA_CONTROLLER_LISTENER_NAMES":        "CONTROLLER",
			"KAFKA_LOG_DIRS":                         "/tmp/kraft-combined-logs",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
			"CLUSTER_ID":                             clusterID,

			// --- Single-broker replication factors ---
			// Confluent Server creates several internal topics (license,
			// cluster-link metadata, audit log, etc.) that default to a
			// replication factor of 3. With a single broker these must be
			// pinned to 1 or the broker fails to start.
			"KAFKA_DEFAULT_REPLICATION_FACTOR":                                    "1",
			"KAFKA_CONFLUENT_LICENSE_TOPIC_REPLICATION_FACTOR":                    "1",
			"KAFKA_CONFLUENT_BALANCER_TOPIC_REPLICATION_FACTOR":                   "1",
			"KAFKA_CONFLUENT_CLUSTER_LINK_METADATA_TOPIC_REPLICATION_FACTOR":      "1",
			"KAFKA_CONFLUENT_SECURITY_EVENT_LOGGER_EXPORTER_KAFKA_TOPIC_REPLICAS": "1",
			"KAFKA_TRANSACTION_STATE_LOG_REPLICATION_FACTOR":                      "1",
			"KAFKA_TRANSACTION_STATE_LOG_MIN_ISR":                                 "1",
			"KAFKA_CONFLUENT_DURABILITY_TOPIC_REPLICATION_FACTOR":                 "1",
			"KAFKA_CONFLUENT_TIER_METADATA_REPLICATION_FACTOR":                    "1",
			"KAFKA_QUOTAS_TOPIC_REPLICATION_FACTOR":                               "1",
			"KAFKA_SHARE_COORDINATOR_STATE_TOPIC_REPLICATION_FACTOR":              "1",
			"KAFKA_LISTENER_NAME_OUTSIDE_SASL_ENABLED_MECHANISMS":                 "SCRAM-SHA-256",
			"KAFKA_SASL_ENABLED_MECHANISMS":                                       "SCRAM-SHA-256",
			"KAFKA_SASL_MECHANISM_INTER_BROKER_PROTOCOL":                          "SCRAM-SHA-256",
			"KAFKA_LISTENER_NAME_OUTSIDE_SCRAM___SHA___256_SASL_JAAS_CONFIG":      "org.apache.kafka.common.security.scram.ScramLoginModule required;",
		},
		// The SCRAM credentials must be provisioned before the broker starts.
		// The cp-kafka image does this via `kafka-storage format --add-scram`,
		// so we override the entrypoint to run the format step first.
		Cmd: []string{
			"bash",
			"-c",
			`
			. /etc/confluent/docker/bash-config

			echo "===> Configuring ..."
			/etc/confluent/docker/configure

			echo "===> Using provided cluster id $CLUSTER_ID with SCRAM credentials..."
			result=$(kafka-storage format --cluster-id=$CLUSTER_ID -c /etc/kafka/kafka.properties --add-scram 'SCRAM-SHA-256=[name=admin,password=admin-secret]' 2>&1) || \
				echo $result | grep -i "already formatted" || \
				{ echo $result && (exit 1) }

			echo "===> Launching ... "
			exec /etc/confluent/docker/launch
			`,
		},
		HostConfigModifier: fixedPortBinding("9092/tcp", hostPort),
		WaitingFor:         wait.ForLog("Kafka Server started").WithStartupTimeout(90 * time.Second),
	}

	c := startContainer(t, req)

	return []string{fmt.Sprintf("localhost:%d", hostPort)}, terminateFn(c), ScramConfig{
		Username: "admin",
		Password: "admin-secret",
	}
}

// generateMTLSCertificates generates a self-signed CA, a server certificate
// (for "localhost"), and a client certificate, all signed by the CA. It writes
// the PEM-encoded CA, client certificate, and client key to the given
// directory, and also produces PKCS12 keystore/truststore files for the Kafka
// broker (which is a JVM and therefore consumes PKCS12 rather than PEM).
//
// It returns the paths to the PEM files (for the Go client) and the PKCS12
// files (for the broker).
func generateMTLSCertificates(t *testing.T, dir string) (caPath, clientCertPath, clientKeyPath, keystorePath, truststorePath string) {
	t.Helper()

	// --- CA ---
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate CA key")

	caTemplate := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{Organization: []string{"Marvin Test CA"}, CommonName: "Marvin Test CA"},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err, "failed to create CA certificate")
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err, "failed to parse CA certificate")

	// --- Server certificate ---
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate server key")

	serverTemplate := x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{Organization: []string{"Marvin Test Server"}, CommonName: "localhost"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, &serverTemplate, caCert, &serverKey.PublicKey, caKey)
	require.NoError(t, err, "failed to create server certificate")
	serverCert, err := x509.ParseCertificate(serverDER)
	require.NoError(t, err, "failed to parse server certificate")

	// --- Client certificate ---
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate client key")

	clientTemplate := x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{Organization: []string{"Marvin Test Client"}, CommonName: "client"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, &clientTemplate, caCert, &clientKey.PublicKey, caKey)
	require.NoError(t, err, "failed to create client certificate")

	// --- Write PEM files for the Go client ---
	caPath = filepath.Join(dir, "ca.crt")
	clientCertPath = filepath.Join(dir, "client.crt")
	clientKeyPath = filepath.Join(dir, "client.key")

	require.NoError(t, os.WriteFile(caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0644))
	require.NoError(t, os.WriteFile(clientCertPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER}), 0644))
	require.NoError(t, os.WriteFile(clientKeyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)}), 0600))

	// --- Write PKCS12 keystore/truststore for the broker ---
	keystorePath = filepath.Join(dir, "kafka.keystore.p12")
	truststorePath = filepath.Join(dir, "kafka.truststore.p12")

	// Use the Legacy encoder (DES-based) to match the output of OpenSSL's
	// `pkcs12 -export`, which is what the Confluent demo images and Java's
	// keytool expect. Modern (AES-based) PKCS12 is not readable by older JVMs.
	keystoreP12, err := pkcs12.Legacy.Encode(serverKey, serverCert, []*x509.Certificate{caCert}, keystorePassword)
	require.NoError(t, err, "failed to encode PKCS12 keystore")
	require.NoError(t, os.WriteFile(keystorePath, keystoreP12, 0644))

	truststoreP12, err := pkcs12.Legacy.EncodeTrustStore([]*x509.Certificate{caCert}, keystorePassword)
	require.NoError(t, err, "failed to encode PKCS12 truststore")
	require.NoError(t, os.WriteFile(truststorePath, truststoreP12, 0644))

	return caPath, clientCertPath, clientKeyPath, keystorePath, truststorePath
}

// StartKafkaWithMTLS starts a single-node Kafka broker in KRaft mode with an
// SSL listener that requires mutual TLS (client authentication). Certificates
// are generated in pure Go (crypto/x509 + crypto/rsa) and packaged into PKCS12
// keystores for the broker.
//
// It returns the bootstrap servers, a cleanup function, the client TLS config
// (PEM paths), and the same paths again as a CertPaths struct.
func StartKafkaWithMTLS(t *testing.T) (bootstrapServers []string, cleanup func(), tlsConfig TLSConfig, certPaths CertPaths) {
	t.Helper()

	hostPort := freePort(t)

	dir, err := os.MkdirTemp("", "marvin-kafka-mtls-*")
	require.NoError(t, err, "failed to create temp dir")
	require.NoError(t, os.Chmod(dir, 0o755), "failed to chmod temp dir")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	caPath, clientCertPath, clientKeyPath, keystorePath, truststorePath := generateMTLSCertificates(t, dir)

	// The cp-kafka image's SSL preflight checks for a "confluent" file in the
	// secrets directory; create it to satisfy that check.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "confluent"), []byte("confluent"), 0644))

	req := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-kafka:latest",
		ExposedPorts: []string{"9093/tcp"},
		Env: map[string]string{
			"KAFKA_NODE_ID":                          "1",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":   "CONTROLLER:PLAINTEXT,SSL:SSL",
			"KAFKA_ADVERTISED_LISTENERS":             fmt.Sprintf("SSL://localhost:%d", hostPort),
			"KAFKA_PROCESS_ROLES":                    "broker,controller",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":         "1@localhost:29093",
			"KAFKA_LISTENERS":                        "SSL://0.0.0.0:9093,CONTROLLER://0.0.0.0:29093",
			"KAFKA_INTER_BROKER_LISTENER_NAME":       "SSL",
			"KAFKA_CONTROLLER_LISTENER_NAMES":        "CONTROLLER",
			"KAFKA_LOG_DIRS":                         "/tmp/kraft-combined-logs",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
			"CLUSTER_ID":                             clusterID,
			"KAFKA_SSL_KEYSTORE_LOCATION":            "/etc/kafka/secrets/kafka.keystore.p12",
			"KAFKA_SSL_KEYSTORE_PASSWORD":            keystorePassword,
			"KAFKA_SSL_KEYSTORE_TYPE":                "PKCS12",
			"KAFKA_SSL_KEY_PASSWORD":                 keystorePassword,
			"KAFKA_SSL_TRUSTSTORE_LOCATION":          "/etc/kafka/secrets/kafka.truststore.p12",
			"KAFKA_SSL_TRUSTSTORE_PASSWORD":          keystorePassword,
			"KAFKA_SSL_TRUSTSTORE_TYPE":              "PKCS12",
			"KAFKA_SSL_KEYSTORE_FILENAME":            "kafka.keystore.p12",
			"KAFKA_SSL_TRUSTSTORE_FILENAME":          "kafka.truststore.p12",
			"KAFKA_SSL_KEYSTORE_CREDENTIALS":         keystorePassword,
			"KAFKA_SSL_KEY_CREDENTIALS":              keystorePassword,
			"KAFKA_SSL_TRUSTSTORE_CREDENTIALS":       keystorePassword,
			"KAFKA_SSL_CLIENT_AUTH":                  "required",
		},
		Mounts: testcontainers.ContainerMounts{
			testcontainers.BindMount(dir, "/etc/kafka/secrets"),
		},
		HostConfigModifier: fixedPortBinding("9093/tcp", hostPort),
		WaitingFor:         wait.ForLog("Kafka Server started").WithStartupTimeout(90 * time.Second),
	}

	c := startContainer(t, req)

	_ = keystorePath
	_ = truststorePath

	tlsConfig = TLSConfig{CAPath: caPath, CertPath: clientCertPath, KeyPath: clientKeyPath}
	certPaths = CertPaths{CAPath: caPath, CertPath: clientCertPath, KeyPath: clientKeyPath}

	return []string{fmt.Sprintf("localhost:%d", hostPort)}, terminateFn(c), tlsConfig, certPaths
}

// StartKafkaWithKerberos starts a Kerberos KDC (ubuntu:22.04 with krb5-kdc)
// and a single-node Kafka broker configured for GSSAPI (Kerberos)
// authentication, both attached to a shared Docker network.
//
// The KDC provisions a service principal for the broker and a client principal
// (kafkaclient / kafkaclient-secret), and exports their keytabs. The broker is
// configured with a JAAS config and krb5.conf that reference the KDC.
//
// It returns the bootstrap servers, a cleanup function, the Kerberos config,
// the path to the client keytab, and the path to a krb5.conf suitable for the
// test process.
func StartKafkaWithKerberos(t *testing.T) (bootstrapServers []string, cleanup func(), krbConfig KerberosConfig, clientKeytab string, krb5Conf string) {
	t.Helper()

	const (
		realm   = "EXAMPLE.COM"
		kdcHost = "kdc"
	)

	networkName := createNetwork(t)

	dir, err := os.MkdirTemp("", "marvin-kafka-krb5-*")
	require.NoError(t, err, "failed to create temp dir")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	// --- KDC setup script (runs inside the KDC container) ---
	kdcSetupScript := `#!/bin/bash
set -e

# Create KDC database
kdb5_util create -s -P krbmaster123

# Start KDC and kadmin
krb5kdc
kadmind

# Create service principals for Kafka (both hostnames)
kadmin.local -q "addprinc -randkey kafka/kafka.example.com@EXAMPLE.COM"
kadmin.local -q "addprinc -randkey kafka/localhost@EXAMPLE.COM"

# Create client principal with fixed password
kadmin.local -q "addprinc -pw kafkaclient-secret kafkaclient@EXAMPLE.COM"

# Export keytabs
kadmin.local -q "ktadd -k /keytabs/kafka.keytab kafka/kafka.example.com@EXAMPLE.COM"
kadmin.local -q "ktadd -k /keytabs/kafka.keytab kafka/localhost@EXAMPLE.COM"
kadmin.local -q "ktadd -k /keytabs/client.keytab kafkaclient@EXAMPLE.COM"

# Set permissions
chmod 644 /keytabs/*.keytab

echo "KDC setup complete"
tail -f /dev/null
`

	krb5ConfContent := fmt.Sprintf(`[libdefaults]
    default_realm = %s
    dns_lookup_realm = false
    dns_lookup_kdc = false
    ticket_lifetime = 24h
    renew_lifetime = 7d
    forwardable = true
    default_ccache_name = FILE:/tmp/krb5cc_%%{uid}

[realms]
    %s = {
        kdc = %s:8888
        admin_server = %s
    }

[domain_realm]
    .example.com = %s
    example.com = %s
`, realm, realm, kdcHost, kdcHost, realm, realm)

	kdcConfContent := `[kdcdefaults]
    kdc_ports = 8888
    kdc_tcp_ports = 8888

[realms]
    EXAMPLE.COM = {
        acl_file = /var/kerberos/krb5kdc/kadm5.acl
        dict_file = /usr/share/dict/words
        admin_keytab = /var/kerberos/krb5kdc/kadm5.keytab
        supported_enctypes = aes256-cts:normal aes128-cts:normal
        max_renewable_life = 7d
    }
`

	kadmACLContent := "*/admin@EXAMPLE.COM *\n"

	setupScriptPath := filepath.Join(dir, "kdc-setup.sh")
	require.NoError(t, os.WriteFile(setupScriptPath, []byte(kdcSetupScript), 0755))

	krb5ConfPath := filepath.Join(dir, "krb5.conf")
	require.NoError(t, os.WriteFile(krb5ConfPath, []byte(krb5ConfContent), 0644))

	kdcConfPath := filepath.Join(dir, "kdc.conf")
	require.NoError(t, os.WriteFile(kdcConfPath, []byte(kdcConfContent), 0644))

	kadmACLPath := filepath.Join(dir, "kadm5.acl")
	require.NoError(t, os.WriteFile(kadmACLPath, []byte(kadmACLContent), 0644))

	keytabsDir := filepath.Join(dir, "keytabs")
	require.NoError(t, os.MkdirAll(keytabsDir, 0755))

	// --- KDC container ---
	kdcReq := testcontainers.ContainerRequest{
		Image:        "ubuntu:22.04",
		ExposedPorts: []string{"8888/tcp", "8888/udp", "749/tcp"},
		Hostname:     kdcHost,
		Networks:     []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {kdcHost},
		},
		Mounts: testcontainers.ContainerMounts{
			testcontainers.BindMount(setupScriptPath, "/kdc-setup.sh"),
			testcontainers.BindMount(krb5ConfPath, "/etc/krb5.conf"),
			testcontainers.BindMount(kdcConfPath, "/var/kerberos/krb5kdc/kdc.conf"),
			testcontainers.BindMount(kadmACLPath, "/var/kerberos/krb5kdc/kadm5.acl"),
			testcontainers.BindMount(keytabsDir, "/keytabs"),
		},
		Cmd: []string{
			"bash",
			"-c",
			`
			apt-get update && apt-get install -y krb5-kdc krb5-admin-server krb5-user dnsutils
			mkdir -p /var/kerberos/krb5kdc
			cp /var/kerberos/krb5kdc/kdc.conf /etc/krb5kdc/ || true
			/kdc-setup.sh
			`,
		},
		WaitingFor: wait.ForLog("KDC setup complete").WithStartupTimeout(180 * time.Second),
	}

	kdcContainer := startContainer(t, kdcReq)

	// Give the KDC a moment to fully start listening before the broker tries
	// to authenticate against it.
	time.Sleep(5 * time.Second)

	// --- Kafka broker with GSSAPI ---
	hostPort := freePort(t)

	localhostPrincipal := "kafka/localhost@EXAMPLE.COM"

	jaasConfig := fmt.Sprintf(`KafkaServer {
    com.sun.security.auth.module.Krb5LoginModule required
    useKeyTab=true
    storeKey=true
    keyTab="/etc/kafka/secrets/kafka.keytab"
    principal="%s";
};

KafkaClient {
    com.sun.security.auth.module.Krb5LoginModule required
    useKeyTab=true
    storeKey=true
    keyTab="/etc/kafka/secrets/kafka.keytab"
    principal="%s";
};
`, localhostPrincipal, localhostPrincipal)

	jaasPath := filepath.Join(dir, "kafka_server_jaas.conf")
	require.NoError(t, os.WriteFile(jaasPath, []byte(jaasConfig), 0644))

	kafkaReq := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-kafka:latest",
		ExposedPorts: []string{"9092/tcp"},
		Hostname:     "kafka.example.com",
		Networks:     []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"kafka.example.com"},
		},
		Env: map[string]string{
			"KAFKA_NODE_ID":                                       "1",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":                "CONTROLLER:PLAINTEXT,OUTSIDE:SASL_PLAINTEXT,INTERNAL:PLAINTEXT",
			"KAFKA_ADVERTISED_LISTENERS":                          fmt.Sprintf("OUTSIDE://localhost:%d,INTERNAL://kafka.example.com:9093", hostPort),
			"KAFKA_PROCESS_ROLES":                                 "broker,controller",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":                      "1@kafka.example.com:29093",
			"KAFKA_LISTENERS":                                     "OUTSIDE://0.0.0.0:9092,CONTROLLER://0.0.0.0:29093,INTERNAL://0.0.0.0:9093",
			"KAFKA_INTER_BROKER_LISTENER_NAME":                    "INTERNAL",
			"KAFKA_CONTROLLER_LISTENER_NAMES":                     "CONTROLLER",
			"KAFKA_LOG_DIRS":                                      "/tmp/kraft-combined-logs",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR":              "1",
			"CLUSTER_ID":                                          clusterID,
			"KAFKA_LISTENER_NAME_OUTSIDE_SASL_ENABLED_MECHANISMS": "GSSAPI",
			"KAFKA_SASL_ENABLED_MECHANISMS":                       "GSSAPI",
			"KAFKA_SASL_KERBEROS_SERVICE_NAME":                    "kafka",
			"KAFKA_LISTENER_NAME_OUTSIDE_GSSAPI_SASL_JAAS_CONFIG": fmt.Sprintf(`com.sun.security.auth.module.Krb5LoginModule required useKeyTab=true storeKey=true keyTab="/etc/kafka/secrets/kafka.keytab" principal="%s";`, localhostPrincipal),
			"KAFKA_ALLOW_EVERYONE_IF_NO_ACL_FOUND":                "true",
			"KAFKA_SUPER_USERS":                                   "User:kafkaclient",
			"KAFKA_OPTS":                                          "-Djava.security.krb5.conf=/etc/kafka/secrets/krb5.conf -Djava.security.auth.login.config=/etc/kafka/secrets/kafka_server_jaas.conf",
		},
		Mounts: testcontainers.ContainerMounts{
			testcontainers.BindMount(keytabsDir, "/etc/kafka/secrets"),
			testcontainers.BindMount(krb5ConfPath, "/etc/kafka/secrets/krb5.conf"),
			testcontainers.BindMount(jaasPath, "/etc/kafka/secrets/kafka_server_jaas.conf"),
		},
		HostConfigModifier: fixedPortBinding("9092/tcp", hostPort),
		WaitingFor:         wait.ForLog("Kafka Server started").WithStartupTimeout(120 * time.Second),
	}

	kafkaContainer := startContainer(t, kafkaReq)

	// Build a krb5.conf for the test process that points at the KDC's mapped
	// host port.
	kdcMappedPort, err := kdcContainer.MappedPort(context.Background(), "8888/tcp")
	require.NoError(t, err, "failed to get KDC mapped port")

	clientKrb5ConfContent := fmt.Sprintf(`[libdefaults]
    default_realm = %s
    dns_lookup_realm = false
    dns_lookup_kdc = false
    ticket_lifetime = 24h
    renew_lifetime = 7d
    forwardable = true
    udp_preference_limit = 1
    default_ccache_name = FILE:/tmp/krb5cc_%%{uid}

[realms]
    %s = {
        kdc = localhost:%s
        admin_server = localhost:%s
    }

[domain_realm]
    .example.com = %s
    example.com = %s
`, realm, realm, kdcMappedPort.Port(), kdcMappedPort.Port(), realm, realm)

	clientKrb5ConfPath := filepath.Join(dir, "client-krb5.conf")
	require.NoError(t, os.WriteFile(clientKrb5ConfPath, []byte(clientKrb5ConfContent), 0644))

	clientKeytabPath := filepath.Join(keytabsDir, "client.keytab")

	krbConfig = KerberosConfig{
		Realm:       realm,
		KDC:         kdcHost,
		ServiceName: "kafka",
		Principal:   localhostPrincipal,
	}

	cleanup = newCleanup(t, terminateFn(kafkaContainer), terminateFn(kdcContainer))

	return []string{fmt.Sprintf("localhost:%d", hostPort)}, cleanup, krbConfig, clientKeytabPath, clientKrb5ConfPath
}

// StartKafkaWithSchemaRegistry starts a single-node Kafka broker and a Schema
// Registry, both attached to a shared Docker network. The broker exposes an
// internal PLAINTEXT listener for the Schema Registry and an external
// PLAINTEXT_HOST listener for the test process.
//
// It returns the bootstrap servers, the externally reachable Schema Registry
// URL, and a cleanup function.
func StartKafkaWithSchemaRegistry(t *testing.T) (bootstrapServers []string, srURL string, cleanup func()) {
	t.Helper()

	networkName := createNetwork(t)

	kafkaHostPort := freePort(t)

	kafkaReq := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-kafka:latest",
		ExposedPorts: []string{"9093/tcp"},
		Env: map[string]string{
			"KAFKA_NODE_ID":                          "1",
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":   "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT",
			"KAFKA_ADVERTISED_LISTENERS":             fmt.Sprintf("PLAINTEXT://kafka:9092,PLAINTEXT_HOST://localhost:%d", kafkaHostPort),
			"KAFKA_PROCESS_ROLES":                    "broker,controller",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":         "1@localhost:29093",
			"KAFKA_LISTENERS":                        "PLAINTEXT://0.0.0.0:9092,PLAINTEXT_HOST://0.0.0.0:9093,CONTROLLER://0.0.0.0:29093",
			"KAFKA_INTER_BROKER_LISTENER_NAME":       "PLAINTEXT",
			"KAFKA_CONTROLLER_LISTENER_NAMES":        "CONTROLLER",
			"KAFKA_LOG_DIRS":                         "/tmp/kraft-combined-logs",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
			"CLUSTER_ID":                             clusterID,
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"kafka"},
		},
		HostConfigModifier: fixedPortBinding("9093/tcp", kafkaHostPort),
		WaitingFor:         wait.ForLog("Kafka Server started").WithStartupTimeout(90 * time.Second),
	}

	kafkaContainer := startContainer(t, kafkaReq)

	// Give Kafka a moment to be fully ready before the Schema Registry connects.
	time.Sleep(5 * time.Second)

	srReq := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-schema-registry:latest",
		ExposedPorts: []string{"8081/tcp"},
		Env: map[string]string{
			"SCHEMA_REGISTRY_HOST_NAME":                    "schema-registry",
			"SCHEMA_REGISTRY_KAFKASTORE_BOOTSTRAP_SERVERS": "kafka:9092",
			"SCHEMA_REGISTRY_LISTENERS":                    "http://0.0.0.0:8081",
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"schema-registry"},
		},
		WaitingFor: wait.ForLog("Server started, listening for requests").WithStartupTimeout(90 * time.Second),
	}

	srContainer := startContainer(t, srReq)

	srMappedPort, err := srContainer.MappedPort(context.Background(), "8081/tcp")
	require.NoError(t, err, "failed to get schema registry mapped port")

	srURL = fmt.Sprintf("http://localhost:%s", srMappedPort.Port())

	cleanup = newCleanup(t, terminateFn(kafkaContainer), terminateFn(srContainer))

	return []string{fmt.Sprintf("localhost:%d", kafkaHostPort)}, srURL, cleanup
}

// generateTokenKeypair generates an RSA keypair for MDS token signing and
// writes the private key (keypair.pem) and public key (public.pem) to the
// given directory. MDS uses the private key to sign OAuthBearer tokens and the
// public key to verify them.
func generateTokenKeypair(t *testing.T, dir string) (keypairPath, publicPath string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate token signing key")

	keypairPath = filepath.Join(dir, "keypair.pem")
	publicPath = filepath.Join(dir, "public.pem")

	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	require.NoError(t, os.WriteFile(keypairPath, keyPEM, 0644))

	pubDER, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err, "failed to marshal public key")
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	require.NoError(t, os.WriteFile(publicPath, pubPEM, 0644))

	return keypairPath, publicPath
}

// StartKafkaWithRBAC starts a single-node Confluent Server (cp-server) broker
// with MDS (Metadata Service) and RBAC enabled. The broker exposes an external
// SASL_PLAINTEXT listener secured by OAuthBearer tokens issued by MDS, and an
// MDS HTTP listener on port 8090.
//
// This is a minimal single-broker port of the Confluent RBAC demo stack: it
// configures the ConfluentServerAuthorizer, a token-signing keypair, and
// super users. It returns the bootstrap servers, the externally reachable MDS
// URL, a cleanup function, and the RBAC config (including the token endpoint).
func StartKafkaWithRBAC(t *testing.T) (bootstrapServers []string, mdsURL string, cleanup func(), rbacConfig RBACConfig) {
	t.Helper()

	hostPort := freePort(t)
	mdsHostPort := freePort(t)

	dir, err := os.MkdirTemp("", "marvin-kafka-rbac-*")
	require.NoError(t, err, "failed to create temp dir")
	require.NoError(t, os.Chmod(dir, 0o755), "failed to chmod temp dir")
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	keypairPath, publicPath := generateTokenKeypair(t, dir)

	// MDS authenticates users against a FILE-based user store. The file maps
	// "principal:password,role" entries; admin is granted the SystemAdmin role
	// so it can bootstrap RBAC bindings.
	usersPath := filepath.Join(dir, "users.properties")
	require.NoError(t, os.WriteFile(usersPath, []byte("admin:admin-secret,SystemAdmin\n"), 0600))

	// The internal listener uses PLAIN SASL for broker-to-broker and
	// MDS-to-broker communication. The external listener uses OAUTHBEARER
	// tokens issued by MDS.
	req := testcontainers.ContainerRequest{
		Image:        "confluentinc/cp-server:latest",
		ExposedPorts: []string{"9092/tcp", "8090/tcp"},
		Env: map[string]string{
			// --- Broker basics (KRaft) ---
			"KAFKA_NODE_ID":                          "1",
			"KAFKA_PROCESS_ROLES":                    "broker,controller",
			"KAFKA_CONTROLLER_QUORUM_VOTERS":         "1@localhost:29093",
			"KAFKA_LISTENERS":                        "INTERNAL://0.0.0.0:9093,EXTERNAL://0.0.0.0:9092,CONTROLLER://0.0.0.0:29093",
			"KAFKA_ADVERTISED_LISTENERS":             fmt.Sprintf("INTERNAL://localhost:9093,EXTERNAL://localhost:%d", hostPort),
			"KAFKA_LISTENER_SECURITY_PROTOCOL_MAP":   "INTERNAL:SASL_PLAINTEXT,EXTERNAL:SASL_PLAINTEXT,CONTROLLER:PLAINTEXT",
			"KAFKA_INTER_BROKER_LISTENER_NAME":       "INTERNAL",
			"KAFKA_CONTROLLER_LISTENER_NAMES":        "CONTROLLER",
			"KAFKA_LOG_DIRS":                         "/tmp/kraft-combined-logs",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
			"CLUSTER_ID":                             clusterID,

			// --- Super users (unlimited access) ---
			"KAFKA_SUPER_USERS": "User:admin;User:mds;User:ANONYMOUS",

			// --- Internal listener (PLAIN) ---
			"KAFKA_SASL_MECHANISM_INTER_BROKER_PROTOCOL":           "PLAIN",
			"KAFKA_LISTENER_NAME_INTERNAL_SASL_ENABLED_MECHANISMS": "PLAIN",
			"KAFKA_LISTENER_NAME_INTERNAL_PLAIN_SASL_JAAS_CONFIG":  "org.apache.kafka.common.security.plain.PlainLoginModule required username=\"admin\" password=\"admin-secret\" user_admin=\"admin-secret\" user_mds=\"mds-secret\";",

			// --- External listener (OAUTHBEARER) ---
			"KAFKA_LISTENER_NAME_EXTERNAL_SASL_ENABLED_MECHANISMS":                        "OAUTHBEARER",
			"KAFKA_LISTENER_NAME_EXTERNAL_OAUTHBEARER_SASL_SERVER_CALLBACK_HANDLER_CLASS": "io.confluent.kafka.server.plugins.auth.token.TokenBearerValidatorCallbackHandler",
			"KAFKA_LISTENER_NAME_EXTERNAL_OAUTHBEARER_SASL_LOGIN_CALLBACK_HANDLER_CLASS":  "io.confluent.kafka.server.plugins.auth.token.TokenBearerServerLoginCallbackHandler",
			"KAFKA_LISTENER_NAME_EXTERNAL_OAUTHBEARER_SASL_JAAS_CONFIG":                   "org.apache.kafka.common.security.oauthbearer.OAuthBearerLoginModule required publicKeyPath=\"/tmp/conf/public.pem\";",

			// --- Authorizer ---
			"KAFKA_AUTHORIZER_CLASS_NAME":                      "io.confluent.kafka.security.authorizer.ConfluentServerAuthorizer",
			"KAFKA_CONFLUENT_AUTHORIZER_ACCESS_RULE_PROVIDERS": "CONFLUENT",

			// --- MDS-to-broker connection ---
			"KAFKA_CONFLUENT_METADATA_BOOTSTRAP_SERVERS":        "INTERNAL://localhost:9093",
			"KAFKA_CONFLUENT_METADATA_SASL_MECHANISM":           "PLAIN",
			"KAFKA_CONFLUENT_METADATA_SASL_JAAS_CONFIG":         "org.apache.kafka.common.security.plain.PlainLoginModule required username=\"mds\" password=\"mds-secret\";",
			"KAFKA_CONFLUENT_METADATA_TOPIC_REPLICATION_FACTOR": "1",

			// --- MDS HTTP server ---
			"KAFKA_CONFLUENT_METADATA_SERVER_AUTHENTICATION_METHOD": "BEARER",
			"KAFKA_CONFLUENT_METADATA_SERVER_LISTENERS":             "http://0.0.0.0:8090",
			"KAFKA_CONFLUENT_METADATA_SERVER_ADVERTISED_LISTENERS":  "http://localhost:8090",
			"KAFKA_CONFLUENT_METADATA_SERVER_USER_STORE":            "FILE",
			"KAFKA_CONFLUENT_METADATA_SERVER_USER_STORE_FILE_PATH":  "/tmp/conf/users.properties",

			// --- MDS token server ---
			"KAFKA_CONFLUENT_METADATA_SERVER_TOKEN_AUTH_ENABLE":         "true",
			"KAFKA_CONFLUENT_METADATA_SERVER_TOKEN_MAX_LIFETIME_MS":     "3600000",
			"KAFKA_CONFLUENT_METADATA_SERVER_TOKEN_SIGNATURE_ALGORITHM": "RS256",
			"KAFKA_CONFLUENT_METADATA_SERVER_TOKEN_KEY_PATH":            "/tmp/conf/keypair.pem",
			"KAFKA_CONFLUENT_METADATA_SERVER_PUBLIC_KEY_PATH":           "/tmp/conf/public.pem",
		},
		Mounts: testcontainers.ContainerMounts{
			testcontainers.BindMount(dir, "/tmp/conf"),
		},
		HostConfigModifier: func(hc *container.HostConfig) {
			hc.PortBindings = network.PortMap{
				network.MustParsePort("9092/tcp"): {{HostPort: strconv.Itoa(hostPort)}},
				network.MustParsePort("8090/tcp"): {{HostPort: strconv.Itoa(mdsHostPort)}},
			}
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("Started NetworkTrafficServerConnector").WithStartupTimeout(120*time.Second),
			wait.ForListeningPort("8090/tcp").WithStartupTimeout(120*time.Second),
		),
	}

	c := startContainer(t, req)

	_ = keypairPath
	_ = publicPath

	mdsURL = fmt.Sprintf("http://localhost:%d", mdsHostPort)

	rbacConfig = RBACConfig{
		MDSURL:        mdsURL,
		Principal:     "admin",
		Password:      "admin-secret",
		TokenEndpoint: fmt.Sprintf("http://localhost:%d/security/1.0/authenticate", mdsHostPort),
	}

	return []string{fmt.Sprintf("localhost:%d", hostPort)}, mdsURL, terminateFn(c), rbacConfig
}
