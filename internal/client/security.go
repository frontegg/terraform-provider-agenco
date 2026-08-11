package client

import (
	"context"
	"fmt"
)

const securityPath = appIntegrationsPrefix + "/resources/security/v1"

// IPRestriction is a single allowed or blocked IP or CIDR range for an application.
type IPRestriction struct {
	ID          string  `json:"id"`
	VendorID    string  `json:"vendorId"`
	AppID       string  `json:"appId"`
	TenantID    *string `json:"tenantId"`
	Description *string `json:"description"`
	IPType      string  `json:"ipType"`
	IP          string  `json:"ip"`
}

// IPRestrictionConfig switches IP restrictions between allow-list and block-list mode.
type IPRestrictionConfig struct {
	ID       string `json:"id"`
	VendorID string `json:"vendorId"`
	AppID    string `json:"appId"`
	Strategy string `json:"strategy"`
	IsActive bool   `json:"isActive"`
}

// GeoFence is the set of countries an application accepts or rejects traffic from.
type GeoFence struct {
	ID        string   `json:"id"`
	VendorID  string   `json:"vendorId"`
	AppID     string   `json:"appId"`
	TenantID  *string  `json:"tenantId"`
	Countries []string `json:"countries"`
}

// GeoFenceConfig switches geo-fencing between allow-list and block-list mode.
type GeoFenceConfig struct {
	ID       string `json:"id"`
	VendorID string `json:"vendorId"`
	AppID    string `json:"appId"`
	Strategy string `json:"strategy"`
	IsActive bool   `json:"isActive"`
}

// TimeOfWorkWindow is one contiguous working-hours window on a set of weekdays.
type TimeOfWorkWindow struct {
	WorkingDays       []int64 `json:"workingDays"`
	StartWorkingHours int64   `json:"startWorkingHours"`
	EndWorkingHours   int64   `json:"endWorkingHours"`
}

// TimeOfWork restricts tool access to working hours. One per application.
type TimeOfWork struct {
	ID       string             `json:"id"`
	VendorID string             `json:"vendorId"`
	AppID    string             `json:"appId"`
	TenantID *string            `json:"tenantId"`
	Action   string             `json:"action"`
	IsActive bool               `json:"isActive"`
	Windows  []TimeOfWorkWindow `json:"windows"`
}

// RateLimitConfig holds per-minute request budgets keyed by source, tool, tenant and user.
type RateLimitConfig struct {
	ID       string           `json:"id"`
	VendorID string           `json:"vendorId"`
	AppID    string           `json:"appId"`
	Sources  map[string]int64 `json:"sources"`
	Tools    map[string]int64 `json:"tools"`
	Tenants  map[string]int64 `json:"tenants"`
	Users    map[string]int64 `json:"users"`
}

// AgentType gates which AI platforms may reach an application, and for whom.
type AgentType struct {
	ID          string   `json:"id"`
	VendorID    string   `json:"vendorId"`
	AppID       string   `json:"appId"`
	AgentType   string   `json:"agentType"`
	AllUsers    bool     `json:"allUsers"`
	IsActive    bool     `json:"isActive"`
	GroupIDs    []string `json:"groupIds"`
	Description *string  `json:"description,omitempty"`
}

// ---- IP restrictions ----

func (c *Client) CreateIPRestriction(ctx context.Context, appID, ip string, description *string) (*IPRestriction, error) {
	body := map[string]interface{}{"appId": appID, "ip": ip}
	if description != nil {
		body["description"] = *description
	}
	var restriction IPRestriction
	if err := c.post(ctx, securityPath+"/ip-restrictions", body, &restriction); err != nil {
		return nil, err
	}
	return &restriction, nil
}

func (c *Client) ListIPRestrictions(ctx context.Context, appID string) ([]IPRestriction, error) {
	var restrictions []IPRestriction
	path := withQuery(securityPath+"/ip-restrictions", map[string]string{"appId": appID})
	if err := c.get(ctx, path, &restrictions); err != nil {
		return nil, err
	}
	return restrictions, nil
}

// GetIPRestriction returns one restriction, or nil when it is gone.
func (c *Client) GetIPRestriction(ctx context.Context, appID, restrictionID string) (*IPRestriction, error) {
	restrictions, err := c.ListIPRestrictions(ctx, appID)
	if err != nil {
		return nil, err
	}
	for i := range restrictions {
		if restrictions[i].ID == restrictionID {
			return &restrictions[i], nil
		}
	}
	return nil, nil
}

// UpdateIPRestriction patches the description. The IP itself is immutable on the API.
func (c *Client) UpdateIPRestriction(ctx context.Context, appID, restrictionID string, description *string) (*IPRestriction, error) {
	body := map[string]interface{}{"appId": appID}
	if description != nil {
		body["description"] = *description
	}
	var restriction IPRestriction
	path := fmt.Sprintf("%s/ip-restrictions/%s", securityPath, restrictionID)
	if err := c.patch(ctx, path, body, &restriction); err != nil {
		return nil, err
	}
	return &restriction, nil
}

func (c *Client) DeleteIPRestriction(ctx context.Context, appID, restrictionID string) error {
	path := withQuery(fmt.Sprintf("%s/ip-restrictions/%s", securityPath, restrictionID), map[string]string{"appId": appID})
	return c.delete(ctx, path)
}

// GetIPRestrictionConfig reads the config, which the API creates on first read.
func (c *Client) GetIPRestrictionConfig(ctx context.Context, appID string) (*IPRestrictionConfig, error) {
	var config IPRestrictionConfig
	path := withQuery(securityPath+"/ip-restriction-config", map[string]string{"appId": appID})

	err := c.get(ctx, path, &config)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if config.ID == "" {
		return nil, nil
	}
	return &config, nil
}

func (c *Client) UpsertIPRestrictionConfig(ctx context.Context, appID, strategy string, isActive bool) (*IPRestrictionConfig, error) {
	body := map[string]interface{}{"appId": appID, "strategy": strategy, "isActive": isActive}
	var config IPRestrictionConfig
	if err := c.put(ctx, securityPath+"/ip-restriction-config", body, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *Client) DeleteIPRestrictionConfig(ctx context.Context, appID string) error {
	return c.delete(ctx, withQuery(securityPath+"/ip-restriction-config", map[string]string{"appId": appID}))
}

// ---- Geo-fencing ----

// GetGeoFence returns the application's geo-fence, or nil when none is set.
func (c *Client) GetGeoFence(ctx context.Context, appID string) (*GeoFence, error) {
	var fence GeoFence
	path := withQuery(securityPath+"/geo-fences", map[string]string{"appId": appID})

	err := c.get(ctx, path, &fence)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if fence.ID == "" {
		return nil, nil
	}
	return &fence, nil
}

func (c *Client) UpsertGeoFence(ctx context.Context, appID string, countries []string, description *string) (*GeoFence, error) {
	body := map[string]interface{}{"appId": appID, "countries": countries}
	if description != nil {
		body["description"] = *description
	}
	var fence GeoFence
	if err := c.post(ctx, securityPath+"/geo-fences", body, &fence); err != nil {
		return nil, err
	}
	return &fence, nil
}

func (c *Client) DeleteGeoFence(ctx context.Context, appID string) error {
	return c.delete(ctx, withQuery(securityPath+"/geo-fences", map[string]string{"appId": appID}))
}

// GetGeoFenceConfig reads the config, which the API creates on first read.
func (c *Client) GetGeoFenceConfig(ctx context.Context, appID string) (*GeoFenceConfig, error) {
	var config GeoFenceConfig
	path := withQuery(securityPath+"/geo-fence-config", map[string]string{"appId": appID})

	err := c.get(ctx, path, &config)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if config.ID == "" {
		return nil, nil
	}
	return &config, nil
}

func (c *Client) UpsertGeoFenceConfig(ctx context.Context, appID, strategy string, isActive bool) (*GeoFenceConfig, error) {
	body := map[string]interface{}{"appId": appID, "strategy": strategy, "isActive": isActive}
	var config GeoFenceConfig
	if err := c.put(ctx, securityPath+"/geo-fence-config", body, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *Client) DeleteGeoFenceConfig(ctx context.Context, appID string) error {
	return c.delete(ctx, withQuery(securityPath+"/geo-fence-config", map[string]string{"appId": appID}))
}

// ---- Time of work ----

// GetTimeOfWork returns the application's working-hours rule, or nil when none is set.
func (c *Client) GetTimeOfWork(ctx context.Context, appID string) (*TimeOfWork, error) {
	var timeOfWork TimeOfWork
	path := withQuery(securityPath+"/time-of-work", map[string]string{"appId": appID})

	err := c.get(ctx, path, &timeOfWork)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if timeOfWork.ID == "" {
		return nil, nil
	}
	return &timeOfWork, nil
}

func (c *Client) UpsertTimeOfWork(ctx context.Context, appID, action string, isActive bool, windows []TimeOfWorkWindow) (*TimeOfWork, error) {
	body := map[string]interface{}{
		"appId":    appID,
		"action":   action,
		"isActive": isActive,
		"windows":  windows,
	}
	var timeOfWork TimeOfWork
	if err := c.post(ctx, securityPath+"/time-of-work", body, &timeOfWork); err != nil {
		return nil, err
	}
	return &timeOfWork, nil
}

func (c *Client) DeleteTimeOfWork(ctx context.Context, appID string) error {
	return c.delete(ctx, withQuery(securityPath+"/time-of-work", map[string]string{"appId": appID}))
}

// ---- Rate limits ----

// GetRateLimitConfig returns the application's rate-limit budgets, or nil when none are set.
func (c *Client) GetRateLimitConfig(ctx context.Context, appID string) (*RateLimitConfig, error) {
	var config RateLimitConfig
	path := withQuery(securityPath+"/rate-limit-configs", map[string]string{"appId": appID})

	err := c.get(ctx, path, &config)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if config.ID == "" {
		return nil, nil
	}
	return &config, nil
}

// UpsertRateLimitConfig fully replaces the budgets; omitted maps are cleared by the API.
func (c *Client) UpsertRateLimitConfig(ctx context.Context, appID string, sources, tools, tenants, users map[string]int64) (*RateLimitConfig, error) {
	body := map[string]interface{}{"appId": appID}
	if sources != nil {
		body["sources"] = sources
	}
	if tools != nil {
		body["tools"] = tools
	}
	if tenants != nil {
		body["tenants"] = tenants
	}
	if users != nil {
		body["users"] = users
	}

	var config RateLimitConfig
	if err := c.put(ctx, securityPath+"/rate-limit-configs", body, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *Client) DeleteRateLimitConfig(ctx context.Context, appID string) error {
	return c.delete(ctx, withQuery(securityPath+"/rate-limit-configs", map[string]string{"appId": appID}))
}

// ---- Agent types ----

func (c *Client) UpsertAgentType(ctx context.Context, req AgentType) (*AgentType, error) {
	body := map[string]interface{}{
		"appId":     req.AppID,
		"agentType": req.AgentType,
		"allUsers":  req.AllUsers,
		"isActive":  req.IsActive,
		"groupIds":  req.GroupIDs,
	}
	if req.Description != nil {
		body["description"] = *req.Description
	}
	if req.GroupIDs == nil {
		body["groupIds"] = []string{}
	}

	var agentType AgentType
	if err := c.post(ctx, securityPath+"/agent-type", body, &agentType); err != nil {
		return nil, err
	}
	return &agentType, nil
}

func (c *Client) ListAgentTypes(ctx context.Context, appID string) ([]AgentType, error) {
	var agentTypes []AgentType
	path := withQuery(securityPath+"/agent-type", map[string]string{"appId": appID})
	if err := c.get(ctx, path, &agentTypes); err != nil {
		return nil, err
	}
	return agentTypes, nil
}

// GetAgentTypeByPlatform returns the rule for one AI platform, or nil when none is set.
func (c *Client) GetAgentTypeByPlatform(ctx context.Context, appID, platform string) (*AgentType, error) {
	agentTypes, err := c.ListAgentTypes(ctx, appID)
	if err != nil {
		return nil, err
	}
	for i := range agentTypes {
		if agentTypes[i].AgentType == platform {
			return &agentTypes[i], nil
		}
	}
	return nil, nil
}

func (c *Client) DeleteAgentType(ctx context.Context, agentTypeID string) error {
	return c.delete(ctx, fmt.Sprintf("%s/agent-type/%s", securityPath, agentTypeID))
}
