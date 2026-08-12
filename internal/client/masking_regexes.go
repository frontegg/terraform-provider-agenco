package client

import (
	"context"
	"fmt"
)

const maskingRegexesPath = appIntegrationsPrefix + "/resources/custom-masking-regexes/v1"

// CustomMaskingRegex is a vendor-defined pattern masking policies can reference.
type CustomMaskingRegex struct {
	ID          string  `json:"id"`
	VendorID    string  `json:"vendorId"`
	AppID       string  `json:"appId"`
	Name        string  `json:"name"`
	Pattern     string  `json:"pattern"`
	Flags       string  `json:"flags"`
	Enabled     bool    `json:"enabled"`
	Description *string `json:"description"`
}

// CustomMaskingRegexRequest creates or updates a custom masking regex.
// appId is only accepted on create; updates carry it in the path.
type CustomMaskingRegexRequest struct {
	AppID       string  `json:"appId,omitempty"`
	Name        *string `json:"name,omitempty"`
	Pattern     *string `json:"pattern,omitempty"`
	Flags       *string `json:"flags,omitempty"`
	Enabled     *bool   `json:"enabled,omitempty"`
	Description *string `json:"description,omitempty"`
}

func (c *Client) CreateMaskingRegex(ctx context.Context, req CustomMaskingRegexRequest) (*CustomMaskingRegex, error) {
	var regex CustomMaskingRegex
	if err := c.post(ctx, maskingRegexesPath, req, &regex); err != nil {
		return nil, err
	}
	return &regex, nil
}

func (c *Client) ListMaskingRegexes(ctx context.Context, appID string) ([]CustomMaskingRegex, error) {
	var regexes []CustomMaskingRegex
	if err := c.get(ctx, withQuery(maskingRegexesPath, map[string]string{"appId": appID}), &regexes); err != nil {
		return nil, err
	}
	return regexes, nil
}

// GetMaskingRegex returns one regex, or nil when it is gone.
func (c *Client) GetMaskingRegex(ctx context.Context, appID, regexID string) (*CustomMaskingRegex, error) {
	regexes, err := c.ListMaskingRegexes(ctx, appID)
	if err != nil {
		return nil, err
	}
	for i := range regexes {
		if regexes[i].ID == regexID {
			return &regexes[i], nil
		}
	}
	return nil, nil
}

func (c *Client) UpdateMaskingRegex(ctx context.Context, appID, regexID string, req CustomMaskingRegexRequest) (*CustomMaskingRegex, error) {
	req.AppID = ""
	var regex CustomMaskingRegex
	path := fmt.Sprintf("%s/%s/%s", maskingRegexesPath, appID, regexID)
	if err := c.patch(ctx, path, req, &regex); err != nil {
		return nil, err
	}
	return &regex, nil
}

func (c *Client) DeleteMaskingRegex(ctx context.Context, appID, regexID string) error {
	return c.delete(ctx, fmt.Sprintf("%s/%s/%s", maskingRegexesPath, appID, regexID))
}
