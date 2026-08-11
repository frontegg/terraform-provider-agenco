package client

import (
	"context"
	"fmt"
)

const customIntegrationsPath = appIntegrationsPrefix + "/resources/custom-integrations/v1"

// CustomIntegration is an installed connector instance built from an integration template.
type CustomIntegration struct {
	ID                    string          `json:"id"`
	VendorID              string          `json:"vendorId"`
	AppID                 string          `json:"appId"`
	IntegrationTemplateID string          `json:"integrationTemplateId"`
	Slug                  *string         `json:"slug"`
	EnabledAPIs           map[string]bool `json:"enabledApis"`
	Name                  string          `json:"name"`
	Description           *string         `json:"description"`
	IsActive              bool            `json:"isActive"`
	IsDevCredsEnabled     bool            `json:"isDevCredsEnabled"`
	AuthType              string          `json:"authType"`
}

// IntegrationTemplate is a connector blueprint offered by Frontegg.
type IntegrationTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category,omitempty"`
	LogoURL     string   `json:"logoUrl,omitempty"`
	APINames    []string `json:"apiNames,omitempty"`
}

// CustomIntegrationConfiguration carries the OAuth credentials for a connector instance.
type CustomIntegrationConfiguration struct {
	ClientID            string                 `json:"clientId"`
	ClientSecret        string                 `json:"clientSecret"`
	BaseURL             string                 `json:"baseUrl,omitempty"`
	CustomConfiguration map[string]interface{} `json:"customConfiguration,omitempty"`
}

// CreateCustomIntegrationRequest installs a connector instance.
type CreateCustomIntegrationRequest struct {
	AppID                 string                          `json:"appId"`
	IntegrationTemplateID string                          `json:"integrationTemplateId"`
	Name                  string                          `json:"name"`
	Slug                  *string                         `json:"slug,omitempty"`
	APINames              []string                        `json:"apiNames,omitempty"`
	Description           *string                         `json:"description,omitempty"`
	AuthType              string                          `json:"authType,omitempty"`
	Configuration         *CustomIntegrationConfiguration `json:"configuration,omitempty"`
	APIKey                *string                         `json:"apiKey,omitempty"`
	IsActive              *bool                           `json:"isActive,omitempty"`
	IsDevCredsEnabled     *bool                           `json:"isDevCredsEnabled,omitempty"`
}

// UpdateCustomIntegrationRequest patches a connector instance.
type UpdateCustomIntegrationRequest struct {
	AppID             string                 `json:"appId"`
	Name              *string                `json:"name,omitempty"`
	Description       *string                `json:"description,omitempty"`
	EnabledAPIs       map[string]bool        `json:"enabledApis,omitempty"`
	Configuration     map[string]interface{} `json:"configuration,omitempty"`
	IsActive          *bool                  `json:"isActive,omitempty"`
	IsDevCredsEnabled *bool                  `json:"isDevCredsEnabled,omitempty"`
	AuthType          *string                `json:"authType,omitempty"`
	APIKey            *string                `json:"apiKey,omitempty"`
}

func (c *Client) CreateCustomIntegration(ctx context.Context, req CreateCustomIntegrationRequest) (*CustomIntegration, error) {
	var integration CustomIntegration
	if err := c.post(ctx, customIntegrationsPath, req, &integration); err != nil {
		return nil, err
	}
	return &integration, nil
}

// GetCustomIntegration returns one connector instance, or nil when it is gone.
func (c *Client) GetCustomIntegration(ctx context.Context, appID, integrationID string) (*CustomIntegration, error) {
	var integration CustomIntegration
	path := withQuery(fmt.Sprintf("%s/%s", customIntegrationsPath, integrationID), map[string]string{"appId": appID})

	err := c.get(ctx, path, &integration)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if integration.ID == "" {
		return nil, nil
	}
	return &integration, nil
}

func (c *Client) UpdateCustomIntegration(ctx context.Context, integrationID string, req UpdateCustomIntegrationRequest) (*CustomIntegration, error) {
	var integration CustomIntegration
	path := fmt.Sprintf("%s/%s", customIntegrationsPath, integrationID)
	if err := c.patch(ctx, path, req, &integration); err != nil {
		return nil, err
	}
	return &integration, nil
}

func (c *Client) DeleteCustomIntegration(ctx context.Context, appID, integrationID string) error {
	path := withQuery(fmt.Sprintf("%s/%s", customIntegrationsPath, integrationID), map[string]string{"appId": appID})
	return c.delete(ctx, path)
}

func (c *Client) ListIntegrationTemplates(ctx context.Context) ([]IntegrationTemplate, error) {
	var templates []IntegrationTemplate
	if err := c.get(ctx, customIntegrationsPath+"/templates", &templates); err != nil {
		return nil, err
	}
	return templates, nil
}
