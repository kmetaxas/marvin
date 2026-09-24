package network

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
)

func TestDNSJSONSchemaIsValid(t *testing.T) {
	t.Parallel()

	schemaLoader := gojsonschema.NewBytesLoader([]byte(dnsSchema))
	schema, err := gojsonschema.NewSchema(schemaLoader)
	require.NoError(t, err)
	require.NotNil(t, schema)
}

func TestPingJSONSchemaIsValid(t *testing.T) {
	t.Parallel()

	schemaLoader := gojsonschema.NewBytesLoader([]byte(pingSchema))
	schema, err := gojsonschema.NewSchema(schemaLoader)
	require.NoError(t, err)
	require.NotNil(t, schema)
}

func TestSocketJSONSchemaIsValid(t *testing.T) {
	t.Parallel()

	schemaLoader := gojsonschema.NewBytesLoader([]byte(socketSchema))
	schema, err := gojsonschema.NewSchema(schemaLoader)
	require.NoError(t, err)
	require.NotNil(t, schema)
}

func TestTLSJSONSchemaIsValid(t *testing.T) {
	t.Parallel()

	schemaLoader := gojsonschema.NewBytesLoader([]byte(tlsSchema))
	schema, err := gojsonschema.NewSchema(schemaLoader)
	require.NoError(t, err)
	require.NotNil(t, schema)
}

func TestDNSJSONSchemaAcceptsValidParams(t *testing.T) {
	t.Parallel()

	schemaLoader := gojsonschema.NewBytesLoader([]byte(dnsSchema))
	schema, err := gojsonschema.NewSchema(schemaLoader)
	require.NoError(t, err)

	valid := []map[string]any{
		{"target": "example.com"},
		{"target": "example.com", "record_type": "A"},
		{"target": "example.com", "record_type": "AAAA", "server": "8.8.8.8:53"},
		{"target": "10.0.0.1", "record_type": "PTR", "protocol": "tcp"},
	}
	for _, params := range valid {
		loader := gojsonschema.NewGoLoader(params)
		res, err := schema.Validate(loader)
		require.NoError(t, err)
		require.True(t, res.Valid(), "expected valid for params %v, errors: %v", params, res.Errors())
	}

	invalid := []map[string]any{
		{}, // missing target
		{"target": "example.com", "record_type": "MX"}, // invalid enum
	}
	for _, params := range invalid {
		loader := gojsonschema.NewGoLoader(params)
		res, err := schema.Validate(loader)
		require.NoError(t, err)
		require.False(t, res.Valid(), "expected invalid for params %v", params)
	}
}

func TestPingJSONSchemaAcceptsValidParams(t *testing.T) {
	t.Parallel()

	schemaLoader := gojsonschema.NewBytesLoader([]byte(pingSchema))
	schema, err := gojsonschema.NewSchema(schemaLoader)
	require.NoError(t, err)

	valid := map[string]any{"target": "example.com", "count": 10}
	res, err := schema.Validate(gojsonschema.NewGoLoader(valid))
	require.NoError(t, err)
	require.True(t, res.Valid())

	invalid := []map[string]any{
		{},                                      // missing target
		{"target": "example.com", "count": 0},   // below minimum
		{"target": "example.com", "count": 100}, // above maximum
		{"target": "example.com", "count": "four"}, // wrong type
	}
	for _, params := range invalid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.False(t, res.Valid(), "expected invalid for params %v", params)
	}
}

func TestSocketJSONSchemaAcceptsValidParams(t *testing.T) {
	t.Parallel()

	schemaLoader := gojsonschema.NewBytesLoader([]byte(socketSchema))
	schema, err := gojsonschema.NewSchema(schemaLoader)
	require.NoError(t, err)

	valid := []map[string]any{
		{"host": "example.com", "port": 443},
		{"host": "10.0.0.1", "port": 53, "protocol": "udp"},
		{"host": "db.internal", "port": 5432, "timeout": "10s"},
	}
	for _, params := range valid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.True(t, res.Valid(), "expected valid for params %v, errors: %v", params, res.Errors())
	}

	invalid := []map[string]any{
		{"host": "example.com"},                                  // missing port
		{"port": 443},                                            // missing host
		{"host": "example.com", "port": 0},                       // port too low
		{"host": "example.com", "port": 70000},                   // port too high
		{"host": "example.com", "port": 443, "protocol": "sctp"}, // invalid enum
	}
	for _, params := range invalid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.False(t, res.Valid(), "expected invalid for params %v", params)
	}
}

func TestTLSJSONSchemaAcceptsValidParams(t *testing.T) {
	t.Parallel()

	schemaLoader := gojsonschema.NewBytesLoader([]byte(tlsSchema))
	schema, err := gojsonschema.NewSchema(schemaLoader)
	require.NoError(t, err)

	valid := []map[string]any{
		{"target": "example.com:443"},
		{"target": "10.0.0.1:443", "insecure_skip_verify": true},
		{"target": "example.com:443", "server_name": "example.com", "ca_cert": "-----BEGIN CERTIFICATE-----\nMIID..."},
		{"target": "mtls.example.com:443", "client_cert": "-----BEGIN CERTIFICATE-----\n...", "client_key": "-----BEGIN PRIVATE KEY-----\n..."},
	}
	for _, params := range valid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.True(t, res.Valid(), "expected valid for params %v, errors: %v", params, res.Errors())
	}

	invalid := []map[string]any{
		{},              // missing target
		{"target": 123}, // wrong type
		{"target": "example.com:443", "insecure_skip_verify": "yes"}, // wrong type
	}
	for _, params := range invalid {
		res, err := schema.Validate(gojsonschema.NewGoLoader(params))
		require.NoError(t, err)
		require.False(t, res.Valid(), "expected invalid for params %v", params)
	}
}
