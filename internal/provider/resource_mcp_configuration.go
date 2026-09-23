package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &McpConfigurationResource{}
	_ resource.ResourceWithImportState = &McpConfigurationResource{}
)

// behaviorRiskLevels are the threshold keys accepted in behavior_risk_actions.
var behaviorRiskLevels = []string{"low", "medium", "high"}

// behaviorRiskActions are the enforcement actions a risk level can map to.
var behaviorRiskActions = []string{"observe", "step_up", "block"}

const (
	minAPITimeout = 500
	// defaultMaxAPITimeout is documented only; the API enforces the real cap, which a feature flag can raise.
	defaultMaxAPITimeout = 5000

	// defaultAPITimeout matches what the Frontegg portal sends when onboarding a SaaS
	// application. The API rejects a missing apiTimeout, so the provider must supply one.
	defaultAPITimeout = 5000

	// placeholderBaseURL stands in for a base URL the caller did not supply. The API requires
	// one, but the gateway resolves a source-backed tool against its own source URL, so this is
	// only ever used by tools with no source. It is the value the Frontegg portal sends, and
	// example.com is reserved by RFC 2606 so it cannot reach a real service.
	placeholderBaseURL = "https://example.com"
)

func NewMcpConfigurationResource() resource.Resource {
	return &McpConfigurationResource{}
}

// McpConfigurationResource manages the SaaS MCP configuration of one application.
type McpConfigurationResource struct {
	client *client.Client
}

// McpConfigurationResourceModel is the Terraform state for an MCP configuration.
type McpConfigurationResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	ApplicationID            types.String `tfsdk:"application_id"`
	BaseURL                  types.String `tfsdk:"base_url"`
	APITimeout               types.Int64  `tfsdk:"api_timeout"`
	ExternalAuthorizationURL types.String `tfsdk:"external_authorization_url"`
	EnableAdvancedTools      types.Bool   `tfsdk:"enable_advanced_tools"`
	SlimSemanticSearch       types.Bool   `tfsdk:"slim_semantic_search_enabled"`
	IntegrationToolsEnabled  types.Bool   `tfsdk:"integration_tools_enabled"`
	BehaviorRiskThreshold    types.String `tfsdk:"behavior_risk_threshold"`
	BehaviorRiskActions      types.Map    `tfsdk:"behavior_risk_actions"`
	ListToolPageSize         types.Int64  `tfsdk:"list_tool_page_size"`
}

func (r *McpConfigurationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_mcp_configuration"
}

func (r *McpConfigurationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the MCP gateway configuration for a SaaS application. The API has no delete " +
			"route for this object, so destroying the resource only drops it from Terraform state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "MCP configuration ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application this configuration belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"base_url": schema.StringAttribute{
				Description: "HTTPS base URL the gateway calls for tools that are not attached to a source. The API " +
					"requires a value, so it defaults to https://example.com — the placeholder the portal " +
					"sends. It is worth setting only if you upsert tools with no source: for anything " +
					"imported into an agenco_mcp_source the gateway replaces this with the source's " +
					"source_url at invocation time. example.com is RFC 2606 reserved, so the placeholder " +
					"cannot reach a real service.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					createDefaultString(placeholderBaseURL),
				},
			},
			"api_timeout": schema.Int64Attribute{
				Description: fmt.Sprintf("Upstream request timeout in milliseconds, at least %d and at most %d "+
					"unless the account allows extended timeouts. The API "+
					"requires this field, so the provider defaults it to %d — the value the Frontegg portal "+
					"uses when onboarding a SaaS application.",
					minAPITimeout, defaultMaxAPITimeout, defaultAPITimeout),
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					createDefaultInt64(defaultAPITimeout),
					int64planmodifier.UseStateForUnknown(),
				},
				Validators: []validator.Int64{
					int64validator.AtLeast(minAPITimeout),
				},
			},
			"external_authorization_url": schema.StringAttribute{
				Description: "HTTPS authorization server URL when the upstream API is OAuth-protected.",
				Optional:    true,
			},
			"enable_advanced_tools": schema.BoolAttribute{
				Description: "Whether advanced tools are exposed through the gateway.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(false),
				},
			},
			"slim_semantic_search_enabled": schema.BoolAttribute{
				Description: "Whether tool discovery uses the slim semantic search response.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(false),
				},
			},
			"integration_tools_enabled": schema.BoolAttribute{
				Description: "Whether connector tools from custom integrations are exposed.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(false),
				},
			},
			"behavior_risk_threshold": schema.StringAttribute{
				Description: "Risk level at which behavior enforcement kicks in. One of: low, medium, high.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					createDefaultString("high"),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(behaviorRiskLevels...),
				},
			},
			"behavior_risk_actions": schema.MapAttribute{
				Description: "Action taken per risk level, for example {low = \"observe\", high = \"block\"}. " +
					"Keys must be low, medium or high; values must be observe, step_up or block.",
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.Map{
					mapvalidator.KeysAre(stringvalidator.OneOf(behaviorRiskLevels...)),
					mapvalidator.ValueStringsAre(stringvalidator.OneOf(behaviorRiskActions...)),
				},
			},
			"list_tool_page_size": schema.Int64Attribute{
				Description: "Page size for tools/list responses, between 1 and 500. Leave unset for no paging.",
				Optional:    true,
				Validators: []validator.Int64{
					int64validator.Between(1, 500),
				},
			},
		},
	}
}

func (r *McpConfigurationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *McpConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan McpConfigurationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.upsert(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *McpConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state McpConfigurationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	configuration, err := r.client.GetMcpConfiguration(ctx, state.ApplicationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read MCP configuration", err.Error())
		return
	}
	if configuration == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyConfiguration(ctx, &state, configuration, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *McpConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan McpConfigurationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.upsert(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete drops the resource from state. The API exposes no delete route for this object.
func (r *McpConfigurationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"MCP configuration was not deleted remotely",
		"The Frontegg API has no delete route for an application's MCP configuration. It has been removed "+
			"from Terraform state but still exists. Delete the application to remove it.",
	)
}

func (r *McpConfigurationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("application_id"), req, resp)
}

func (r *McpConfigurationResource) upsert(ctx context.Context, plan *McpConfigurationResourceModel, diagnostics *diag.Diagnostics) {
	request := client.McpConfigurationRequest{
		AppID:                    plan.ApplicationID.ValueString(),
		BaseURL:                  plan.BaseURL.ValueString(),
		APITimeout:               plan.APITimeout.ValueInt64(),
		Type:                     client.McpConfigurationTypeSaas,
		ExternalAuthorizationURL: stringPointer(plan.ExternalAuthorizationURL),
		EnableAdvancedTools:      boolPointer(plan.EnableAdvancedTools),
		SlimSemanticSearchOn:     boolPointer(plan.SlimSemanticSearch),
		IntegrationToolsEnabled:  boolPointer(plan.IntegrationToolsEnabled),
		BehaviorRiskThreshold:    plan.BehaviorRiskThreshold.ValueString(),
		BehaviorRiskActions:      stringMap(ctx, plan.BehaviorRiskActions, diagnostics),
		ListToolPageSize:         int64Pointer(plan.ListToolPageSize),
	}
	if diagnostics.HasError() {
		return
	}

	configuration, err := r.client.UpsertMcpConfiguration(ctx, request)
	if err != nil {
		diagnostics.AddError("Unable to write MCP configuration", err.Error())
		return
	}
	r.applyConfiguration(ctx, plan, configuration, diagnostics)
}

func (r *McpConfigurationResource) applyConfiguration(
	ctx context.Context,
	model *McpConfigurationResourceModel,
	configuration *client.McpConfiguration,
	diagnostics *diag.Diagnostics,
) {
	model.ID = types.StringValue(configuration.ID)
	model.ApplicationID = types.StringValue(configuration.AppID)
	model.BaseURL = types.StringValue(configuration.BaseURL)
	model.APITimeout = types.Int64Value(configuration.APITimeout)
	model.ExternalAuthorizationURL = optionalString(configuration.ExternalAuthorizationURL)
	model.EnableAdvancedTools = types.BoolValue(configuration.EnableAdvancedTools)
	model.SlimSemanticSearch = types.BoolValue(configuration.SlimSemanticSearchOn)
	model.IntegrationToolsEnabled = types.BoolValue(configuration.IntegrationToolsEnabled)
	model.BehaviorRiskThreshold = types.StringValue(configuration.BehaviorRiskThreshold)
	model.BehaviorRiskActions = stringMapValue(ctx, configuration.BehaviorRiskActions, diagnostics)
	model.ListToolPageSize = optionalInt64(configuration.ListToolPageSize)
}
