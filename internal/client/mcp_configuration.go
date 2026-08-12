package client

import "context"

const mcpConfigurationsPath = appIntegrationsPrefix + "/resources/app-mcp-configurations/v1"

// McpConfigurationTypeSaas is the only configuration type this provider manages today.
const McpConfigurationTypeSaas = "saas"

// McpConfiguration is the per-application MCP gateway configuration.
type McpConfiguration struct {
	ID                       string            `json:"id"`
	VendorID                 string            `json:"vendorId"`
	AppID                    string            `json:"appId"`
	BaseURL                  string            `json:"baseUrl"`
	APITimeout               int64             `json:"apiTimeout"`
	Type                     string            `json:"type"`
	ExternalAuthorizationURL *string           `json:"externalAuthorizationUrl"`
	EnableAdvancedTools      bool              `json:"enableAdvancedTools"`
	SlimSemanticSearchOn     bool              `json:"slimSemanticSearchEnabled"`
	IntegrationToolsEnabled  bool              `json:"integrationToolsEnabled"`
	BehaviorRiskThreshold    string            `json:"behaviorRiskThreshold"`
	BehaviorRiskActions      map[string]string `json:"behaviorRiskActions"`
	ListToolPageSize         *int64            `json:"listToolPageSize"`
}

// McpConfigurationRequest creates or updates the configuration for one application.
type McpConfigurationRequest struct {
	AppID                    string            `json:"appId"`
	BaseURL                  string            `json:"baseUrl"`
	APITimeout               int64             `json:"apiTimeout"`
	Type                     string            `json:"type,omitempty"`
	ExternalAuthorizationURL *string           `json:"externalAuthorizationUrl,omitempty"`
	EnableAdvancedTools      *bool             `json:"enableAdvancedTools,omitempty"`
	SlimSemanticSearchOn     *bool             `json:"slimSemanticSearchEnabled,omitempty"`
	IntegrationToolsEnabled  *bool             `json:"integrationToolsEnabled,omitempty"`
	BehaviorRiskThreshold    string            `json:"behaviorRiskThreshold,omitempty"`
	BehaviorRiskActions      map[string]string `json:"behaviorRiskActions,omitempty"`
	ListToolPageSize         *int64            `json:"listToolPageSize,omitempty"`
}

// UpsertMcpConfiguration creates the configuration on first call and updates it afterwards.
func (c *Client) UpsertMcpConfiguration(ctx context.Context, req McpConfigurationRequest) (*McpConfiguration, error) {
	var configuration McpConfiguration
	if err := c.post(ctx, mcpConfigurationsPath, req, &configuration); err != nil {
		return nil, err
	}
	return &configuration, nil
}

// GetMcpConfiguration returns the configuration for an application, or nil when none exists.
func (c *Client) GetMcpConfiguration(ctx context.Context, appID string) (*McpConfiguration, error) {
	var configuration McpConfiguration
	err := c.get(ctx, withQuery(mcpConfigurationsPath, map[string]string{"appId": appID}), &configuration)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if configuration.ID == "" {
		return nil, nil
	}
	return &configuration, nil
}
