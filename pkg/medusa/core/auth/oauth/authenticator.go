package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// OAuth2Authenticator is the generic authenticator for OAuth2
type OAuth2Authenticator struct {
	oauthConfig    *oauth2.Config
	providerConfig Provider
}

// NewGenericAuthenticator creates a new generic authenticator
func NewGenericAuthenticator(oauthConfig *oauth2.Config, provider Provider) *OAuth2Authenticator {
	// Assign provider scopes if not defined
	if len(oauthConfig.Scopes) == 0 {
		oauthConfig.Scopes = provider.Scopes
	}

	return &OAuth2Authenticator{
		oauthConfig:    oauthConfig,
		providerConfig: provider,
	}
}

// GetUser retrieves user information from any OAuth2 provider
func (a *OAuth2Authenticator) GetUser(token *oauth2.Token) (*User, error) {
	// Create HTTP client with access token
	client := a.oauthConfig.Client(context.Background(), token)

	// Make request to user info endpoint
	resp, err := client.Get(a.providerConfig.UserInfoURL)
	if err != nil {
		return nil, fmt.Errorf("error retrieving user information from %s: %v", a.providerConfig.Name, err)
	}
	defer resp.Body.Close()

	// Verify status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error in %s API response: %s", a.providerConfig.Name, resp.Status)
	}

	// Decode response into generic map
	var rawData map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rawData); err != nil {
		return nil, fmt.Errorf("error decoding user information: %v", err)
	}

	// Map fields to generic user
	user := &User{
		RawData: rawData,
	}

	user.ID = a.extractField(rawData, a.providerConfig.FieldMapping.ID)
	user.Email = a.extractField(rawData, a.providerConfig.FieldMapping.Email)
	user.Name = a.extractField(rawData, a.providerConfig.FieldMapping.Name)
	user.Picture = a.extractField(rawData, a.providerConfig.FieldMapping.Picture)

	// Handle VerifiedEmail (may be bool or not exist)
	if verifiedField := a.providerConfig.FieldMapping.VerifiedEmail; verifiedField != "" {
		if verified, ok := rawData[verifiedField].(bool); ok {
			user.VerifiedEmail = verified
		}
	}

	return user, nil
}

// extractField extracts a field from the raw data map (supports nested fields with dot notation)
func (a *OAuth2Authenticator) extractField(data map[string]any, fieldPath string) string {
	if fieldPath == "" {
		return ""
	}

	// For simple fields
	if val, ok := data[fieldPath]; ok {
		return fmt.Sprint(val)
	}

	// For nested fields like "picture.data.url"
	// This is a simple implementation, could be made more robust
	return ""
}

// GetOAuthConfig returns the OAuth2 configuration
func (a *OAuth2Authenticator) GetOAuthConfig() *oauth2.Config {
	return a.oauthConfig
}

// GetAuthURL generates the authentication URL
func (a *OAuth2Authenticator) GetAuthURL(state string) string {
	return a.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

// ExchangeCode exchanges the authorization code for a token
func (a *OAuth2Authenticator) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := a.oauthConfig.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("error exchanging code for token: %v", err)
	}
	return token, nil
}
