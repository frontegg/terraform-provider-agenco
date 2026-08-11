package client

import (
	"context"
	"fmt"
)

const applicationsPath = applicationsPrefix + "/resources/applications/v1"

// Application is a Frontegg application, the anchor every Agenco resource hangs off.
type Application struct {
	ID            string `json:"id"`
	VendorID      string `json:"vendorId"`
	Name          string `json:"name"`
	AppURL        string `json:"appURL"`
	LoginURL      string `json:"loginURL"`
	LogoURL       string `json:"logoURL"`
	AccessType    string `json:"accessType"`
	IsDefault     bool   `json:"isDefault"`
	IsActive      bool   `json:"isActive"`
	Type          string `json:"type"`
	FrontendStack string `json:"frontendStack"`
	Description   string `json:"description"`
	AppHost       string `json:"appHost,omitempty"`
	AllowDcr      bool   `json:"allowDcr"`
	AllowCimd     bool   `json:"allowCimd"`
	DPoPEnforce   string `json:"dpopEnforcementType,omitempty"`
	CreatedAt     string `json:"createdAt,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty"`
}

// ApplicationRequest is the create and update payload for an application.
type ApplicationRequest struct {
	Name          string `json:"name"`
	AppURL        string `json:"appURL"`
	LoginURL      string `json:"loginURL"`
	LogoURL       string `json:"logoURL,omitempty"`
	AccessType    string `json:"accessType,omitempty"`
	IsDefault     *bool  `json:"isDefault,omitempty"`
	IsActive      *bool  `json:"isActive,omitempty"`
	Type          string `json:"type,omitempty"`
	FrontendStack string `json:"frontendStack,omitempty"`
	Description   string `json:"description,omitempty"`
	AllowDcr      *bool  `json:"allowDcr,omitempty"`
}

func (c *Client) ListApplications(ctx context.Context) ([]Application, error) {
	var applications []Application
	if err := c.get(ctx, applicationsPath, &applications); err != nil {
		return nil, err
	}
	return applications, nil
}

// GetApplication returns the application with the given ID, or nil when it no longer exists.
func (c *Client) GetApplication(ctx context.Context, id string) (*Application, error) {
	applications, err := c.ListApplications(ctx)
	if err != nil {
		return nil, err
	}
	for i := range applications {
		if applications[i].ID == id {
			return &applications[i], nil
		}
	}
	return nil, nil
}

// FindApplicationByName returns the application with the given name, or nil when there is no match.
func (c *Client) FindApplicationByName(ctx context.Context, name string) (*Application, error) {
	applications, err := c.ListApplications(ctx)
	if err != nil {
		return nil, err
	}
	for i := range applications {
		if applications[i].Name == name {
			return &applications[i], nil
		}
	}
	return nil, nil
}

func (c *Client) CreateApplication(ctx context.Context, req ApplicationRequest) (*Application, error) {
	var application Application
	if err := c.post(ctx, applicationsPath, req, &application); err != nil {
		return nil, err
	}
	return &application, nil
}

// UpdateApplication patches an application. The API returns no body, so the caller re-reads.
func (c *Client) UpdateApplication(ctx context.Context, id string, req ApplicationRequest) error {
	return c.patch(ctx, fmt.Sprintf("%s/%s", applicationsPath, id), req, nil)
}

func (c *Client) DeleteApplication(ctx context.Context, id string) error {
	return c.delete(ctx, fmt.Sprintf("%s/%s", applicationsPath, id))
}
