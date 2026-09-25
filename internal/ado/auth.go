package ado

import (
	"context"
	"encoding/base64"
	"errors"
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

// Authentication method names accepted by NewAuthorizer.
const (
	AuthAuto             = "auto"
	AuthPAT              = "pat"
	AuthServicePrincipal = "service-principal"
	AuthAzureCLI         = "azure-cli"
	AuthNone             = "none"
)

// NewAuthorizer returns the Authorizer for method, reading its settings through getenv.
//
// AuthAuto picks AZURE_DEVOPS_PAT when set, otherwise the service principal client secret when
// AZURE_TENANT_ID, AZURE_CLIENT_ID and AZURE_CLIENT_SECRET are all set, otherwise the signed-in
// Azure CLI. AuthPAT, AuthServicePrincipal and AuthAzureCLI force one method and fail when its
// settings are missing. AuthNone configures no credential, so every request needs one from
// WithAuthorization.
func NewAuthorizer(method string, getenv func(string) string) (Authorizer, error) {
	pat := getenv("AZURE_DEVOPS_PAT")
	tenantID, clientID, clientSecret := getenv("AZURE_TENANT_ID"), getenv("AZURE_CLIENT_ID"), getenv("AZURE_CLIENT_SECRET")
	servicePrincipalSet := tenantID != "" && clientID != "" && clientSecret != ""

	switch method {
	case AuthAuto:
		switch {
		case pat != "":
			return newPATAuthorizer(pat), nil
		case servicePrincipalSet:
			return newServicePrincipalAuthorizer(tenantID, clientID, clientSecret)
		}
		return NewAzureCLIAuthorizer()
	case AuthPAT:
		if pat == "" {
			return nil, errors.New("--auth pat needs AZURE_DEVOPS_PAT")
		}
		return newPATAuthorizer(pat), nil
	case AuthServicePrincipal:
		if !servicePrincipalSet {
			return nil, errors.New("--auth service-principal needs AZURE_TENANT_ID, AZURE_CLIENT_ID and AZURE_CLIENT_SECRET")
		}
		return newServicePrincipalAuthorizer(tenantID, clientID, clientSecret)
	case AuthAzureCLI:
		return NewAzureCLIAuthorizer()
	case AuthNone:
		return noneAuthorizer{}, nil
	}
	return nil, fmt.Errorf("unknown authentication method %q; use %s, %s, %s, %s or %s",
		method, AuthAuto, AuthPAT, AuthServicePrincipal, AuthAzureCLI, AuthNone)
}

func newPATAuthorizer(pat string) Authorizer {
	return patAuthorizer{header: "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+pat))}
}

func newServicePrincipalAuthorizer(tenantID, clientID, clientSecret string) (Authorizer, error) {
	credential, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create the service principal credential: %w", err)
	}
	method := fmt.Sprintf("service principal client secret (client %s, tenant %s)", clientID, tenantID)
	return &tokenAuthorizer{method: method, credential: credential}, nil
}

// NewAzureCLIAuthorizer returns an Authorizer for the identity signed in to the Azure CLI.
func NewAzureCLIAuthorizer() (Authorizer, error) {
	credential, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return nil, fmt.Errorf("could not create the Azure CLI credential: %w", err)
	}
	return &tokenAuthorizer{method: "Azure CLI", credential: credential}, nil
}

// noneAuthorizer holds no credential; a request succeeds only with one from WithAuthorization.
type noneAuthorizer struct{}

func (noneAuthorizer) Authorization(context.Context) (string, error) {
	return "", errors.New("no credential: the server is configured with --auth none, so the MCP client must send an Authorization header")
}

func (noneAuthorizer) Method() string {
	return "none (the MCP client sends an Authorization header)"
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
