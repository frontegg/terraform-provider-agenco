package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---- Application ----

var _ datasource.DataSource = &ApplicationDataSource{}

func NewApplicationDataSource() datasource.DataSource {
	return &ApplicationDataSource{}
}

// ApplicationDataSource looks up an existing application by ID or name.
type ApplicationDataSource struct {
	client *client.Client
}

// ApplicationDataSourceModel is the Terraform state for an application lookup.
type ApplicationDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	VendorID      types.String `tfsdk:"vendor_id"`
	AppURL        types.String `tfsdk:"app_url"`
	LoginURL      types.String `tfsdk:"login_url"`
	LogoURL       types.String `tfsdk:"logo_url"`
	AccessType    types.String `tfsdk:"access_type"`
	IsDefault     types.Bool   `tfsdk:"is_default"`
	IsActive      types.Bool   `tfsdk:"is_active"`
	Type          types.String `tfsdk:"type"`
	FrontendStack types.String `tfsdk:"frontend_stack"`
	Description   types.String `tfsdk:"description"`
	AllowDcr      types.Bool   `tfsdk:"allow_dcr"`
	AppHost       types.String `tfsdk:"app_host"`
}

func (d *ApplicationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

func (d *ApplicationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Looks up an existing Frontegg application by ID or by name. Exactly one of the two is required.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Application ID. Set this or name.",
				Optional:    true,
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "Application name. Set this or id.",
				Optional:    true,
				Computed:    true,
			},
			"vendor_id":      schema.StringAttribute{Description: "Vendor that owns the application.", Computed: true},
			"app_url":        schema.StringAttribute{Description: "URL the application is served from.", Computed: true},
			"login_url":      schema.StringAttribute{Description: "Login URL.", Computed: true},
			"logo_url":       schema.StringAttribute{Description: "Logo URL.", Computed: true},
			"access_type":    schema.StringAttribute{Description: "Access type.", Computed: true},
			"is_default":     schema.BoolAttribute{Description: "Whether this is the default application.", Computed: true},
			"is_active":      schema.BoolAttribute{Description: "Whether the application is active.", Computed: true},
			"type":           schema.StringAttribute{Description: "Application type.", Computed: true},
			"frontend_stack": schema.StringAttribute{Description: "Frontend stack.", Computed: true},
			"description":    schema.StringAttribute{Description: "Application description.", Computed: true},
			"allow_dcr":      schema.BoolAttribute{Description: "Whether Dynamic Client Registration is allowed.", Computed: true},
			"app_host":       schema.StringAttribute{Description: "Host assigned by Frontegg.", Computed: true},
		},
	}
}

func (d *ApplicationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *ApplicationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ApplicationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !config.ID.IsNull() && config.ID.ValueString() != ""
	hasName := !config.Name.IsNull() && config.Name.ValueString() != ""

	if hasID == hasName {
		resp.Diagnostics.AddError(
			"Set exactly one of id or name",
			"Look an application up either by id or by name, not both and not neither.",
		)
		return
	}

	var application *client.Application
	var err error
	if hasID {
		application, err = d.client.GetApplication(ctx, config.ID.ValueString())
	} else {
		application, err = d.client.FindApplicationByName(ctx, config.Name.ValueString())
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to look up application", err.Error())
		return
	}
	if application == nil {
		resp.Diagnostics.AddError(
			"Application not found",
			fmt.Sprintf("No application matched id=%q name=%q.", config.ID.ValueString(), config.Name.ValueString()),
		)
		return
	}

	config.ID = types.StringValue(application.ID)
	config.Name = types.StringValue(application.Name)
	config.VendorID = types.StringValue(application.VendorID)
	config.AppURL = types.StringValue(application.AppURL)
	config.LoginURL = types.StringValue(application.LoginURL)
	config.LogoURL = types.StringValue(application.LogoURL)
	config.AccessType = types.StringValue(application.AccessType)
	config.IsDefault = types.BoolValue(application.IsDefault)
	config.IsActive = types.BoolValue(application.IsActive)
	config.Type = types.StringValue(application.Type)
	config.FrontendStack = types.StringValue(application.FrontendStack)
	config.Description = types.StringValue(application.Description)
	config.AllowDcr = types.BoolValue(application.AllowDcr)
	config.AppHost = types.StringValue(application.AppHost)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// ---- Tools ----

var _ datasource.DataSource = &ToolsDataSource{}

func NewToolsDataSource() datasource.DataSource {
	return &ToolsDataSource{}
}

// ToolsDataSource lists an application's tools so their IDs can be referenced.
type ToolsDataSource struct {
	client *client.Client
}

// ToolSummaryModel is one entry in the tools listing.
type ToolSummaryModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	IsActive    types.Bool   `tfsdk:"is_active"`
	SourceID    types.String `tfsdk:"source_id"`
	Method      types.String `tfsdk:"original_method"`
	Path        types.String `tfsdk:"original_path"`
}

// ToolsDataSourceModel is the Terraform state for a tools listing.
type ToolsDataSourceModel struct {
	ApplicationID types.String       `tfsdk:"application_id"`
	SourceID      types.String       `tfsdk:"source_id"`
	Tools         []ToolSummaryModel `tfsdk:"tools"`
}

func (d *ToolsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tools"
}

func (d *ToolsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the tools of an application, optionally narrowed to one source. Use it to feed " +
			"tool IDs into policies, prompts and agenco_tool.",
		Attributes: map[string]schema.Attribute{
			"application_id": schema.StringAttribute{
				Description: "Application whose tools are listed.",
				Required:    true,
			},
			"source_id": schema.StringAttribute{
				Description: "Restrict the listing to one MCP source.",
				Optional:    true,
			},
			"tools": schema.ListNestedAttribute{
				Description: "Matching tools.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":              schema.StringAttribute{Description: "Tool ID.", Computed: true},
						"name":            schema.StringAttribute{Description: "Tool name.", Computed: true},
						"description":     schema.StringAttribute{Description: "Tool description.", Computed: true},
						"is_active":       schema.BoolAttribute{Description: "Whether the tool is exposed.", Computed: true},
						"source_id":       schema.StringAttribute{Description: "Source the tool belongs to.", Computed: true},
						"original_method": schema.StringAttribute{Description: "Upstream HTTP method.", Computed: true},
						"original_path":   schema.StringAttribute{Description: "Upstream path.", Computed: true},
					},
				},
			},
		},
	}
}

func (d *ToolsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *ToolsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ToolsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tools, err := d.client.ListTools(ctx, config.ApplicationID.ValueString(), config.SourceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to list tools", err.Error())
		return
	}

	config.Tools = make([]ToolSummaryModel, 0, len(tools))
	for _, tool := range tools {
		config.Tools = append(config.Tools, ToolSummaryModel{
			ID:          types.StringValue(tool.ID),
			Name:        types.StringValue(tool.Name),
			Description: types.StringValue(tool.Description),
			IsActive:    types.BoolValue(tool.IsActive),
			SourceID:    types.StringValue(tool.SourceID),
			Method:      types.StringValue(tool.OriginalMethod),
			Path:        types.StringValue(tool.OriginalPath),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

// ---- Integration templates ----

var _ datasource.DataSource = &IntegrationTemplatesDataSource{}

func NewIntegrationTemplatesDataSource() datasource.DataSource {
	return &IntegrationTemplatesDataSource{}
}

// IntegrationTemplatesDataSource lists the connector templates Frontegg offers.
type IntegrationTemplatesDataSource struct {
	client *client.Client
}

// IntegrationTemplateModel is one entry in the templates listing.
type IntegrationTemplateModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Category    types.String `tfsdk:"category"`
	LogoURL     types.String `tfsdk:"logo_url"`
	APINames    types.List   `tfsdk:"api_names"`
}

// IntegrationTemplatesDataSourceModel is the Terraform state for a templates listing.
type IntegrationTemplatesDataSourceModel struct {
	Templates []IntegrationTemplateModel `tfsdk:"templates"`
}

func (d *IntegrationTemplatesDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_templates"
}

func (d *IntegrationTemplatesDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists the connector templates available to your vendor. Use the IDs with " +
			"agenco_custom_integration.",
		Attributes: map[string]schema.Attribute{
			"templates": schema.ListNestedAttribute{
				Description: "Available integration templates.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Description: "Template ID.", Computed: true},
						"name":        schema.StringAttribute{Description: "Template name.", Computed: true},
						"description": schema.StringAttribute{Description: "Template description.", Computed: true},
						"category":    schema.StringAttribute{Description: "Template category.", Computed: true},
						"logo_url":    schema.StringAttribute{Description: "Template logo URL.", Computed: true},
						"api_names": schema.ListAttribute{
							Description: "APIs the template exposes.",
							Computed:    true,
							ElementType: types.StringType,
						},
					},
				},
			},
		},
	}
}

func (d *IntegrationTemplatesDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (d *IntegrationTemplatesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config IntegrationTemplatesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	templates, err := d.client.ListIntegrationTemplates(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list integration templates", err.Error())
		return
	}

	config.Templates = make([]IntegrationTemplateModel, 0, len(templates))
	for _, template := range templates {
		config.Templates = append(config.Templates, IntegrationTemplateModel{
			ID:          types.StringValue(template.ID),
			Name:        types.StringValue(template.Name),
			Description: types.StringValue(template.Description),
			Category:    types.StringValue(template.Category),
			LogoURL:     types.StringValue(template.LogoURL),
			APINames:    stringListValue(ctx, orEmpty(template.APINames), &resp.Diagnostics),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
