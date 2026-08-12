package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
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

var (
	_ resource.Resource                = &CustomIntegrationResource{}
	_ resource.ResourceWithImportState = &CustomIntegrationResource{}
)

// integrationAuthTypes are the authentication modes a connector instance can use.
var integrationAuthTypes = []string{"oauth", "api_key"}

func NewCustomIntegrationResource() resource.Resource {
	return &CustomIntegrationResource{}
}

// CustomIntegrationResource manages an installed connector instance.
type CustomIntegrationResource struct {
	client *client.Client
}

// CustomIntegrationResourceModel is the Terraform state for a connector instance.
type CustomIntegrationResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	ApplicationID         types.String `tfsdk:"application_id"`
	IntegrationTemplateID types.String `tfsdk:"integration_template_id"`
	Name                  types.String `tfsdk:"name"`
	Description           types.String `tfsdk:"description"`
	Slug                  types.String `tfsdk:"slug"`
	APINames              types.List   `tfsdk:"api_names"`
	EnabledAPIs           types.Map    `tfsdk:"enabled_apis"`
	AuthType              types.String `tfsdk:"auth_type"`
	IsActive              types.Bool   `tfsdk:"is_active"`
	IsDevCredsEnabled     types.Bool   `tfsdk:"is_dev_creds_enabled"`
	ClientID              types.String `tfsdk:"client_id"`
	ClientSecret          types.String `tfsdk:"client_secret"`
	BaseURL               types.String `tfsdk:"base_url"`
	CustomConfiguration   types.String `tfsdk:"custom_configuration"`
	APIKey                types.String `tfsdk:"api_key"`
}

func (r *CustomIntegrationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_integration"
}

func (r *CustomIntegrationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a connector instance built from a Frontegg integration template. Use the " +
			"agenco_integration_templates data source to discover available template IDs.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Custom integration ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application this connector belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"integration_template_id": schema.StringAttribute{
				Description: "Template the connector is built from, for example \"slack\". Immutable after create.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(64),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Connector display name.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Free-text description of the connector instance.",
				Optional:    true,
			},
			"slug": schema.StringAttribute{
				Description: "Instance discriminator for installing more than one instance of the same template. " +
					"Lowercase kebab-case, up to 64 characters. Immutable after create.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(64),
					stringvalidator.RegexMatches(slugPattern, slugValidationMessage),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"api_names": schema.ListAttribute{
				Description:   "APIs to enable on create. Use enabled_apis to change them afterwards.",
				Optional:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.List{},
			},
			"enabled_apis": schema.MapAttribute{
				Description: "Per-API enablement, keyed by API name.",
				Optional:    true,
				Computed:    true,
				ElementType: types.BoolType,
			},
			"auth_type": schema.StringAttribute{
				Description: fmt.Sprintf("Authentication mode. One of: %v.", integrationAuthTypes),
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					createDefaultString("oauth"),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(integrationAuthTypes...),
				},
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether the connector is active.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(true),
				},
			},
			"is_dev_creds_enabled": schema.BoolAttribute{
				Description: "Whether Frontegg development credentials are used instead of your own OAuth app.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(false),
				},
			},
			"client_id": schema.StringAttribute{
				Description: "OAuth client ID. Required when auth_type is oauth and is_dev_creds_enabled is false.",
				Optional:    true,
			},
			"client_secret": schema.StringAttribute{
				Description: "OAuth client secret paired with client_id.",
				Optional:    true,
				Sensitive:   true,
			},
			"base_url": schema.StringAttribute{
				Description: "Override for the upstream API base URL.",
				Optional:    true,
			},
			"custom_configuration": schema.StringAttribute{
				Description: "Template-specific settings encoded as a JSON object string.",
				Optional:    true,
			},
			"api_key": schema.StringAttribute{
				Description: "API key. Required when auth_type is api_key. The API returns it masked, so the " +
					"configured value is kept in state.",
				Optional:  true,
				Sensitive: true,
			},
		},
	}
}

func (r *CustomIntegrationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *CustomIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CustomIntegrationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := client.CreateCustomIntegrationRequest{
		AppID:                 plan.ApplicationID.ValueString(),
		IntegrationTemplateID: plan.IntegrationTemplateID.ValueString(),
		Name:                  plan.Name.ValueString(),
		Slug:                  stringPointer(plan.Slug),
		APINames:              stringSlice(ctx, plan.APINames, &resp.Diagnostics),
		Description:           stringPointer(plan.Description),
		AuthType:              plan.AuthType.ValueString(),
		APIKey:                stringPointer(plan.APIKey),
		IsActive:              boolPointer(plan.IsActive),
		IsDevCredsEnabled:     boolPointer(plan.IsDevCredsEnabled),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	if configuration := r.buildConfiguration(plan, &resp.Diagnostics); configuration != nil {
		request.Configuration = configuration
	}
	if resp.Diagnostics.HasError() {
		return
	}

	integration, err := r.client.CreateCustomIntegration(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create custom integration", err.Error())
		return
	}

	r.applyIntegration(ctx, &plan, integration, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CustomIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CustomIntegrationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	integration, err := r.client.GetCustomIntegration(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read custom integration", err.Error())
		return
	}
	if integration == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyIntegration(ctx, &state, integration, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CustomIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CustomIntegrationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	authType := plan.AuthType.ValueString()
	request := client.UpdateCustomIntegrationRequest{
		AppID:             plan.ApplicationID.ValueString(),
		Name:              &name,
		Description:       stringPointer(plan.Description),
		EnabledAPIs:       boolMap(ctx, plan.EnabledAPIs, &resp.Diagnostics),
		IsActive:          boolPointer(plan.IsActive),
		IsDevCredsEnabled: boolPointer(plan.IsDevCredsEnabled),
		AuthType:          &authType,
		APIKey:            stringPointer(plan.APIKey),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	if configuration := r.buildConfiguration(plan, &resp.Diagnostics); configuration != nil {
		request.Configuration = map[string]interface{}{
			"clientId":     configuration.ClientID,
			"clientSecret": configuration.ClientSecret,
		}
		if configuration.BaseURL != "" {
			request.Configuration["baseUrl"] = configuration.BaseURL
		}
		if configuration.CustomConfiguration != nil {
			request.Configuration["customConfiguration"] = configuration.CustomConfiguration
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}

	integration, err := r.client.UpdateCustomIntegration(ctx, plan.ID.ValueString(), request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update custom integration", err.Error())
		return
	}

	r.applyIntegration(ctx, &plan, integration, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CustomIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CustomIntegrationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteCustomIntegration(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete custom integration", err.Error())
	}
}

// ImportState accepts "application_id/integration_id".
func (r *CustomIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	segments, err := splitImportID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"application_id/integration_id\": %s", err.Error()),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), segments[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), segments[1])...)
}

// buildConfiguration assembles the OAuth credential block, or nil when none is configured.
func (r *CustomIntegrationResource) buildConfiguration(
	plan CustomIntegrationResourceModel,
	diagnostics *diag.Diagnostics,
) *client.CustomIntegrationConfiguration {
	if plan.ClientID.IsNull() && plan.ClientSecret.IsNull() && plan.BaseURL.IsNull() && plan.CustomConfiguration.IsNull() {
		return nil
	}

	customConfiguration := jsonStringToMap(plan.CustomConfiguration, "custom_configuration", diagnostics)
	if diagnostics.HasError() {
		return nil
	}

	return &client.CustomIntegrationConfiguration{
		ClientID:            plan.ClientID.ValueString(),
		ClientSecret:        plan.ClientSecret.ValueString(),
		BaseURL:             plan.BaseURL.ValueString(),
		CustomConfiguration: customConfiguration,
	}
}

func (r *CustomIntegrationResource) applyIntegration(
	ctx context.Context,
	model *CustomIntegrationResourceModel,
	integration *client.CustomIntegration,
	diagnostics *diag.Diagnostics,
) {
	model.ID = types.StringValue(integration.ID)
	model.ApplicationID = types.StringValue(integration.AppID)
	model.IntegrationTemplateID = types.StringValue(integration.IntegrationTemplateID)
	model.Name = types.StringValue(integration.Name)
	model.IsActive = types.BoolValue(integration.IsActive)
	model.IsDevCredsEnabled = types.BoolValue(integration.IsDevCredsEnabled)
	model.AuthType = types.StringValue(integration.AuthType)
	model.Slug = optionalString(integration.Slug)
	model.Description = optionalString(integration.Description)
	model.EnabledAPIs = boolMapValue(ctx, integration.EnabledAPIs, diagnostics)

	// The credential fields are write-only: the read route masks api_key and omits
	// clientSecret unless dev creds are off, so configured values are kept as-is.
}
