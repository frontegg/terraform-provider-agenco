package client

import (
	"context"
	"fmt"
)

const agentRegistryPath = appIntegrationsPrefix + "/resources/agent-registry/v1"

// Agent is a registered agent identity with its issued credentials and trust posture.
type Agent struct {
	ID                   string   `json:"id"`
	VendorID             string   `json:"vendorId"`
	AppID                string   `json:"appId"`
	AgentName            string   `json:"agentName"`
	AgentClass           string   `json:"agentClass"`
	AgentSource          string   `json:"agentSource"`
	Tags                 []string `json:"tags"`
	OwnerEmail           string   `json:"ownerEmail,omitempty"`
	CredentialsID        string   `json:"credentialsId"`
	ClientSecret         string   `json:"clientSecret,omitempty"`
	Status               string   `json:"status"`
	DPopEnabled          bool     `json:"dPopEnabled"`
	MtlsEnabled          bool     `json:"mtlsEnabled"`
	IPVerified           bool     `json:"ipVerified"`
	IsCimdClient         bool     `json:"isCimdClient"`
	UserIdentityVerified bool     `json:"userIdentityVerified"`
	TrustScore           *float64 `json:"trustScore,omitempty"`
	IdentityTier         string   `json:"identityTier,omitempty"`
}

// CreateAgentRequest registers an agent. The API has no update route.
type CreateAgentRequest struct {
	AppID        string   `json:"appId"`
	AgentName    string   `json:"agentName"`
	AgentClass   string   `json:"agentClass"`
	RedirectURLs []string `json:"redirectURLs,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	OwnerEmail   *string  `json:"ownerEmail,omitempty"`
}

// CreateAutonomousAgentRequest registers an autonomous agent, which has no redirect URLs.
type CreateAutonomousAgentRequest struct {
	AppID       string   `json:"appId"`
	AgentName   string   `json:"agentName"`
	Description *string  `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	OwnerEmail  *string  `json:"ownerEmail,omitempty"`
}

func (c *Client) CreateAgent(ctx context.Context, req CreateAgentRequest) (*Agent, error) {
	var agent Agent
	if err := c.post(ctx, agentRegistryPath, req, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

func (c *Client) CreateAutonomousAgent(ctx context.Context, req CreateAutonomousAgentRequest) (*Agent, error) {
	var agent Agent
	if err := c.post(ctx, agentRegistryPath+"/autonomous", req, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// GetAgent returns one agent, or nil when it is gone. The route resolves from vendor context.
func (c *Client) GetAgent(ctx context.Context, agentID string) (*Agent, error) {
	var agent Agent
	err := c.get(ctx, fmt.Sprintf("%s/%s", agentRegistryPath, agentID), &agent)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if agent.ID == "" {
		return nil, nil
	}
	return &agent, nil
}

func (c *Client) ListAgents(ctx context.Context, appID string) ([]Agent, error) {
	var agents []Agent
	if err := c.get(ctx, withQuery(agentRegistryPath, map[string]string{"appId": appID}), &agents); err != nil {
		return nil, err
	}
	return agents, nil
}

func (c *Client) DeleteAgent(ctx context.Context, agentID string) error {
	return c.delete(ctx, fmt.Sprintf("%s/%s", agentRegistryPath, agentID))
}
