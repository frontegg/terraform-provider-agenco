package client

import (
	"context"
	"fmt"
)

const sourcesPath = appIntegrationsPrefix + "/resources/app-mcp-configuration-sources/v1"

// OverrideHeader is a header injected into every outbound request from a source.
type OverrideHeader struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Source is one upstream an application's MCP gateway pulls tools from.
type Source struct {
	ID                       string           `json:"id"`
	VendorID                 string           `json:"vendorId"`
	AppID                    string           `json:"appId"`
	Name                     string           `json:"name"`
	Slug                     *string          `json:"slug"`
	SourceURL                string           `json:"sourceUrl"`
	Secret                   string           `json:"secret,omitempty"`
	APITimeout               int64            `json:"apiTimeout"`
	Enabled                  bool             `json:"enabled"`
	IsLocal                  bool             `json:"isLocal"`
	TwoStepCallback          bool             `json:"twoStepCallback"`
	OverrideHeaders          []OverrideHeader `json:"overrideHeaders,omitempty"`
	ExternalAuthorizationURL *string          `json:"externalAuthorizationUrl"`
	Scopes                   []string         `json:"scopes"`
	ClientID                 *string          `json:"clientId"`
	ClientSecret             *string          `json:"clientSecret"`
}

// SourceRequest creates or updates a source. Type and IsLocal are create-only on the API.
type SourceRequest struct {
	AppID                    string           `json:"appId"`
	Name                     string           `json:"name"`
	Type                     string           `json:"type"`
	SourceURL                string           `json:"sourceUrl"`
	APITimeout               int64            `json:"apiTimeout"`
	Enabled                  *bool            `json:"enabled,omitempty"`
	IsLocal                  *bool            `json:"isLocal,omitempty"`
	Slug                     *string          `json:"slug,omitempty"`
	TwoStepCallback          *bool            `json:"twoStepCallback,omitempty"`
	OverrideHeaders          []OverrideHeader `json:"overrideHeaders,omitempty"`
	ExternalAuthorizationURL *string          `json:"externalAuthorizationUrl,omitempty"`
	Scopes                   []string         `json:"scopes,omitempty"`
	ClientID                 *string          `json:"clientId,omitempty"`
	ClientSecret             *string          `json:"clientSecret,omitempty"`
}

func (c *Client) CreateSource(ctx context.Context, req SourceRequest) (*Source, error) {
	var source Source
	if err := c.post(ctx, sourcesPath, req, &source); err != nil {
		return nil, err
	}
	return &source, nil
}

func (c *Client) UpdateSource(ctx context.Context, sourceID string, req SourceRequest) (*Source, error) {
	var source Source
	if err := c.patch(ctx, fmt.Sprintf("%s/%s", sourcesPath, sourceID), req, &source); err != nil {
		return nil, err
	}
	return &source, nil
}

func (c *Client) ListSources(ctx context.Context, appID string) ([]Source, error) {
	var sources []Source
	if err := c.get(ctx, withQuery(sourcesPath, map[string]string{"appId": appID}), &sources); err != nil {
		return nil, err
	}
	return sources, nil
}

// GetSource returns one source, or nil when it is gone. The API has no read-by-id route.
func (c *Client) GetSource(ctx context.Context, appID, sourceID string) (*Source, error) {
	sources, err := c.ListSources(ctx, appID)
	if err != nil {
		return nil, err
	}
	for i := range sources {
		if sources[i].ID == sourceID {
			return &sources[i], nil
		}
	}
	return nil, nil
}

func (c *Client) DeleteSource(ctx context.Context, appID, sourceID string) error {
	path := withQuery(fmt.Sprintf("%s/%s", sourcesPath, sourceID), map[string]string{"appId": appID})
	return c.delete(ctx, path)
}

// SetSourceToolsActiveStatus enables or disables every tool imported from a source.
func (c *Client) SetSourceToolsActiveStatus(ctx context.Context, appID, sourceID string, isActive bool) (int64, error) {
	body := map[string]interface{}{"appId": appID, "isActive": isActive}
	var result struct {
		UpdatedCount int64 `json:"updatedCount"`
	}
	path := fmt.Sprintf("%s/%s/set-tools-active-status", sourcesPath, sourceID)
	if err := c.post(ctx, path, body, &result); err != nil {
		return 0, err
	}
	return result.UpdatedCount, nil
}

// ToggleFronteggTenantToolsSource enables or disables the built-in Frontegg tenant tools source.
func (c *Client) ToggleFronteggTenantToolsSource(ctx context.Context, appID string, isActive bool) (*Source, error) {
	body := map[string]interface{}{"appId": appID, "isActive": isActive}
	var source Source
	if err := c.post(ctx, sourcesPath+"/frontegg-tenant-tools-source", body, &source); err != nil {
		return nil, err
	}
	return &source, nil
}
