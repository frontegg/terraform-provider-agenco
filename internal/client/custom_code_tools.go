package client

import (
	"context"
	"fmt"
)

const customCodeToolsPath = appIntegrationsPrefix + "/resources/custom-code-tools/v1"

// CustomCodeTool is a tool backed by vendor-supplied code instead of an upstream API.
type CustomCodeTool struct {
	ID                    string                 `json:"id"`
	VendorID              string                 `json:"vendorId"`
	AppID                 string                 `json:"appId"`
	Name                  string                 `json:"name"`
	Description           string                 `json:"description"`
	CustomCodeID          string                 `json:"customCodeId"`
	IsActive              bool                   `json:"isActive"`
	AttachedIntegrationID *string                `json:"attachedIntegrationId"`
	Scopes                []string               `json:"scopes"`
	ToolType              string                 `json:"toolType"`
	AuthenticationType    string                 `json:"authenticationType"`
	Schema                map[string]interface{} `json:"schema"`
}

// CreateCustomCodeToolRequest registers a custom code tool. Runtime is create-only.
type CreateCustomCodeToolRequest struct {
	AppID                 string                 `json:"appId"`
	Name                  string                 `json:"name"`
	Description           *string                `json:"description,omitempty"`
	CodeContent           string                 `json:"codeContent"`
	Runtime               string                 `json:"runtime"`
	InputSchema           map[string]interface{} `json:"inputSchema"`
	AttachedIntegrationID *string                `json:"attachedIntegrationId,omitempty"`
	Scopes                []string               `json:"scopes,omitempty"`
}

// UpdateCustomCodeToolRequest patches a custom code tool.
type UpdateCustomCodeToolRequest struct {
	AppID                 string                 `json:"appId"`
	Name                  *string                `json:"name,omitempty"`
	Description           *string                `json:"description,omitempty"`
	CodeContent           *string                `json:"codeContent,omitempty"`
	InputSchema           map[string]interface{} `json:"inputSchema,omitempty"`
	IsActive              *bool                  `json:"isActive,omitempty"`
	AttachedIntegrationID *string                `json:"attachedIntegrationId,omitempty"`
	Scopes                []string               `json:"scopes,omitempty"`
}

func (c *Client) CreateCustomCodeTool(ctx context.Context, req CreateCustomCodeToolRequest) (*CustomCodeTool, error) {
	var tool CustomCodeTool
	if err := c.post(ctx, customCodeToolsPath, req, &tool); err != nil {
		return nil, err
	}
	return &tool, nil
}

// GetCustomCodeTool returns one custom code tool, or nil when it is gone.
func (c *Client) GetCustomCodeTool(ctx context.Context, appID, toolID string) (*CustomCodeTool, error) {
	var tool CustomCodeTool
	path := withQuery(fmt.Sprintf("%s/%s", customCodeToolsPath, toolID), map[string]string{"appId": appID})

	err := c.get(ctx, path, &tool)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if tool.ID == "" {
		return nil, nil
	}
	return &tool, nil
}

func (c *Client) UpdateCustomCodeTool(ctx context.Context, toolID string, req UpdateCustomCodeToolRequest) (*CustomCodeTool, error) {
	var tool CustomCodeTool
	if err := c.patch(ctx, fmt.Sprintf("%s/%s", customCodeToolsPath, toolID), req, &tool); err != nil {
		return nil, err
	}
	return &tool, nil
}

func (c *Client) DeleteCustomCodeTool(ctx context.Context, appID, toolID string) error {
	path := withQuery(fmt.Sprintf("%s/%s", customCodeToolsPath, toolID), map[string]string{"appId": appID})
	return c.delete(ctx, path)
}
