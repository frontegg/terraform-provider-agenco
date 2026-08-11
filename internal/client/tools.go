package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

const (
	internalToolsPath = appIntegrationsPrefix + "/resources/internal-tools/v1"
	toolsPageSize     = 50
)

// InternalTool is a single tool exposed through the MCP gateway.
type InternalTool struct {
	ID                 string                 `json:"id,omitempty"`
	VendorID           string                 `json:"vendorId,omitempty"`
	AppID              string                 `json:"appId,omitempty"`
	Name               string                 `json:"name"`
	Description        string                 `json:"description"`
	OriginalMethod     string                 `json:"originalMethod,omitempty"`
	OriginalPath       string                 `json:"originalPath,omitempty"`
	IsActive           bool                   `json:"isActive"`
	Schema             map[string]interface{} `json:"schema,omitempty"`
	AuthenticationType string                 `json:"authenticationType,omitempty"`
	SourceID           string                 `json:"sourceId,omitempty"`
	ToolType           string                 `json:"toolType,omitempty"`
}

// UpsertToolsRequest replaces the tool set for an application and tool type.
type UpsertToolsRequest struct {
	AppID    string         `json:"appId"`
	ToolType string         `json:"toolType,omitempty"`
	Tools    []InternalTool `json:"tools"`
}

// UpdateToolRequest patches the mutable fields of an already-imported tool.
type UpdateToolRequest struct {
	AppID              string                 `json:"appId"`
	Name               *string                `json:"name,omitempty"`
	Description        *string                `json:"description,omitempty"`
	IsActive           *bool                  `json:"isActive,omitempty"`
	OriginalMethod     *string                `json:"originalMethod,omitempty"`
	OriginalPath       *string                `json:"originalPath,omitempty"`
	AuthenticationType *string                `json:"authenticationType,omitempty"`
	SourceID           *string                `json:"sourceId,omitempty"`
	OutputSchema       map[string]interface{} `json:"outputSchema,omitempty"`
}

// ImportOpenAPISchema turns an OpenAPI document into tool definitions without persisting them.
func (c *Client) ImportOpenAPISchema(ctx context.Context, appID string, schema []byte, filename string) ([]InternalTool, error) {
	return c.importSchema(ctx, appID, schema, filename, "openapi", internalToolsPath+"/openapi/import")
}

// ImportGraphQLSchema turns a GraphQL schema into tool definitions without persisting them.
func (c *Client) ImportGraphQLSchema(ctx context.Context, appID string, schema []byte, filename string) ([]InternalTool, error) {
	return c.importSchema(ctx, appID, schema, filename, "graphql", internalToolsPath+"/graphql/import")
}

func (c *Client) importSchema(ctx context.Context, appID string, schema []byte, filename, fieldName, path string) ([]InternalTool, error) {
	token, err := c.accessTokenValue(ctx)
	if err != nil {
		return nil, err
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("appId", appID); err != nil {
		return nil, fmt.Errorf("write appId field: %w", err)
	}
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(schema); err != nil {
		return nil, fmt.Errorf("write schema content: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, &body)
	if err != nil {
		return nil, fmt.Errorf("build import request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute import request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	payload, _ := io.ReadAll(resp.Body)
	if !isSuccess(resp.StatusCode) {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Method:     http.MethodPost,
			Path:       path,
			Body:       string(payload),
			TraceID:    resp.Header.Get("frontegg-trace-id"),
		}
	}

	var tools []InternalTool
	if err := json.Unmarshal(payload, &tools); err != nil {
		return nil, fmt.Errorf("decode import response: %w", err)
	}

	tflog.Info(ctx, "imported schema", map[string]interface{}{"tools": len(tools), "path": path})
	return tools, nil
}

func (c *Client) UpsertTools(ctx context.Context, req UpsertToolsRequest) ([]InternalTool, error) {
	var tools []InternalTool
	if err := c.post(ctx, internalToolsPath+"/upsert", req, &tools); err != nil {
		return nil, err
	}
	return tools, nil
}

// ImportAndUpsertTools imports a schema and persists the resulting tools against a source.
func (c *Client) ImportAndUpsertTools(ctx context.Context, appID, sourceID, schemaType string, schema []byte, filename string) (int, error) {
	var tools []InternalTool
	var err error

	switch schemaType {
	case "openapi":
		tools, err = c.ImportOpenAPISchema(ctx, appID, schema, filename)
	case "graphql":
		tools, err = c.ImportGraphQLSchema(ctx, appID, schema, filename)
	default:
		return 0, fmt.Errorf("unsupported schema type %q", schemaType)
	}
	if err != nil {
		return 0, err
	}
	if len(tools) == 0 {
		return 0, nil
	}

	toolType := "REST"
	if schemaType == "graphql" {
		toolType = "GRAPHQL"
	}
	for i := range tools {
		tools[i].SourceID = sourceID
		tools[i].IsActive = true
		// The import response stamps each tool with its type, but upsert takes the type once at
		// the top level and overwrites the per-tool value. Clear it so the payload matches what
		// the dashboard sends.
		tools[i].ToolType = ""
	}

	upserted, err := c.UpsertTools(ctx, UpsertToolsRequest{AppID: appID, ToolType: toolType, Tools: tools})
	if err != nil {
		return 0, err
	}
	return len(upserted), nil
}

// ListTools walks every page of an application's tools, optionally narrowed to one source.
func (c *Client) ListTools(ctx context.Context, appID, sourceID string) ([]InternalTool, error) {
	var all []InternalTool

	for offset := 0; ; offset++ {
		path := withQuery(internalToolsPath, map[string]string{
			"appId":    appID,
			"sourceId": sourceID,
			"_limit":   strconv.Itoa(toolsPageSize),
			"_offset":  strconv.Itoa(offset),
		})

		var page struct {
			Items    []InternalTool `json:"items"`
			Metadata struct {
				TotalPages int `json:"totalPages"`
			} `json:"_metadata"`
		}
		if err := c.get(ctx, path, &page); err != nil {
			return nil, err
		}

		all = append(all, page.Items...)
		if len(page.Items) == 0 || offset+1 >= page.Metadata.TotalPages {
			return all, nil
		}
	}
}

// GetTool returns one tool by ID, or nil when it is gone.
func (c *Client) GetTool(ctx context.Context, appID, toolID string) (*InternalTool, error) {
	var tools []InternalTool
	path := withQuery(internalToolsPath+"/with-schema", map[string]string{"appId": appID, "toolIds": toolID})

	err := c.get(ctx, path, &tools)
	if IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for i := range tools {
		if tools[i].ID == toolID {
			return &tools[i], nil
		}
	}
	return nil, nil
}

func (c *Client) UpdateTool(ctx context.Context, toolID string, req UpdateToolRequest) error {
	return c.patch(ctx, fmt.Sprintf("%s/%s", internalToolsPath, toolID), req, nil)
}

func (c *Client) DeleteTool(ctx context.Context, appID, toolID string) error {
	path := withQuery(fmt.Sprintf("%s/%s", internalToolsPath, toolID), map[string]string{"appId": appID})
	return c.delete(ctx, path)
}

// DeleteToolsByType removes every tool of one type from an application.
func (c *Client) DeleteToolsByType(ctx context.Context, appID, toolType string) error {
	path := withQuery(internalToolsPath+"/type", map[string]string{"appId": appID, "toolType": toolType})
	return c.delete(ctx, path)
}
