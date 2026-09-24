package azure

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/marvin-agent/marvin/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var clientInitMu sync.Mutex

type fakeCredential struct{}

func (fakeCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{}, nil
}

func TestCreateAzureCredentialUsesClientSecret(t *testing.T) {
	t.Parallel()
	clientInitMu.Lock()
	defer clientInitMu.Unlock()

	origSecret := newClientSecretCredential
	origDefault := newDefaultAzureCredential
	t.Cleanup(func() {
		newClientSecretCredential = origSecret
		newDefaultAzureCredential = origDefault
	})

	secretCalled := false
	defaultCalled := false
	newClientSecretCredential = func(tenantID, clientID, clientSecret string, _ *azidentity.ClientSecretCredentialOptions) (azcore.TokenCredential, error) {
		secretCalled = true
		assert.Equal(t, "tenant", tenantID)
		assert.Equal(t, "client", clientID)
		assert.Equal(t, "secret", clientSecret)
		return fakeCredential{}, nil
	}
	newDefaultAzureCredential = func(_ *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		defaultCalled = true
		return fakeCredential{}, nil
	}

	cred, err := createAzureCredential(config.AzureConfig{TenantID: "tenant", ClientID: "client", ClientSecret: "secret"})
	require.NoError(t, err)
	require.NotNil(t, cred)
	assert.True(t, secretCalled)
	assert.False(t, defaultCalled)
}

func TestCreateAzureCredentialFallsBackToDefault(t *testing.T) {
	t.Parallel()
	clientInitMu.Lock()
	defer clientInitMu.Unlock()

	origSecret := newClientSecretCredential
	origDefault := newDefaultAzureCredential
	t.Cleanup(func() {
		newClientSecretCredential = origSecret
		newDefaultAzureCredential = origDefault
	})

	defaultCalled := false
	newClientSecretCredential = func(_, _, _ string, _ *azidentity.ClientSecretCredentialOptions) (azcore.TokenCredential, error) {
		return nil, errors.New("should not be called")
	}
	newDefaultAzureCredential = func(_ *azidentity.DefaultAzureCredentialOptions) (azcore.TokenCredential, error) {
		defaultCalled = true
		return fakeCredential{}, nil
	}

	cred, err := createAzureCredential(config.AzureConfig{SubscriptionID: "sub"})
	require.NoError(t, err)
	require.NotNil(t, cred)
	assert.True(t, defaultCalled)
}

func TestNewAzureClientsCapturesInitializationError(t *testing.T) {
	t.Parallel()
	clientInitMu.Lock()
	defer clientInitMu.Unlock()

	origSecret := newClientSecretCredential
	origSecurityGroups := newSecurityGroupsClient
	t.Cleanup(func() {
		newClientSecretCredential = origSecret
		newSecurityGroupsClient = origSecurityGroups
	})

	newClientSecretCredential = func(_, _, _ string, _ *azidentity.ClientSecretCredentialOptions) (azcore.TokenCredential, error) {
		return fakeCredential{}, nil
	}
	newSecurityGroupsClient = func(string, azcore.TokenCredential) (*armnetwork.SecurityGroupsClient, error) {
		return nil, errors.New("boom")
	}

	clients := newAzureClients(config.AzureConfig{TenantID: "tenant", ClientID: "client", ClientSecret: "secret", SubscriptionID: "sub"})
	require.NotNil(t, clients)
	require.Error(t, clients.initErr)
	assert.Contains(t, clients.initErr.Error(), "create azure security groups client")
}
