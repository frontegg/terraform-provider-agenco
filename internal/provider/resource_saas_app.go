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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// onboardingPlaceholderURL is sent for a URL the caller omitted. The API requires appURL and
// loginURL on create, but the derived values depend on the appHost that same call assigns, so a
// placeholder is unavoidable for one round trip. This is the value Frontegg's own onboarding uses.
const onboardingPlaceholderURL = "http://localhost:3000"

var (
	_ resource.Resource                = &SaasAppResource{}
	_ resource.ResourceWithImportState = &SaasAppResource{}
	_ resource.ResourceWithConfigure   = &SaasAppResource{}
)

func NewSaasAppResource() resource.Resource {
	return &SaasAppResource{}
}

// SaasAppResource creates an application and its MCP gateway configuration as a single unit.
type SaasAppResource struct {
	client *client.Client
}

// SaasAppResourceModel is the Terraform state for an onboarded SaaS application.
type SaasAppResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	VendorID      types.String `tfsdk:"vendor_id"`
	AppHost       types.String `tfsdk:"app_host"`

	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	LogoURL       types.String `tfsdk:"logo_url"`
	AppURL        types.String `tfsdk:"app_url"`
	LoginURL      types.String `tfsdk:"login_url"`
	AccessType    types.String `tfsdk:"access_type"`
	Type          types.String `tfsdk:"type"`
	FrontendStack types.String `tfsdk:"frontend_stack"`
	IsDefault     types.Bool   `tfsdk:"is_default"`
	IsActive      types.Bool   `tfsdk:"is_active"`
	AllowDcr      types.Bool   `tfsdk:"allow_dcr"`
	AllowCimd     types.Bool   `tfsdk:"allow_cimd"`
	DPoPEnforce   types.String `tfsdk:"dpop_enforcement_type"`

	McpConfigurationID       types.String `tfsdk:"mcp_configuration_id"`
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

func (r *SaasAppResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_saas_app"
}

func (r *SaasAppResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Creates a SaaS application together with its MCP gateway configuration, so onboarding " +
			"an Agenco-enabled application is a single resource rather than two.\n\n" +
			"This resource spans two API objects, which has three consequences worth understanding before " +
			"using it:\n\n" +
			"  * **Do not also manage the same application with `agenco_application` or " +
			"`agenco_mcp_configuration`.** The MCP configuration endpoint is an upsert keyed on the " +
			"application, so two resources pointed at one application will fight over it on every apply.\n" +
			"  * **Destroying this resource deletes the application**, and the MCP configuration along with " +
			"it, since the API has no delete route for the configuration on its own.\n" +
			"  * Attach sources, tools and policies using the exported `application_id`.\n\n" +
			"For finer-grained control, use `agenco_application` plus `agenco_mcp_configuration` instead.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Application ID. This resource is keyed on the application it creates.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application ID, for wiring up sources, tools and policies.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vendor_id": schema.StringAttribute{
				Description: "Vendor that owns the application.",
				Computed:    true,
			},
			"app_host": schema.StringAttribute{
				Description: "Host Frontegg assigned to the application.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "Display name of the application.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Free-text description of the application.",
				Optional:    true,
			},
			"logo_url": schema.StringAttribute{
				Description: "URL of the application logo.",
				Optional:    true,
			},
			"app_url": schema.StringAttribute{
				Description: "URL the application is served from. Omit it and the provider derives " +
					"https://{app_host}/oauth/portal from the host Frontegg assigns, which is what an " +
					"agent-only application wants. A value you supply is never overridden.",
				Optional: true,
				Computed: true,
			},
			"login_url": schema.StringAttribute{
				Description: "URL users are sent to in order to log in. Omit it and the provider derives " +
					"https://{app_host}/oauth — the application's own OAuth endpoint, where MCP clients " +
					"authenticate. A value you supply is never overridden.",
				Optional: true,
				Computed: true,
			},
			"access_type": schema.StringAttribute{
				Description: fmt.Sprintf("Access model for the application. One of: %v. Defaults to FREE_ACCESS.",
					applicationAccessTypes),
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					createDefaultString("FREE_ACCESS"),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(applicationAccessTypes...),
				},
			},
			"type": schema.StringAttribute{
				Description: fmt.Sprintf("Client form factor. One of: %v. Defaults to web.", applicationTypes),
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					createDefaultString("web"),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(applicationTypes...),
				},
			},
			"frontend_stack": schema.StringAttribute{
				Description: fmt.Sprintf("Frontend stack. One of: %v. Defaults to react.", applicationFrontendStacks),
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					createDefaultString("react"),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(applicationFrontendStacks...),
				},
			},
			"is_default": schema.BoolAttribute{
				Description: "Whether this is the vendor's default application. Defaults to false.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(false),
				},
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether the application is active. Defaults to true.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(true),
				},
			},
			"allow_dcr": schema.BoolAttribute{
				Description: "Whether OAuth Dynamic Client Registration is allowed, which is how MCP clients " +
					"register themselves. Defaults to true, matching portal onboarding.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(true),
				},
			},
			"allow_cimd": schema.BoolAttribute{
				Description: "Whether clients may identify themselves with a Client ID Metadata Document " +
					"instead of pre-registering. The CIMD counterpart to allow_dcr. Defaults to false.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(false),
				},
			},
			"dpop_enforcement_type": schema.StringAttribute{
				Description: fmt.Sprintf("How strictly DPoP proof-of-possession is applied to tokens issued "+
					"for this application. One of: %v. Defaults to disabled.", dpopEnforcementTypes),
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					createDefaultString("disabled"),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(dpopEnforcementTypes...),
				},
			},
			"mcp_configuration_id": schema.StringAttribute{
				Description: "ID of the MCP configuration created for the application.",
				Computed:    true,
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
				Description: fmt.Sprintf("Upstream request timeout in milliseconds, between %d and %d. Defaults to %d.",
					minAPITimeout, maxAPITimeout, defaultAPITimeout),
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					createDefaultInt64(defaultAPITimeout),
				},
				Validators: []validator.Int64{
					int64validator.Between(minAPITimeout, maxAPITimeout),
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
				Description: "Action taken per risk level, for example {low = \"observe\", high = \"block\"}.",
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

func (r *SaasAppResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *SaasAppResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SaasAppResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Capture what the caller supplied before the API response overwrites the model. Whatever
	// they left out is derived from the host the create call assigns.
	suppliedAppURL, suppliedLoginURL := stringPointer(plan.AppURL), stringPointer(plan.LoginURL)

	application, err := r.client.CreateApplication(ctx,
		r.applicationRequest(plan, valueOr(suppliedAppURL, onboardingPlaceholderURL), valueOr(suppliedLoginURL, onboardingPlaceholderURL)))
	if err != nil {
		resp.Diagnostics.AddError("Unable to create application", err.Error())
		return
	}

	// From here the application exists remotely, so a later failure saves state before
	// returning rather than leaking it as an object Terraform does not know about.
	r.applyApplication(&plan, application)

	if suppliedAppURL == nil || suppliedLoginURL == nil {
		derivedAppURL, derivedLoginURL := fronteggOAuthURLs(application.AppHost)
		appURL, loginURL := valueOr(suppliedAppURL, derivedAppURL), valueOr(suppliedLoginURL, derivedLoginURL)

		if err := r.client.UpdateApplication(ctx, application.ID, r.applicationRequest(plan, appURL, loginURL)); err != nil {
			r.saveIncomplete(ctx, &plan, resp,
				"Unable to set the application URLs",
				fmt.Sprintf("The application was created as %s, but setting its URLs from the assigned host "+
					"failed: %s. It has been written to state; the next apply will retry.",
					application.ID, err.Error()))
			return
		}

		refreshed, err := r.client.GetApplication(ctx, application.ID)
		if err != nil || refreshed == nil {
			r.saveIncomplete(ctx, &plan, resp,
				"Unable to re-read the application after setting its URLs",
				fmt.Sprintf("The application was created as %s but could not be read back. It has been "+
					"written to state; the next apply will reconcile it.", application.ID))
			return
		}
		r.applyApplication(&plan, refreshed)
	}

	configuration, err := r.upsertConfiguration(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err != nil {
		r.saveIncomplete(ctx, &plan, resp,
			"Unable to configure the MCP gateway",
			fmt.Sprintf("The application was created as %s but its MCP configuration failed: %s. The "+
				"application has been written to state; the next apply will complete the configuration.",
				application.ID, err.Error()))
		return
	}

	r.applyConfiguration(ctx, &plan, configuration, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *SaasAppResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SaasAppResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	application, err := r.client.GetApplication(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read application", err.Error())
		return
	}
	if application == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	r.applyApplication(&state, application)

	configuration, err := r.client.GetMcpConfiguration(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read MCP configuration", err.Error())
		return
	}
	if configuration == nil {
		// The application outlived its configuration. Clear the computed half so the next plan
		// shows the gap and the upsert on apply restores it.
		state.McpConfigurationID = types.StringNull()
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	r.applyConfiguration(ctx, &state, configuration, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SaasAppResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SaasAppResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state SaasAppResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	applicationID := state.ID.ValueString()

	if err := r.client.UpdateApplication(ctx, applicationID,
		r.applicationRequest(plan, plan.AppURL.ValueString(), plan.LoginURL.ValueString())); err != nil {
		resp.Diagnostics.AddError("Unable to update application", err.Error())
		return
	}

	application, err := r.client.GetApplication(ctx, applicationID)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read application after update", err.Error())
		return
	}
	if application == nil {
		resp.Diagnostics.AddError(
			"Application disappeared after update",
			"The application was updated but can no longer be read. Re-run the plan to reconcile.",
		)
		return
	}
	r.applyApplication(&plan, application)

	configuration, err := r.upsertConfiguration(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to update the MCP configuration", err.Error())
		return
	}

	r.applyConfiguration(ctx, &plan, configuration, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the application, which takes its MCP configuration with it.
func (r *SaasAppResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state SaasAppResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteApplication(ctx, state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete application", err.Error())
	}
}

// ImportState takes the application ID; Read recovers both API objects from it.
func (r *SaasAppResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// saveIncomplete records a partially onboarded application before reporting the failure, so the
// created application is never orphaned outside Terraform's knowledge.
func (r *SaasAppResource) saveIncomplete(
	ctx context.Context,
	plan *SaasAppResourceModel,
	resp *resource.CreateResponse,
	summary, detail string,
) {
	plan.McpConfigurationID = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	resp.Diagnostics.AddError(summary, detail)
}

func (r *SaasAppResource) applicationRequest(plan SaasAppResourceModel, appURL, loginURL string) client.ApplicationRequest {
	return client.ApplicationRequest{
		Name:          plan.Name.ValueString(),
		AppURL:        appURL,
		LoginURL:      loginURL,
		LogoURL:       plan.LogoURL.ValueString(),
		AccessType:    plan.AccessType.ValueString(),
		IsDefault:     boolPointer(plan.IsDefault),
		IsActive:      boolPointer(plan.IsActive),
		Type:          plan.Type.ValueString(),
		FrontendStack: plan.FrontendStack.ValueString(),
		Description:   plan.Description.ValueString(),
		AllowDcr:      boolPointer(plan.AllowDcr),
		AllowCimd:     boolPointer(plan.AllowCimd),
		DPoPEnforce:   plan.DPoPEnforce.ValueString(),
	}
}

func (r *SaasAppResource) upsertConfiguration(
	ctx context.Context,
	plan SaasAppResourceModel,
	diagnostics *diag.Diagnostics,
) (*client.McpConfiguration, error) {
	request := client.McpConfigurationRequest{
		AppID:                    plan.ID.ValueString(),
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
		return nil, nil
	}
	return r.client.UpsertMcpConfiguration(ctx, request)
}

func (r *SaasAppResource) applyApplication(model *SaasAppResourceModel, application *client.Application) {
	model.ID = types.StringValue(application.ID)
	model.ApplicationID = types.StringValue(application.ID)
	model.VendorID = types.StringValue(application.VendorID)
	model.AppHost = types.StringValue(application.AppHost)
	model.Name = types.StringValue(application.Name)
	model.AppURL = types.StringValue(application.AppURL)
	model.LoginURL = types.StringValue(application.LoginURL)
	model.AccessType = types.StringValue(application.AccessType)
	model.IsDefault = types.BoolValue(application.IsDefault)
	model.IsActive = types.BoolValue(application.IsActive)
	model.Type = types.StringValue(application.Type)
	model.FrontendStack = types.StringValue(application.FrontendStack)
	model.AllowDcr = types.BoolValue(application.AllowDcr)
	model.AllowCimd = types.BoolValue(application.AllowCimd)
	model.DPoPEnforce = types.StringValue(application.DPoPEnforce)
	model.LogoURL = optionalString(&application.LogoURL)
	model.Description = optionalString(&application.Description)
}

func (r *SaasAppResource) applyConfiguration(
	ctx context.Context,
	model *SaasAppResourceModel,
	configuration *client.McpConfiguration,
	diagnostics *diag.Diagnostics,
) {
	model.McpConfigurationID = types.StringValue(configuration.ID)
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

// fronteggOAuthURLs returns the application's own OAuth endpoints on the host Frontegg assigned
// it. These paths are taken from portal and bootstrap behavior, not a documented contract.
func fronteggOAuthURLs(appHost string) (appURL string, loginURL string) {
	return fmt.Sprintf("https://%s/oauth/portal", appHost), fmt.Sprintf("https://%s/oauth", appHost)
}

// valueOr resolves an optional caller-supplied string against a fallback.
func valueOr(supplied *string, fallback string) string {
	if supplied == nil {
		return fallback
	}
	return *supplied
}
