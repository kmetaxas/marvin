package network

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/marvin-agent/marvin/internal/task"
)

// tlsTask performs TLS handshakes and returns certificate details.
type tlsTask struct{}

const tlsSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "title": "TLS Handshake Parameters",
  "description": "Parameters for performing a TLS handshake and inspecting the certificate chain.",
  "properties": {
    "target": {
      "type": "string",
      "description": "Target host and port to connect to (e.g., 'example.com:443')."
    },
    "server_name": {
      "type": "string",
      "description": "Server Name Indication (SNI) for TLS verification. Defaults to the host from target."
    },
    "ca_cert": {
      "type": "string",
      "description": "PEM-encoded CA certificate(s) to trust for verification."
    },
    "client_cert": {
      "type": "string",
      "description": "PEM-encoded client certificate for mutual TLS."
    },
    "client_key": {
      "type": "string",
      "description": "PEM-encoded client private key for mutual TLS."
    },
    "insecure_skip_verify": {
      "type": "boolean",
      "description": "If true, disables certificate chain verification. Not recommended for production.",
      "default": false
    }
  },
  "required": ["target"]
}`

func (t *tlsTask) Name() string { return "network.tls.handshake" }

func (t *tlsTask) JSONSchema() string { return tlsSchema }

func (t *tlsTask) Execute(ctx context.Context, params map[string]any) (task.Result, error) {
	start := time.Now()
	result := task.Result{
		Success:   true,
		Timestamp: start,
	}

	target, ok := params["target"].(string)
	if !ok || target == "" {
		result.Success = false
		result.Error = "missing required parameter: target"
		return result, nil
	}

	serverName, _ := params["server_name"].(string)
	caCertPEM, _ := params["ca_cert"].(string)
	clientCertPEM, _ := params["client_cert"].(string)
	clientKeyPEM, _ := params["client_key"].(string)
	insecure, _ := params["insecure_skip_verify"].(bool)

	slog.Info("TLS handshake starting", "capability", t.Name(), "target", target, "insecure", insecure)
	slog.Debug("TLS handshake parameters", "capability", t.Name(), "target", target, "server_name", serverName, "has_ca_cert", caCertPEM != "", "has_client_cert", clientCertPEM != "", "has_client_key", clientKeyPEM != "")

	var tlsCfg tls.Config

	if caCertPEM != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(caCertPEM)) {
			result.Success = false
			result.Error = "failed to parse CA certificate"
			slog.Info("TLS handshake failed: CA certificate parse error", "capability", t.Name())
			return result, nil
		}
		tlsCfg.RootCAs = pool
	}

	if clientCertPEM != "" && clientKeyPEM != "" {
		cert, err := tls.X509KeyPair([]byte(clientCertPEM), []byte(clientKeyPEM))
		if err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("failed to load client certificate: %v", err)
			slog.Info("TLS handshake failed: client certificate load error", "capability", t.Name(), "error", err)
			return result, nil
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	// Parse host from target (e.g. "example.com:443" -> "example.com")
	host, _, err := net.SplitHostPort(target)
	if err != nil {
		// target may not have a port
		host = target
	}

	// If server_name is provided, use it. Otherwise use the host from target.
	// If the host is an IP address, we must skip Go's default verification
	// and perform manual verification ourselves (Go's tls requires ServerName
	// or InsecureSkipVerify when dialing an IP).
	isIP := net.ParseIP(host) != nil
	if serverName != "" {
		tlsCfg.ServerName = serverName
	} else if !isIP {
		tlsCfg.ServerName = host
	} else {
		// IP target without explicit server_name: skip default verification
		// so the handshake proceeds; we validate manually below.
		tlsCfg.InsecureSkipVerify = true
	}

	// Only override with explicit user param if they actually set it
	if insecure {
		tlsCfg.InsecureSkipVerify = true
	}

	dialer := net.Dialer{Timeout: 10 * time.Second}
	slog.Debug("TLS TCP dial starting", "capability", t.Name(), "target", target)
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to connect: %v", err)
		slog.Info("TLS handshake failed: TCP connection error", "capability", t.Name(), "target", target, "error", err)
		return result, nil
	}
	defer conn.Close()

	tlsConn := tls.Client(conn, &tlsCfg)
	if err := tlsConn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("failed to set deadline: %v", err)
		slog.Info("TLS handshake failed: deadline error", "capability", t.Name(), "error", err)
		return result, nil
	}

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		result.Success = false
		result.Error = fmt.Sprintf("TLS handshake failed: %v", err)
		slog.Info("TLS handshake failed", "capability", t.Name(), "target", target, "error", err)
		return result, nil
	}
	defer tlsConn.Close()

	state := tlsConn.ConnectionState()
	validationOk := true
	validationErr := ""

	if !insecure {
		// Verify the peer certificate chain against our configured roots.
		var roots *x509.CertPool
		if tlsCfg.RootCAs != nil {
			roots = tlsCfg.RootCAs
		} else {
			var err error
			roots, err = x509.SystemCertPool()
			if err != nil {
				// Fallback if system pool unavailable
				roots = x509.NewCertPool()
			}
		}

		opts := x509.VerifyOptions{
			Roots:         roots,
			DNSName:       serverName,
			Intermediates: x509.NewCertPool(),
		}
		if len(state.PeerCertificates) > 1 {
			for _, cert := range state.PeerCertificates[1:] {
				opts.Intermediates.AddCert(cert)
			}
		}
		if _, err := state.PeerCertificates[0].Verify(opts); err != nil {
			validationOk = false
			validationErr = err.Error()
		}
	}

	var certs []certInfo
	for _, cert := range state.PeerCertificates {
		certs = append(certs, certInfo{
			Subject:   cert.Subject.String(),
			Issuer:    cert.Issuer.String(),
			NotBefore: cert.NotBefore.Format(time.RFC3339),
			NotAfter:  cert.NotAfter.Format(time.RFC3339),
			Serial:    cert.SerialNumber.String(),
		})
	}

	result.Data = map[string]any{
		"handshake_ok":     true,
		"validation_ok":    validationOk,
		"tls_version":      tlsVersionName(state.Version),
		"cipher_suite":     tls.CipherSuiteName(state.CipherSuite),
		"certificates":     certs,
		"validation_error": validationErr,
		"duration_ms":      time.Since(start).Milliseconds(),
	}
	slog.Info("TLS handshake succeeded", "capability", t.Name(), "target", target, "tls_version", tlsVersionName(state.Version), "validation_ok", validationOk)
	slog.Debug("TLS handshake details", "capability", t.Name(), "target", target, "cipher_suite", tls.CipherSuiteName(state.CipherSuite), "cert_count", len(certs), "duration_ms", time.Since(start).Milliseconds())
	return result, nil
}

type certInfo struct {
	Subject   string `json:"subject"`
	Issuer    string `json:"issuer"`
	NotBefore string `json:"not_before"`
	NotAfter  string `json:"not_after"`
	Serial    string `json:"serial"`
}

func tlsVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "1.0"
	case tls.VersionTLS11:
		return "1.1"
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS13:
		return "1.3"
	default:
		return fmt.Sprintf("unknown(%d)", version)
	}
}

// parsePEMCerts extracts all certificates from a PEM block.
func parsePEMCerts(pemData string) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	for {
		block, rest := pem.Decode([]byte(pemData))
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			pemData = string(rest)
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, cert)
		pemData = string(rest)
	}
	return certs, nil
}
