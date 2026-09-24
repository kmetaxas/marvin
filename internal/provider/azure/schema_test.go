package azure

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
)

func TestAzureTaskSchemasAreValid(t *testing.T) {
	t.Parallel()

	for name, schemaText := range map[string]string{
		"nsg":             nsgSchema,
		"udr":             udrSchema,
		"lb_rules":        lbRulesSchema,
		"lb_health":       lbHealthSchema,
		"appgw_backend":   appGWBackendSchema,
		"appgw_listeners": appGWListenersSchema,
		"logs_appgw":      appGWLogsSchema,
		"logs_vnet":       vnetFlowLogsSchema,
		"activity":        activitySchema,
	} {
		schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader([]byte(schemaText)))
		require.NoError(t, err, name)
		require.NotNil(t, schema, name)
	}
}

func TestAzureTaskSchemasValidateExamples(t *testing.T) {
	t.Parallel()

	tests := []struct {
		schema  string
		valid   map[string]any
		invalid map[string]any
	}{
		{nsgSchema, map[string]any{"resource_group": "rg1", "include_rules": true}, map[string]any{"include_rules": "yes"}},
		{udrSchema, map[string]any{"resource_group": "rg1", "include_routes": true}, map[string]any{"include_routes": "yes"}},
		{lbRulesSchema, map[string]any{"resource_group": "rg1", "load_balancer_name": "lb1"}, map[string]any{"resource_group": "rg1"}},
		{lbHealthSchema, map[string]any{"resource_group": "rg1", "load_balancer_name": "lb1"}, map[string]any{"load_balancer_name": "lb1"}},
		{appGWBackendSchema, map[string]any{"resource_group": "rg1", "app_gateway_name": "agw1"}, map[string]any{"resource_group": "rg1"}},
		{appGWListenersSchema, map[string]any{"resource_group": "rg1", "app_gateway_name": "agw1"}, map[string]any{"app_gateway_name": "agw1"}},
		{appGWLogsSchema, map[string]any{"workspace_id": "ws", "timespan": "PT1H", "max_rows": 10}, map[string]any{"max_rows": 0}},
		{vnetFlowLogsSchema, map[string]any{"workspace_id": "ws", "source_ip": "10.0.0.1"}, map[string]any{"max_rows": 1001.5}},
		{activitySchema, map[string]any{"timespan": "1h", "max_rows": 10}, map[string]any{"max_rows": 0}},
	}

	for _, tt := range tests {
		schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader([]byte(tt.schema)))
		require.NoError(t, err)

		validRes, err := schema.Validate(gojsonschema.NewGoLoader(tt.valid))
		require.NoError(t, err)
		require.True(t, validRes.Valid(), validRes.Errors())

		invalidRes, err := schema.Validate(gojsonschema.NewGoLoader(tt.invalid))
		require.NoError(t, err)
		require.False(t, invalidRes.Valid())
	}
}
