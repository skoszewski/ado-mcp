package ado

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// ResourceID is the Microsoft Entra application ID of Azure DevOps, the resource its access
// tokens are issued for.
const ResourceID = "499b84ac-1321-427f-aa17-267ca6975798"

const tokenRefreshMargin = 5 * time.Minute

// Authorizer supplies the Authorization header value for Azure DevOps REST requests.
type Authorizer interface {
	// Authorization returns the complete Authorization header value.
	Authorization(ctx context.Context) (string, error)
	// Method describes the authentication method, for the startup banner.
	Method() string
}

// NewAuthorizer selects the authentication method from the environment read through getenv:
// AZURE_DEVOPS_PAT when set, otherwise the service principal client secret when
// AZURE_TENANT_ID, AZURE_CLIENT_ID and AZURE_CLIENT_SECRET are all set, otherwise the signed-in
// Azure CLI.
func NewAuthorizer(getenv func(string) string) (Authorizer, error) {
	if pat := getenv("AZURE_DEVOPS_PAT"); pat != "" {
		return patAuthorizer{header: "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+pat))}, nil
	}

	tenantID, clientID, clientSecret := getenv("AZURE_TENANT_ID"), getenv("AZURE_CLIENT_ID"), getenv("AZURE_CLIENT_SECRET")
	if tenantID != "" && clientID != "" && clientSecret != "" {
		credential, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
		if err != nil {
			return nil, fmt.Errorf("could not create the service principal credential: %w", err)
		}
		method := fmt.Sprintf("service principal client secret (client %s, tenant %s)", clientID, tenantID)
		return &tokenAuthorizer{method: method, credential: credential}, nil
	}

	return NewAzureCLIAuthorizer()
}

// NewAzureCLIAuthorizer returns an Authorizer for the identity signed in to the Azure CLI.
func NewAzureCLIAuthorizer() (Authorizer, error) {
	credential, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return nil, fmt.Errorf("could not create the Azure CLI credential: %w", err)
	}
	return &tokenAuthorizer{method: "Azure CLI", credential: credential}, nil
}

// patAuthorizer authenticates with a personal access token as HTTP Basic credentials with an
// empty user name.
type patAuthorizer struct {
	header string
}

func (a patAuthorizer) Authorization(context.Context) (string, error) {
	return a.header, nil
}

func (a patAuthorizer) Method() string {
	return "personal access token (AZURE_DEVOPS_PAT)"
}

// tokenAuthorizer authenticates with a Microsoft Entra bearer token from an Azure Identity
// credential, cached until five minutes before it expires.
type tokenAuthorizer struct {
	method     string
	credential azcore.TokenCredential
	mu         sync.Mutex
	token      azcore.AccessToken
}

func (a *tokenAuthorizer) Authorization(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.token.Token == "" || time.Now().After(a.token.ExpiresOn.Add(-tokenRefreshMargin)) {
		token, err := a.credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{ResourceID + "/.default"}})
		if err != nil {
			return "", fmt.Errorf("could not get an Azure DevOps access token with %s: %w", a.method, err)
		}
		a.token = token
	}
	return "Bearer " + a.token.Token, nil
}

func (a *tokenAuthorizer) Method() string {
	return a.method
}
