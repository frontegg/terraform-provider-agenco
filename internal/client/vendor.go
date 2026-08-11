package client

import "context"

const (
	vendorPath                = "/vendors"
	identityConfigurationPath = identityPrefix + "/resources/configurations/v1"
)

// VendorConfig is the vendor-level configuration, including the CORS allow-list.
type VendorConfig struct {
	ID             string   `json:"id"`
	Name           string   `json:"name,omitempty"`
	AllowedOrigins []string `json:"allowedOrigins"`
}

// IdentityConfiguration is the vendor's identity settings relevant to agent tokens.
type IdentityConfiguration struct {
	ID                     string `json:"id"`
	DefaultTokenExpiration int64  `json:"defaultTokenExpiration"`
}

func (c *Client) GetVendorConfig(ctx context.Context) (*VendorConfig, error) {
	var config VendorConfig
	if err := c.get(ctx, vendorPath, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *Client) UpdateAllowedOrigins(ctx context.Context, origins []string) (*VendorConfig, error) {
	if origins == nil {
		origins = []string{}
	}
	var config VendorConfig
	body := map[string]interface{}{"allowedOrigins": origins}
	if err := c.put(ctx, vendorPath, body, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetIdentityConfiguration reads the current configuration. The API exposes only an
// upsert route, so an empty body is posted to read back the stored state.
func (c *Client) GetIdentityConfiguration(ctx context.Context) (*IdentityConfiguration, error) {
	var config IdentityConfiguration
	if err := c.post(ctx, identityConfigurationPath, map[string]interface{}{}, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

func (c *Client) UpdateIdentityConfiguration(ctx context.Context, defaultTokenExpiration int64) (*IdentityConfiguration, error) {
	var config IdentityConfiguration
	body := map[string]interface{}{"defaultTokenExpiration": defaultTokenExpiration}
	if err := c.post(ctx, identityConfigurationPath, body, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
