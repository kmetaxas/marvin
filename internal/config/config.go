package config

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	agentmetadata "github.com/marvin-agent/marvin/internal/metadata"
	"github.com/marvin-agent/marvin/pkg/capability"
	"github.com/marvin-agent/marvin/pkg/log"
	"gopkg.in/yaml.v3"
)

type Config struct {
	AgentID         string                 `yaml:"agent_id"`
	RegistrationKey string                 `yaml:"registration_key"`
	Labels          []string               `yaml:"labels"`
	ControlPlane    ControlPlane           `yaml:"control_plane"`
	Authentication  Authentication         `yaml:"authentication"`
	Metadata        agentmetadata.Metadata `yaml:"metadata"`
	Capabilities    Capabilities           `yaml:"capabilities"`
	Logging         Logging                `yaml:"logging"`
	Azure           AzureConfig            `yaml:"azure"`
	Kubernetes      KubernetesConfig       `yaml:"kubernetes"`
	Prometheus      PrometheusConfig       `yaml:"prometheus"`
	Kafka           KafkaConfig            `yaml:"kafka"`
	Linux           LinuxConfig            `yaml:"linux"`
	Graylog         GraylogConfig          `yaml:"graylog"`
}

type PrometheusConfig struct {
	URL        string               `yaml:"url" json:"url"`
	Timeout    time.Duration        `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Auth       PrometheusAuthConfig `yaml:"auth,omitempty" json:"auth,omitempty"`
	Guardrails PrometheusGuardrails `yaml:"guardrails,omitempty" json:"guardrails,omitempty"`
}

type PrometheusAuthConfig struct {
	Type        string `yaml:"type,omitempty" json:"type,omitempty"`
	Username    string `yaml:"username,omitempty" json:"username,omitempty"`
	Password    string `yaml:"password,omitempty" json:"password,omitempty"`
	Token       string `yaml:"token,omitempty" json:"token,omitempty"`
	HeaderName  string `yaml:"header_name,omitempty" json:"header_name,omitempty"`
	HeaderValue string `yaml:"header_value,omitempty" json:"header_value,omitempty"`
}

type PrometheusGuardrails struct {
	MaxQueryRange      time.Duration `yaml:"max_query_range,omitempty" json:"max_query_range,omitempty"`
	MinStep            time.Duration `yaml:"min_step,omitempty" json:"min_step,omitempty"`
	QueryTimeout       time.Duration `yaml:"query_timeout,omitempty" json:"query_timeout,omitempty"`
	MaxReturnedSeries  int           `yaml:"max_returned_series,omitempty" json:"max_returned_series,omitempty"`
	MaxReturnedSamples int           `yaml:"max_returned_samples,omitempty" json:"max_returned_samples,omitempty"`
	MaxMetadataResults int           `yaml:"max_metadata_results,omitempty" json:"max_metadata_results,omitempty"`
	MaxResponseBytes   int           `yaml:"max_response_bytes,omitempty" json:"max_response_bytes,omitempty"`
}

type KubernetesConfig struct {
	KubeconfigPath string `yaml:"kubeconfig_path,omitempty"`
	Context        string `yaml:"context,omitempty"`
	Namespace      string `yaml:"namespace,omitempty"`
	// MaxPodLogLines is the hard upper limit on the number of log lines
	// that kubernetes.pod.logs will return. A value of 0 means 100.
	MaxPodLogLines int `yaml:"max_pod_log_lines,omitempty"`
}

type KafkaConfig struct {
	BootstrapServers []string                  `yaml:"bootstrap_servers"`
	TLS              KafkaTLSConfig            `yaml:"tls,omitempty"`
	SASL             KafkaSASLConfig           `yaml:"sasl,omitempty"`
	DialTimeout      time.Duration             `yaml:"dial_timeout,omitempty"`
	RequestTimeout   time.Duration             `yaml:"request_timeout,omitempty"`
	MetadataMaxAge   time.Duration             `yaml:"metadata_max_age,omitempty"`
	Guardrails       KafkaGuardrails           `yaml:"guardrails,omitempty"`
	SchemaRegistry   KafkaSchemaRegistryConfig `yaml:"schema_registry,omitempty"`
	MDS              KafkaMDSConfig            `yaml:"mds,omitempty"`
	Connect          KafkaConnectConfig        `yaml:"connect,omitempty"`
}

type KafkaTLSConfig struct {
	Enabled  bool   `yaml:"enabled,omitempty"`
	CAFile   string `yaml:"ca_file,omitempty"`
	CAData   string `yaml:"ca_data,omitempty"`
	CertFile string `yaml:"cert_file,omitempty"`
	CertData string `yaml:"cert_data,omitempty"`
	KeyFile  string `yaml:"key_file,omitempty"`
	KeyData  string `yaml:"key_data,omitempty"`
	Insecure bool   `yaml:"insecure_skip_verify,omitempty"`
}

type KafkaSASLConfig struct {
	Mechanism           string `yaml:"mechanism,omitempty"`
	Username            string `yaml:"username,omitempty"`
	Password            string `yaml:"password,omitempty"`
	KerberosServiceName string `yaml:"kerberos_service_name,omitempty"`
	KerberosRealm       string `yaml:"kerberos_realm,omitempty"`
	KeytabPath          string `yaml:"keytab_path,omitempty"`
	KeytabData          string `yaml:"keytab_data,omitempty"`
	Krb5ConfData        string `yaml:"krb5_conf_data,omitempty"`
	Krb5ConfPath        string `yaml:"krb5_conf_path,omitempty"`
}

type KafkaGuardrails struct {
	MaxTopicsPerRequest     int           `yaml:"max_topics_per_request,omitempty"`
	MaxPartitionsPerRequest int           `yaml:"max_partitions_per_request,omitempty"`
	MaxConsumerGroups       int           `yaml:"max_consumer_groups,omitempty"`
	MaxRecordsPerSample     int           `yaml:"max_records_per_sample,omitempty"`
	MaxResponseBytes        int           `yaml:"max_response_bytes,omitempty"`
	MaxQueryDuration        time.Duration `yaml:"max_query_duration,omitempty"`
}

type KafkaSchemaRegistryConfig struct {
	URL  string          `yaml:"url,omitempty"`
	TLS  KafkaTLSConfig  `yaml:"tls,omitempty"`
	Auth KafkaSASLConfig `yaml:"auth,omitempty"`
}

type KafkaMDSConfig struct {
	URL  string          `yaml:"url,omitempty"`
	TLS  KafkaTLSConfig  `yaml:"tls,omitempty"`
	Auth KafkaSASLConfig `yaml:"auth,omitempty"`
}

type KafkaConnectConfig struct {
	Clusters []KafkaConnectClusterConfig `yaml:"clusters,omitempty"`
}

type KafkaConnectClusterConfig struct {
	Name string          `yaml:"name"`
	URL  string          `yaml:"url"`
	TLS  KafkaTLSConfig  `yaml:"tls,omitempty"`
	Auth KafkaSASLConfig `yaml:"auth,omitempty"`
}

type AzureConfig struct {
	TenantID       string `yaml:"tenant_id"`
	ClientID       string `yaml:"client_id"`
	ClientSecret   string `yaml:"client_secret"`
	SubscriptionID string `yaml:"subscription_id"`
	ResourceGroup  string `yaml:"resource_group,omitempty"`
	WorkspaceID    string `yaml:"workspace_id,omitempty"`
}

type Logging struct {
	Level string `yaml:"level"`
}

type ControlPlane struct {
	Address string    `yaml:"address"`
	TLS     TLSConfig `yaml:"tls"`
}

type TLSConfig struct {
	Enabled            *bool  `yaml:"enabled,omitempty"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
	CACertFile         string `yaml:"ca_cert_file"`
}

type Authentication struct {
	SecretKey *SecretKeyAuth `yaml:"secret_key,omitempty"`
	MTLS      *MTLSAuth      `yaml:"mtls,omitempty"`
}

type SecretKeyAuth struct {
	Key string `yaml:"key"`
}

type MTLSAuth struct {
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

type LinuxConfig struct {
	AllowedReadPaths    []string `yaml:"allowed_read_paths,omitempty"`
	MaxReadBytes        int      `yaml:"max_read_bytes,omitempty"`
	MaxReadOffset       int      `yaml:"max_read_offset,omitempty"`
	MaxDirectoryDepth   int      `yaml:"max_directory_depth,omitempty"`
	MaxDirectoryEntries int      `yaml:"max_directory_entries,omitempty"`
	RedactSecrets       *bool    `yaml:"redact_secrets,omitempty"`
	SecretPatterns      []string `yaml:"secret_patterns,omitempty"`
	JournalDirectory    string   `yaml:"journal_directory,omitempty"`
	SystemdBusAddress   string   `yaml:"systemd_bus_address,omitempty"`
}

type GraylogConfig struct {
	URL        string            `yaml:"url"`
	Timeout    time.Duration     `yaml:"timeout,omitempty"`
	Auth       GraylogAuthConfig `yaml:"auth,omitempty"`
	Guardrails GraylogGuardrails `yaml:"guardrails,omitempty"`
}

type GraylogAuthConfig struct {
	Type     string `yaml:"type,omitempty"`     // "token", "basic", "none"
	Token    string `yaml:"token,omitempty"`    // for token auth
	Username string `yaml:"username,omitempty"` // for basic auth
	Password string `yaml:"password,omitempty"` // for basic auth
}

type GraylogGuardrails struct {
	MaxQueryRange       time.Duration `yaml:"max_query_range,omitempty"`
	MaxResults          int           `yaml:"max_results,omitempty"`
	MaxHistogramBuckets int           `yaml:"max_histogram_buckets,omitempty"`
	QueryTimeout        time.Duration `yaml:"query_timeout,omitempty"`
}

type Capabilities struct {
	Enabled []string `yaml:"enabled"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config %q: %w", path, err)
	}

	if err := cfg.Metadata.AutodetectDefaults(); err != nil {
		return nil, fmt.Errorf("autodetect metadata defaults: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("config is nil")
	}
	if c.AgentID == "" {
		return fmt.Errorf("agent_id is required")
	}
	if c.ControlPlane.Address == "" {
		return fmt.Errorf("control_plane.address is required")
	}
	if err := c.ControlPlane.TLS.Validate(); err != nil {
		return fmt.Errorf("validate control_plane.tls: %w", err)
	}
	if err := c.Authentication.Validate(); err != nil {
		return fmt.Errorf("validate authentication: %w", err)
	}
	if c.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is required")
	}
	for _, enabled := range c.Capabilities.Enabled {
		if capability.IsPattern(enabled) {
			if err := capability.ValidatePattern(enabled); err != nil {
				return fmt.Errorf("validate capabilities.enabled pattern %q: %w", enabled, err)
			}
		} else if err := capability.ValidateName(enabled); err != nil {
			return fmt.Errorf("validate capabilities.enabled entry %q: %w", enabled, err)
		}
	}
	if err := log.ValidateLevel(c.Logging.Level); err != nil {
		return fmt.Errorf("validate logging.level: %w", err)
	}

	return nil
}

func (c *Config) ValidateProviderConfigs() []error {
	var warnings []error
	if c.hasAzureCapabilityEnabled() {
		if err := c.Azure.Validate(); err != nil {
			warnings = append(warnings, fmt.Errorf("azure config invalid, capabilities will be unavailable: %w", err))
		}
	}
	if c.hasKubernetesCapabilityEnabled() {
		if err := c.Kubernetes.Validate(); err != nil {
			warnings = append(warnings, fmt.Errorf("kubernetes config invalid, capabilities will be unavailable: %w", err))
		}
	}
	if c.hasPrometheusCapabilityEnabled() {
		if err := c.Prometheus.Validate(); err != nil {
			warnings = append(warnings, fmt.Errorf("prometheus config invalid, capabilities will be unavailable: %w", err))
		}
	}
	if c.hasLinuxCapabilityEnabled() {
		if err := c.Linux.Validate(); err != nil {
			warnings = append(warnings, fmt.Errorf("linux config invalid, capabilities will be unavailable: %w", err))
		}
	}
	if c.hasKafkaCapabilityEnabled() {
		if err := c.Kafka.Validate(); err != nil {
			warnings = append(warnings, fmt.Errorf("kafka config invalid, capabilities will be unavailable: %w", err))
		}
	}
	if c.hasGraylogCapabilityEnabled() {
		if err := c.Graylog.Validate(); err != nil {
			warnings = append(warnings, fmt.Errorf("graylog config invalid, capabilities will be unavailable: %w", err))
		}
	}
	return warnings
}

func (a Authentication) Validate() error {
	count := 0
	if a.SecretKey != nil {
		count++
		if a.SecretKey.Key == "" {
			return fmt.Errorf("authentication.secret_key.key is required")
		}
	}
	if a.MTLS != nil {
		count++
		if a.MTLS.CertFile == "" {
			return fmt.Errorf("authentication.mtls.cert_file is required")
		}
		if a.MTLS.KeyFile == "" {
			return fmt.Errorf("authentication.mtls.key_file is required")
		}
	}
	if count != 1 {
		return fmt.Errorf("exactly one authentication method must be configured")
	}

	return nil
}

func (k KubernetesConfig) Validate() error {
	if k.KubeconfigPath != "" {
		if _, err := os.Stat(k.KubeconfigPath); err != nil {
			return fmt.Errorf("kubernetes.kubeconfig_path: %w", err)
		}
	}
	return nil
}

func (c Config) hasKubernetesCapabilityEnabled() bool {
	for _, name := range c.Capabilities.Enabled {
		if capability.IsPattern(name) {
			matched, err := capability.Match("kubernetes.pod.list", name)
			if err == nil && matched {
				return true
			}
		}
		if strings.HasPrefix(name, "kubernetes.") {
			return true
		}
	}
	return false
}

func (a AzureConfig) Validate() error {
	if a.TenantID == "" {
		return fmt.Errorf("azure.tenant_id is required")
	}
	if a.ClientID == "" {
		return fmt.Errorf("azure.client_id is required")
	}
	if a.ClientSecret == "" {
		return fmt.Errorf("azure.client_secret is required")
	}
	if a.SubscriptionID == "" {
		return fmt.Errorf("azure.subscription_id is required")
	}
	return nil
}

func (c Config) hasAzureCapabilityEnabled() bool {
	for _, name := range c.Capabilities.Enabled {
		if capability.IsPattern(name) {
			matched, err := capability.Match("azure.network.nsg.list", name)
			if err == nil && matched {
				return true
			}
		}
		if strings.HasPrefix(name, "azure.") {
			return true
		}
	}
	return false
}

func (c Config) hasPrometheusCapabilityEnabled() bool {
	for _, name := range c.Capabilities.Enabled {
		if capability.IsPattern(name) {
			matched, err := capability.Match("prometheus.query.instant", name)
			if err == nil && matched {
				return true
			}
		}
		if strings.HasPrefix(name, "prometheus.") {
			return true
		}
	}
	return false
}

func (p PrometheusConfig) Validate() error {
	if p.URL == "" {
		return fmt.Errorf("prometheus.url is required")
	}
	if _, err := url.Parse(p.URL); err != nil {
		return fmt.Errorf("prometheus.url: %w", err)
	}
	switch p.Auth.Type {
	case "", "none":
		// ok
	case "basic":
		if p.Auth.Username == "" {
			return fmt.Errorf("prometheus.auth.username is required for basic auth")
		}
	case "bearer":
		if p.Auth.Token == "" {
			return fmt.Errorf("prometheus.auth.token is required for bearer auth")
		}
	case "header":
		if p.Auth.HeaderName == "" {
			return fmt.Errorf("prometheus.auth.header_name is required for header auth")
		}
	default:
		return fmt.Errorf("prometheus.auth.type: unsupported auth type %q", p.Auth.Type)
	}
	return nil
}

func (c Config) hasLinuxCapabilityEnabled() bool {
	for _, name := range c.Capabilities.Enabled {
		if capability.IsPattern(name) {
			matched, err := capability.Match("linux.system.info", name)
			if err == nil && matched {
				return true
			}
		}
		if strings.HasPrefix(name, "linux.") {
			return true
		}
	}
	return false
}

func (l LinuxConfig) Validate() error {
	if l.MaxReadBytes < 0 {
		return fmt.Errorf("linux.max_read_bytes must be non-negative")
	}
	if l.MaxDirectoryDepth < 0 {
		return fmt.Errorf("linux.max_directory_depth must be non-negative")
	}
	if l.MaxDirectoryEntries < 0 {
		return fmt.Errorf("linux.max_directory_entries must be non-negative")
	}
	return nil
}

func (c Config) hasKafkaCapabilityEnabled() bool {
	for _, name := range c.Capabilities.Enabled {
		if capability.IsPattern(name) {
			matched, err := capability.Match("kafka.cluster.describe", name)
			if err == nil && matched {
				return true
			}
		}
		if strings.HasPrefix(name, "kafka.") {
			return true
		}
	}
	return false
}

func (k KafkaConfig) Validate() error {
	if len(k.BootstrapServers) == 0 {
		return fmt.Errorf("kafka.bootstrap_servers is required")
	}
	for _, s := range k.BootstrapServers {
		host, _, err := net.SplitHostPort(s)
		if err != nil || host == "" {
			return fmt.Errorf("kafka.bootstrap_servers: invalid address %q", s)
		}
	}
	if k.SASL.Mechanism != "" {
		valid := map[string]bool{
			"PLAIN": true, "SCRAM-SHA-256": true, "SCRAM-SHA-512": true, "GSSAPI": true,
		}
		if !valid[k.SASL.Mechanism] {
			return fmt.Errorf("kafka.sasl.mechanism: unsupported %q", k.SASL.Mechanism)
		}
		if k.SASL.Mechanism == "GSSAPI" {
			if k.SASL.KeytabPath == "" && k.SASL.KeytabData == "" {
				return fmt.Errorf("kafka.sasl.keytab_path or kafka.sasl.keytab_data is required for GSSAPI")
			}
			if k.SASL.KerberosServiceName == "" {
				return fmt.Errorf("kafka.sasl.kerberos_service_name is required for GSSAPI")
			}
			if k.SASL.KerberosRealm == "" {
				return fmt.Errorf("kafka.sasl.kerberos_realm is required for GSSAPI")
			}
		}
	}
	if k.TLS.Enabled {
		if k.TLS.CAFile == "" && k.TLS.CAData == "" && !k.TLS.Insecure {
			return fmt.Errorf("kafka.tls.ca_file or kafka.tls.ca_data is required when tls is enabled")
		}
		if (k.TLS.CertFile != "" || k.TLS.CertData != "" || k.TLS.KeyFile != "" || k.TLS.KeyData != "") &&
			((k.TLS.CertFile == "" && k.TLS.CertData == "") || (k.TLS.KeyFile == "" && k.TLS.KeyData == "")) {
			return fmt.Errorf("kafka.tls client certificate requires cert_file/cert_data and key_file/key_data")
		}
	}
	if k.SchemaRegistry.URL != "" {
		if _, err := url.Parse(k.SchemaRegistry.URL); err != nil {
			return fmt.Errorf("kafka.schema_registry.url: %w", err)
		}
	}
	if k.MDS.URL != "" {
		if _, err := url.Parse(k.MDS.URL); err != nil {
			return fmt.Errorf("kafka.mds.url: %w", err)
		}
	}
	return nil
}

func (c Config) hasGraylogCapabilityEnabled() bool {
	for _, name := range c.Capabilities.Enabled {
		if capability.IsPattern(name) {
			matched, err := capability.Match("log.graylog.search", name)
			if err == nil && matched {
				return true
			}
		}
		if strings.HasPrefix(name, "log.") {
			return true
		}
	}
	return false
}

func (g GraylogConfig) Validate() error {
	if g.URL == "" {
		return fmt.Errorf("graylog.url is required")
	}
	u, err := url.Parse(g.URL)
	if err != nil {
		return fmt.Errorf("graylog.url: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("graylog.url: scheme must be https, got %q", u.Scheme)
	}
	switch g.Auth.Type {
	case "", "none":
	case "token":
		if g.Auth.Token == "" {
			return fmt.Errorf("graylog.auth.token is required for token auth")
		}
	case "basic":
		if g.Auth.Username == "" {
			return fmt.Errorf("graylog.auth.username is required for basic auth")
		}
		if g.Auth.Password == "" {
			return fmt.Errorf("graylog.auth.password is required for basic auth")
		}
	default:
		return fmt.Errorf("graylog.auth.type: unsupported auth type %q", g.Auth.Type)
	}
	return nil
}

func (a Authentication) AuthMethod() (string, any, error) {
	if err := a.Validate(); err != nil {
		return "", nil, err
	}
	if a.SecretKey != nil {
		return "secret_key", *a.SecretKey, nil
	}
	if a.MTLS != nil {
		return "mtls", *a.MTLS, nil
	}

	return "", nil, fmt.Errorf("no authentication method configured")
}

func (c TLSConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return true
	}
	return *c.Enabled
}

func (c TLSConfig) Validate() error {
	if c.CACertFile == "" {
		return nil
	}
	if _, err := os.Stat(c.CACertFile); err != nil {
		return fmt.Errorf("stat ca cert file %q: %w", c.CACertFile, err)
	}

	return nil
}

func (c TLSConfig) BuildTLSConfig() (*tls.Config, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: c.InsecureSkipVerify}
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("load system cert pool: %w", err)
	}
	if pool == nil {
		pool = x509.NewCertPool()
	}

	if c.CACertFile != "" {
		pemData, err := os.ReadFile(c.CACertFile)
		if err != nil {
			return nil, fmt.Errorf("read ca cert file %q: %w", c.CACertFile, err)
		}
		if ok := pool.AppendCertsFromPEM(pemData); !ok {
			return nil, fmt.Errorf("append ca cert file %q: invalid pem data", c.CACertFile)
		}
	}

	tlsCfg.RootCAs = pool
	return tlsCfg, nil
}
