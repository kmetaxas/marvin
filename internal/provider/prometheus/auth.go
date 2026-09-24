package prometheus

import (
	"fmt"
	"net/http"

	"github.com/marvin-agent/marvin/internal/config"
	promconfig "github.com/prometheus/common/config"
)

func buildAuthRoundTripper(auth config.PrometheusAuthConfig, base http.RoundTripper) (http.RoundTripper, error) {
	if base == nil {
		base = http.DefaultTransport
	}
	switch auth.Type {
	case "", "none":
		return base, nil
	case "basic":
		return promconfig.NewBasicAuthRoundTripper(promconfig.NewInlineSecret(auth.Username), promconfig.NewInlineSecret(auth.Password), base), nil
	case "bearer":
		return promconfig.NewAuthorizationCredentialsRoundTripper("Bearer", promconfig.NewInlineSecret(auth.Token), base), nil
	case "header":
		return promconfig.NewHeadersRoundTripper(&promconfig.Headers{
			Headers: map[string]promconfig.Header{
				auth.HeaderName: {Values: []string{auth.HeaderValue}},
			},
		}, base), nil
	default:
		return nil, fmt.Errorf("unsupported auth type: %s", auth.Type)
	}
}
