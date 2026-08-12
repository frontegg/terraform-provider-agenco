package client

import "context"

const toolHooksPath = appIntegrationsPrefix + "/resources/internal-tool-hooks/v1"

// ToolHook is the pre-hook code run for an application on LIST_TOOLS or CALL_TOOL.
// One hook exists per application and hook type.
type ToolHook struct {
	ID              string   `json:"id"`
	HookType        string   `json:"hookType"`
	InternalToolIDs []string `json:"internalToolIds"`
	Prehook         struct {
		IsActive   bool   `json:"isActive"`
		FailMethod string `json:"failMethod"`
		Timeout    *int64 `json:"timeout"`
	} `json:"prehook"`
}

// CreateToolHookRequest registers a hook. Runtime is create-only on the API.
type CreateToolHookRequest struct {
	AppID           string   `json:"appId"`
	IsActive        bool     `json:"isActive"`
	HookType        string   `json:"hookType"`
	Code            string   `json:"code"`
	Runtime         string   `json:"runtime"`
	FailMethod      string   `json:"failMethod"`
	Timeout         *int64   `json:"timeout,omitempty"`
	InternalToolIDs []string `json:"internalToolIds,omitempty"`
}

// UpdateToolHookRequest patches the hook identified by appId plus hookType.
type UpdateToolHookRequest struct {
	AppID           string   `json:"appId"`
	HookType        string   `json:"hookType"`
	IsActive        *bool    `json:"isActive,omitempty"`
	Code            *string  `json:"code,omitempty"`
	FailMethod      *string  `json:"failMethod,omitempty"`
	Timeout         *int64   `json:"timeout,omitempty"`
	InternalToolIDs []string `json:"internalToolIds,omitempty"`
}

func (c *Client) CreateToolHook(ctx context.Context, req CreateToolHookRequest) (*ToolHook, error) {
	var hook ToolHook
	if err := c.post(ctx, toolHooksPath, req, &hook); err != nil {
		return nil, err
	}
	return &hook, nil
}

func (c *Client) UpdateToolHook(ctx context.Context, req UpdateToolHookRequest) (*ToolHook, error) {
	var hook ToolHook
	if err := c.patch(ctx, toolHooksPath, req, &hook); err != nil {
		return nil, err
	}
	return &hook, nil
}

func (c *Client) ListToolHooks(ctx context.Context, appID string) ([]ToolHook, error) {
	var hooks []ToolHook
	if err := c.get(ctx, withQuery(toolHooksPath, map[string]string{"appId": appID}), &hooks); err != nil {
		return nil, err
	}
	return hooks, nil
}

// GetToolHook returns the hook of one type for an application, or nil when none is set.
func (c *Client) GetToolHook(ctx context.Context, appID, hookType string) (*ToolHook, error) {
	hooks, err := c.ListToolHooks(ctx, appID)
	if err != nil {
		return nil, err
	}
	for i := range hooks {
		if hooks[i].HookType == hookType {
			return &hooks[i], nil
		}
	}
	return nil, nil
}

func (c *Client) DeleteToolHook(ctx context.Context, appID, hookType string) error {
	path := withQuery(toolHooksPath, map[string]string{"appId": appID, "hookType": hookType})
	return c.delete(ctx, path)
}
