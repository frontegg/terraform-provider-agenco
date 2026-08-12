package provider

import (
	"context"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// restrictionStrategies are the allow-list and block-list modes shared by IP and geo controls.
var restrictionStrategies = []string{"allow", "block"}

// strategyConfigModel is the shared state of the IP-restriction and geo-fence config singletons.
type strategyConfigModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	Strategy      types.String `tfsdk:"strategy"`
	IsActive      types.Bool   `tfsdk:"is_active"`
}

// strategyConfigSchema builds the identical schema used by both strategy config resources.
func strategyConfigSchema(description, subject string) schema.Schema {
	return schema.Schema{
		Description: description,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Configuration ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application this configuration applies to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"strategy": schema.StringAttribute{
				Description: "Whether the configured " + subject + " are allowed or blocked. One of: allow, block.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(restrictionStrategies...),
				},
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether enforcement is switched on.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(true),
				},
			},
		},
	}
}

// ---- IP restriction config ----

var (
	_ resource.Resource                = &IPRestrictionConfigResource{}
	_ resource.ResourceWithImportState = &IPRestrictionConfigResource{}
)

func NewIPRestrictionConfigResource() resource.Resource {
	return &IPRestrictionConfigResource{}
}

// IPRestrictionConfigResource manages the allow/block mode of an application's IP list.
type IPRestrictionConfigResource struct {
	client *client.Client
}

func (r *IPRestrictionConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_restriction_config"
}

func (r *IPRestrictionConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = strategyConfigSchema(
		"Manages whether an application's IP restrictions act as an allow-list or a block-list. "+
			"One configuration exists per application.",
		"IP addresses",
	)
}

func (r *IPRestrictionConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *IPRestrictionConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan strategyConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.UpsertIPRestrictionConfig(
		ctx, plan.ApplicationID.ValueString(), plan.Strategy.ValueString(), plan.IsActive.ValueBool(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to write IP restriction config", err.Error())
		return
	}

	plan.ID = types.StringValue(config.ID)
	plan.ApplicationID = types.StringValue(config.AppID)
	plan.Strategy = types.StringValue(config.Strategy)
	plan.IsActive = types.BoolValue(config.IsActive)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IPRestrictionConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state strategyConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.GetIPRestrictionConfig(ctx, state.ApplicationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read IP restriction config", err.Error())
		return
	}
	if config == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(config.ID)
	state.ApplicationID = types.StringValue(config.AppID)
	state.Strategy = types.StringValue(config.Strategy)
	state.IsActive = types.BoolValue(config.IsActive)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IPRestrictionConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan strategyConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.UpsertIPRestrictionConfig(
		ctx, plan.ApplicationID.ValueString(), plan.Strategy.ValueString(), plan.IsActive.ValueBool(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update IP restriction config", err.Error())
		return
	}

	plan.ID = types.StringValue(config.ID)
	plan.Strategy = types.StringValue(config.Strategy)
	plan.IsActive = types.BoolValue(config.IsActive)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IPRestrictionConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state strategyConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteIPRestrictionConfig(ctx, state.ApplicationID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete IP restriction config", err.Error())
	}
}

func (r *IPRestrictionConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("application_id"), req, resp)
}

// ---- Geo-fence config ----

var (
	_ resource.Resource                = &GeoFenceConfigResource{}
	_ resource.ResourceWithImportState = &GeoFenceConfigResource{}
)

func NewGeoFenceConfigResource() resource.Resource {
	return &GeoFenceConfigResource{}
}

// GeoFenceConfigResource manages the allow/block mode of an application's geo-fence.
type GeoFenceConfigResource struct {
	client *client.Client
}

func (r *GeoFenceConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_geo_fence_config"
}

func (r *GeoFenceConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = strategyConfigSchema(
		"Manages whether an application's geo-fence acts as an allow-list or a block-list. "+
			"One configuration exists per application.",
		"countries",
	)
}

func (r *GeoFenceConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *GeoFenceConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan strategyConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.UpsertGeoFenceConfig(
		ctx, plan.ApplicationID.ValueString(), plan.Strategy.ValueString(), plan.IsActive.ValueBool(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to write geo-fence config", err.Error())
		return
	}

	plan.ID = types.StringValue(config.ID)
	plan.ApplicationID = types.StringValue(config.AppID)
	plan.Strategy = types.StringValue(config.Strategy)
	plan.IsActive = types.BoolValue(config.IsActive)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *GeoFenceConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state strategyConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.GetGeoFenceConfig(ctx, state.ApplicationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read geo-fence config", err.Error())
		return
	}
	if config == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(config.ID)
	state.ApplicationID = types.StringValue(config.AppID)
	state.Strategy = types.StringValue(config.Strategy)
	state.IsActive = types.BoolValue(config.IsActive)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *GeoFenceConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan strategyConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.UpsertGeoFenceConfig(
		ctx, plan.ApplicationID.ValueString(), plan.Strategy.ValueString(), plan.IsActive.ValueBool(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update geo-fence config", err.Error())
		return
	}

	plan.ID = types.StringValue(config.ID)
	plan.Strategy = types.StringValue(config.Strategy)
	plan.IsActive = types.BoolValue(config.IsActive)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *GeoFenceConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state strategyConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteGeoFenceConfig(ctx, state.ApplicationID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete geo-fence config", err.Error())
	}
}

func (r *GeoFenceConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("application_id"), req, resp)
}
