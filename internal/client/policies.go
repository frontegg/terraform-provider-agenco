package client

import (
	"context"
	"fmt"
)

const policiesPath = appIntegrationsPrefix + "/resources/policies/v1"

// PolicyCondition is a leaf condition. Value shape depends on Op, so it stays opaque JSON.
type PolicyCondition struct {
	Attribute string                 `json:"attribute"`
	Negate    bool                   `json:"negate"`
	Op        string                 `json:"op"`
	Value     map[string]interface{} `json:"value"`
}

// PolicyConditionGroup nests conditions under their own logic operator.
type PolicyConditionGroup struct {
	ConditionLogic string            `json:"conditionLogic"`
	Conditions     []PolicyCondition `json:"conditions"`
}

// PolicyIfBlock holds the top-level conditions and any nested groups. Conditions is
// heterogeneous on the wire — each element is either a condition or a condition group.
type PolicyIfBlock struct {
	ConditionLogic string        `json:"conditionLogic,omitempty"`
	Conditions     []interface{} `json:"conditions"`
}

// PolicyThenBlock is the outcome applied when the conditions match.
type PolicyThenBlock struct {
	Result            string  `json:"result"`
	ApprovalFlowID    *string `json:"approvalFlowId,omitempty"`
	StepUpActionAsync *bool   `json:"stepUpActionAsync,omitempty"`
}

// PolicyTargeting is the if/then rule attached to a conditional or masking policy.
type PolicyTargeting struct {
	If   *PolicyIfBlock  `json:"if,omitempty"`
	Then PolicyThenBlock `json:"then"`
}

// Policy is the read shape shared by conditional, RBAC and masking policies.
type Policy struct {
	ID                     string                 `json:"id"`
	VendorID               string                 `json:"vendorId"`
	TenantID               *string                `json:"tenantId"`
	Name                   string                 `json:"name"`
	Description            *string                `json:"description"`
	Type                   string                 `json:"type"`
	Slug                   *string                `json:"slug"`
	AppIDs                 []string               `json:"appIds"`
	Enabled                bool                   `json:"enabled"`
	InternalToolIDs        []string               `json:"internalToolIds"`
	CustomCodeToolIDs      []string               `json:"customCodeToolIds"`
	CustomIntegrationTools []string               `json:"customIntegrationTools"`
	AllowToAllUsers        bool                   `json:"allowToAllUsers"`
	Keys                   []string               `json:"keys"`
	Targeting              *PolicyTargeting       `json:"targeting"`
	PolicyConfiguration    map[string]interface{} `json:"policyConfiguration"`
	Metadata               map[string]interface{} `json:"metadata"`
}

// ConditionalPolicyRequest creates or updates a conditional policy.
type ConditionalPolicyRequest struct {
	AppIDs                 []string               `json:"appIds,omitempty"`
	Name                   string                 `json:"name"`
	Description            *string                `json:"description,omitempty"`
	Enabled                bool                   `json:"enabled"`
	Slug                   *string                `json:"slug,omitempty"`
	TenantID               *string                `json:"tenantId,omitempty"`
	InternalToolIDs        []string               `json:"internalToolIds"`
	CustomCodeToolIDs      []string               `json:"customCodeToolIds,omitempty"`
	CustomIntegrationTools []string               `json:"customIntegrationTools,omitempty"`
	AllowToAllUsers        *bool                  `json:"allowToAllUsers,omitempty"`
	Targeting              *PolicyTargeting       `json:"targeting,omitempty"`
	Metadata               map[string]interface{} `json:"metadata,omitempty"`
}

// MaskingPolicyRequest creates or updates a masking policy.
type MaskingPolicyRequest struct {
	ConditionalPolicyRequest
	PolicyConfiguration map[string]interface{} `json:"policyConfiguration"`
}

// RbacPolicyRequest creates or updates an RBAC policy.
type RbacPolicyRequest struct {
	AppIDs                 []string `json:"appIds,omitempty"`
	Name                   string   `json:"name"`
	Description            *string  `json:"description,omitempty"`
	Enabled                bool     `json:"enabled"`
	Slug                   *string  `json:"slug,omitempty"`
	TenantID               *string  `json:"tenantId,omitempty"`
	Type                   string   `json:"type"`
	Keys                   []string `json:"keys"`
	InternalToolIDs        []string `json:"internalToolIds,omitempty"`
	CustomCodeToolIDs      []string `json:"customCodeToolIds,omitempty"`
	CustomIntegrationTools []string `json:"customIntegrationTools,omitempty"`
}

type createPolicyResponse struct {
	ID string `json:"id"`
}

// createPolicy posts to a policy endpoint and returns the new policy ID.
func (c *Client) createPolicy(ctx context.Context, path string, req interface{}) (string, error) {
	var created createPolicyResponse
	if err := c.post(ctx, path, req, &created); err != nil {
		return "", err
	}
	return created.ID, nil
}

// getPolicy reads a policy by ID, returning nil when it no longer exists.
func (c *Client) getPolicy(ctx context.Context, path string) (*Policy, error) {
	var policy Policy
	err := c.get(ctx, path, &policy)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if policy.ID == "" {
		return nil, nil
	}
	return &policy, nil
}

func (c *Client) CreateConditionalPolicy(ctx context.Context, req ConditionalPolicyRequest) (string, error) {
	return c.createPolicy(ctx, policiesPath, req)
}

func (c *Client) GetConditionalPolicy(ctx context.Context, id string) (*Policy, error) {
	return c.getPolicy(ctx, fmt.Sprintf("%s/%s", policiesPath, id))
}

func (c *Client) UpdateConditionalPolicy(ctx context.Context, id string, req ConditionalPolicyRequest) error {
	return c.patch(ctx, fmt.Sprintf("%s/%s", policiesPath, id), req, nil)
}

func (c *Client) CreateRbacPolicy(ctx context.Context, req RbacPolicyRequest) (string, error) {
	return c.createPolicy(ctx, policiesPath+"/rbac", req)
}

func (c *Client) GetRbacPolicy(ctx context.Context, id string) (*Policy, error) {
	return c.getPolicy(ctx, fmt.Sprintf("%s/rbac/%s", policiesPath, id))
}

func (c *Client) UpdateRbacPolicy(ctx context.Context, id string, req RbacPolicyRequest) error {
	return c.patch(ctx, fmt.Sprintf("%s/rbac/%s", policiesPath, id), req, nil)
}

func (c *Client) CreateMaskingPolicy(ctx context.Context, req MaskingPolicyRequest) (string, error) {
	return c.createPolicy(ctx, policiesPath+"/masking", req)
}

func (c *Client) GetMaskingPolicy(ctx context.Context, id string) (*Policy, error) {
	return c.getPolicy(ctx, fmt.Sprintf("%s/masking/%s", policiesPath, id))
}

func (c *Client) UpdateMaskingPolicy(ctx context.Context, id string, req MaskingPolicyRequest) error {
	return c.patch(ctx, fmt.Sprintf("%s/masking/%s", policiesPath, id), req, nil)
}

// DeletePolicy removes any policy type; the API deletes by ID on the base route.
func (c *Client) DeletePolicy(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("%s/%s", policiesPath, id))
}
