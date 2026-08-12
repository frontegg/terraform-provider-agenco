package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &McpSourceResource{}
	_ resource.ResourceWithImportState = &McpSourceResource{}
)

// sourceTypes are the tool source types the API accepts on a configuration source.
var sourceTypes = []string{"REST", "GRAPHQL", "MOCK", "MCP_PROXY", "FRONTEGG", "CUSTOM_INTEGRATION", "CUSTOM_CODE"}

func NewMcpSourceResource() resource.Resource {
	return &McpSourceResource{}
}

// McpSourceResource manages one upstream source of an application's MCP gateway.
type McpSourceResource struct {
	client *client.Client
}

// OverrideHeaderModel is a single injected outbound header.
type OverrideHeaderModel struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

// McpSourceResourceModel is the Terraform state for an MCP source.
type McpSourceResourceModel struct {
	ID                       types.String          `tfsdk:"id"`
	ApplicationID            types.String          `tfsdk:"application_id"`
	Name                     types.String          `tfsdk:"name"`
	Type                     types.String          `tfsdk:"type"`
	SourceURL                types.String          `tfsdk:"source_url"`
	APITimeout               types.Int64           `tfsdk:"api_timeout"`
	Enabled                  types.Bool            `tfsdk:"enabled"`
	IsLocal                  types.Bool            `tfsdk:"is_local"`
	Slug                     types.String          `tfsdk:"slug"`
	TwoStepCallback          types.Bool            `tfsdk:"two_step_callback"`
	OverrideHeaders          []OverrideHeaderModel `tfsdk:"override_headers"`
	ExternalAuthorizationURL types.String          `tfsdk:"external_authorization_url"`
	Scopes                   types.List            `tfsdk:"scopes"`
	ClientID                 types.String          `tfsdk:"client_id"`
	ClientSecret             types.String          `tfsdk:"client_secret"`
	VendorID                 types.String          `tfsdk:"vendor_id"`
	Secret                   types.String          `tfsdk:"secret"`
}

func (r *McpSourceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_source"
}

func (r *McpSourceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages an MCP configuration source — one upstream an application pulls tools from.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Source ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application this source belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Source name, up to 36 characters.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(36),
				},
			},
			"type": schema.StringAttribute{
				Description: fmt.Sprintf("Source type. One of: %v. Immutable after create.", sourceTypes),
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(sourceTypes...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"source_url": schema.StringAttribute{
				Description: "Upstream URL. Must be HTTPS with a valid TLD unless is_local is true.",
				Required:    true,
			},
			"api_timeout": schema.Int64Attribute{
				Description: fmt.Sprintf("Upstream request timeout in milliseconds, between %d and %d. The API "+
					"requires this field, so the provider defaults it to %d for consistency with "+
					"agenco_mcp_configuration.", minAPITimeout, maxAPITimeout, defaultAPITimeout),
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					createDefaultInt64(defaultAPITimeout),
				},
				Validators: []validator.Int64{
					int64validator.Between(minAPITimeout, maxAPITimeout),
				},
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the source is enabled.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(true),
				},
			},
			"is_local": schema.BoolAttribute{
				Description: "Whether the source is local, relaxing the HTTPS and DNS checks on source_url. " +
					"Immutable after create.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(false),
					boolplanmodifier.RequiresReplace(),
				},
			},
			"slug": schema.StringAttribute{
				Description: "Instance discriminator for running more than one instance of the same source. " +
					"Lowercase kebab-case, up to 64 characters. Changing it re-labels the source's imported tools.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(64),
					stringvalidator.RegexMatches(slugPattern, slugValidationMessage),
				},
			},
			"two_step_callback": schema.BoolAttribute{
				Description: "Whether the gateway OAuth callback requires an explicit user confirmation before " +
					"exchanging the authorization code.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(false),
				},
			},
			"external_authorization_url": schema.StringAttribute{
				Description: "HTTPS authorization server URL. Setting it marks the source as OAuth-protected and " +
					"makes the gateway run per-user OAuth at runtime.",
				Optional: true,
			},
			"scopes": schema.ListAttribute{
				Description: "OAuth scopes requested when authenticating with this source.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"client_id": schema.StringAttribute{
				Description: "Pre-registered OAuth client ID. When set, the gateway skips Dynamic Client Registration.",
				Optional:    true,
			},
			"client_secret": schema.StringAttribute{
				Description: "Pre-registered OAuth client secret. Omit for public clients.",
				Optional:    true,
				Sensitive:   true,
			},
			"vendor_id": schema.StringAttribute{
				Description: "Vendor that owns the source.",
				Computed:    true,
			},
			"secret": schema.StringAttribute{
				Description: "Source secret issued by Frontegg.",
				Computed:    true,
				Sensitive:   true,
			},
		},
		Blocks: map[string]schema.Block{
			"override_headers": schema.ListNestedBlock{
				Description: "Headers injected into every outbound request from this source.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							Description: "Header name.",
							Required:    true,
						},
						"value": schema.StringAttribute{
							Description: "Header value.",
							Required:    true,
							Sensitive:   true,
						},
					},
				},
			},
		},
	}
}

func (r *McpSourceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *McpSourceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan McpSourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := r.buildRequest(ctx, plan, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	source, err := r.client.CreateSource(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create MCP source", err.Error())
		return
	}

	r.applySource(ctx, &plan, source, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *McpSourceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state McpSourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	source, err := r.client.GetSource(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read MCP source", err.Error())
		return
	}
	if source == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applySource(ctx, &state, source, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *McpSourceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan McpSourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := r.buildRequest(ctx, plan, false, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	source, err := r.client.UpdateSource(ctx, plan.ID.ValueString(), request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update MCP source", err.Error())
		return
	}

	r.applySource(ctx, &plan, source, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *McpSourceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state McpSourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteSource(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete MCP source", err.Error())
	}
}

// ImportState accepts "application_id/source_id" because reads are scoped to an application.
func (r *McpSourceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	segments, err := splitImportID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"application_id/source_id\": %s", err.Error()),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), segments[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), segments[1])...)
}

// buildRequest assembles the API payload. isCreate controls the create-only fields.
func (r *McpSourceResource) buildRequest(
	ctx context.Context,
	plan McpSourceResourceModel,
	isCreate bool,
	diagnostics *diag.Diagnostics,
) client.SourceRequest {
	request := client.SourceRequest{
		AppID:                    plan.ApplicationID.ValueString(),
		Name:                     plan.Name.ValueString(),
		Type:                     plan.Type.ValueString(),
		SourceURL:                plan.SourceURL.ValueString(),
		APITimeout:               plan.APITimeout.ValueInt64(),
		Enabled:                  boolPointer(plan.Enabled),
		Slug:                     stringPointer(plan.Slug),
		TwoStepCallback:          boolPointer(plan.TwoStepCallback),
		ExternalAuthorizationURL: stringPointer(plan.ExternalAuthorizationURL),
		Scopes:                   stringSlice(ctx, plan.Scopes, diagnostics),
		ClientID:                 stringPointer(plan.ClientID),
		ClientSecret:             stringPointer(plan.ClientSecret),
	}

	if isCreate {
		request.IsLocal = boolPointer(plan.IsLocal)
	}

	if len(plan.OverrideHeaders) > 0 {
		headers := make([]client.OverrideHeader, 0, len(plan.OverrideHeaders))
		for _, header := range plan.OverrideHeaders {
			headers = append(headers, client.OverrideHeader{
				Key:   header.Key.ValueString(),
				Value: header.Value.ValueString(),
			})
		}
		request.OverrideHeaders = headers
	}

	return request
}

func (r *McpSourceResource) applySource(
	ctx context.Context,
	model *McpSourceResourceModel,
	source *client.Source,
	diagnostics *diag.Diagnostics,
) {
	model.ID = types.StringValue(source.ID)
	model.ApplicationID = types.StringValue(source.AppID)
	model.VendorID = types.StringValue(source.VendorID)
	model.Name = types.StringValue(source.Name)
	// type forces replacement, so leaving it unset here would make every imported source be
	// destroyed and recreated on the next apply.
	model.Type = types.StringValue(source.Type)
	model.SourceURL = types.StringValue(source.SourceURL)
	model.APITimeout = types.Int64Value(source.APITimeout)
	model.Enabled = types.BoolValue(source.Enabled)
	model.IsLocal = types.BoolValue(source.IsLocal)
	model.TwoStepCallback = types.BoolValue(source.TwoStepCallback)
	model.Slug = optionalString(source.Slug)
	model.ExternalAuthorizationURL = optionalString(source.ExternalAuthorizationURL)
	model.ClientID = optionalString(source.ClientID)
	model.Secret = types.StringValue(source.Secret)

	// The API returns scopes as an empty array when none are set; keep that null so an
	// unset attribute does not permanently differ from the read-back value.
	if len(source.Scopes) == 0 {
		model.Scopes = types.ListNull(types.StringType)
	} else {
		model.Scopes = stringListValue(ctx, source.Scopes, diagnostics)
	}

	// client_secret is deliberately not read back. Hybrid and on-prem deployments store it
	// already encrypted with their own key, so the returned value need not equal what was
	// sent, and writing a value into an attribute the configuration left unset would make
	// Terraform reject the apply. The configured value is kept instead, which means a secret
	// rotated outside Terraform is not detected as drift.

	if len(source.OverrideHeaders) == 0 {
		model.OverrideHeaders = nil
		return
	}
	headers := make([]OverrideHeaderModel, 0, len(source.OverrideHeaders))
	for _, header := range source.OverrideHeaders {
		headers = append(headers, OverrideHeaderModel{
			Key:   types.StringValue(header.Key),
			Value: types.StringValue(header.Value),
		})
	}
	model.OverrideHeaders = headers
}
