// Package testutil provides testcontainers-based helpers for spinning up
// Kafka (and Kafka-adjacent) infrastructure for integration tests.
//
// The helpers in this package are designed to be used from the kafka
// provider's integration tests (see internal/provider/kafka/integration_test.go)
// and follow the same conventions as the prometheus provider's integration
// test: each helper starts one or more containers, registers their termination
// via t.Cleanup, and returns the externally reachable connection details.
//
// All helpers use KRaft mode (no ZooKeeper) where possible, which keeps the
// topology to a single broker and dramatically simplifies startup. The
// following images are used:
//
//   - confluentinc/cp-kafka:latest          — plaintext, SCRAM, mTLS, Kerberos
//   - confluentinc/cp-schema-registry:latest — Schema Registry
//   - confluentinc/cp-server:latest          — MDS / RBAC (cp-server is required
//     because MDS is not available in the plain cp-kafka image)
//   - ubuntu:22.04                           — Kerberos KDC (krb5-kdc)
//
// Everything is pure Go: certificates are generated with crypto/x509 and
// crypto/rsa, PKCS12 keystores with software.sslmate.com/src/go-pkcs12, and
// Kerberos keytabs are produced inside the KDC container. No CGO is used.
package testutil
