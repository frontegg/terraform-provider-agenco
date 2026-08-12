package client

import (
	"context"
	"fmt"
)

const promptsPath = appIntegrationsPrefix + "/resources/prompts/v1"

// Prompt is a reusable prompt exposed to agents alongside an application's tools.
type Prompt struct {
	ID          string   `json:"id"`
	AppID       string   `json:"appId"`
	Name        string   `json:"name"`
	Prompt      string   `json:"prompt"`
	Connections []string `json:"connections"`
}

// CreatePromptRequest creates a prompt against an application.
type CreatePromptRequest struct {
	AppID   string   `json:"appId"`
	Name    string   `json:"name"`
	Prompt  string   `json:"prompt"`
	ToolIDs []string `json:"toolIds,omitempty"`
}

// UpdatePromptRequest patches a prompt. The appId travels in the path, not the body.
type UpdatePromptRequest struct {
	Name    *string  `json:"name,omitempty"`
	Prompt  *string  `json:"prompt,omitempty"`
	ToolIDs []string `json:"toolIds,omitempty"`
}

func (c *Client) CreatePrompt(ctx context.Context, req CreatePromptRequest) (*Prompt, error) {
	var prompt Prompt
	if err := c.post(ctx, promptsPath, req, &prompt); err != nil {
		return nil, err
	}
	return &prompt, nil
}

func (c *Client) ListPrompts(ctx context.Context, appID string) ([]Prompt, error) {
	var prompts []Prompt
	if err := c.get(ctx, withQuery(promptsPath, map[string]string{"appId": appID}), &prompts); err != nil {
		return nil, err
	}
	return prompts, nil
}

// GetPrompt returns one prompt, or nil when it is gone. The API has no read-by-id route.
func (c *Client) GetPrompt(ctx context.Context, appID, promptID string) (*Prompt, error) {
	prompts, err := c.ListPrompts(ctx, appID)
	if err != nil {
		return nil, err
	}
	for i := range prompts {
		if prompts[i].ID == promptID {
			return &prompts[i], nil
		}
	}
	return nil, nil
}

func (c *Client) UpdatePrompt(ctx context.Context, appID, promptID string, req UpdatePromptRequest) error {
	return c.patch(ctx, fmt.Sprintf("%s/%s/%s", promptsPath, appID, promptID), req, nil)
}

func (c *Client) DeletePrompt(ctx context.Context, appID, promptID string) error {
	return c.delete(ctx, fmt.Sprintf("%s/%s/%s", promptsPath, appID, promptID))
}
