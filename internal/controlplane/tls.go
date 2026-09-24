package controlplane

import (
	"crypto/tls"

	"github.com/marvin-agent/marvin/internal/config"
)

func BuildTLSConfig(cfg config.TLSConfig) (*tls.Config, error) {
	return cfg.BuildTLSConfig()
}
