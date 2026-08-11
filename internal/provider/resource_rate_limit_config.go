package provider

import (
	"context"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
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
	_ resource.Resource                = &RateLimitConfigResource{}
	_ resource.ResourceWithImportState = &RateLimitConfigResource{}
)

func NewRateLimitConfigResource() resource.Resource {
	return &RateLimitConfigResource{}
}

// RateLimitConfigResource manages an application's per-minute request budgets.
type RateLimitConfigResource struct {
	client *client.Client
}

// RateLimitConfigResourceModel is the Terraform state for a rate-limit configuration.
type RateLimitConfigResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	Sources       types.Map    `tfsdk:"sources"`
	Tools         types.Map    `tfsdk:"tools"`
	Tenants       types.Map    `tfsdk:"tenants"`
	Users         types.Map    `tfsdk:"users"`
}

func (r *RateLimitConfigResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rate_limit_config"
}

func (r *RateLimitConfigResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	budgetValidators := []validator.Map{
		mapvalidator.ValueInt64sAre(int64validator.AtLeast(1)),
	}

	resp.Schema = schema.Schema{
		Description: "Manages an application's rate-limit budgets, in requests per minute. One configuration " +
			"exists per application and every write replaces it in full.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Rate-limit configuration ID.",
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
			"sources": schema.MapAttribute{
				Description: "Requests per minute keyed by MCP source ID.",
				Optional:    true,
				ElementType: types.Int64Type,
				Validators:  budgetValidators,
			},
			"tools": schema.MapAttribute{
				Description: "Requests per minute keyed by internal tool ID.",
				Optional:    true,
				ElementType: types.Int64Type,
				Validators:  budgetValidators,
			},
			"tenants": schema.MapAttribute{
				Description: "Requests per minute keyed by tenant ID.",
				Optional:    true,
				ElementType: types.Int64Type,
				Validators:  budgetValidators,
			},
			"users": schema.MapAttribute{
				Description: "Requests per minute keyed by user ID.",
				Optional:    true,
				ElementType: types.Int64Type,
				Validators:  budgetValidators,
			},
		},
	}
}

func (r *RateLimitConfigResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *RateLimitConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RateLimitConfigResourceModel
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

func (r *RateLimitConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RateLimitConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.GetRateLimitConfig(ctx, state.ApplicationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read rate-limit config", err.Error())
		return
	}
	if config == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyConfig(ctx, &state, config, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *RateLimitConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RateLimitConfigResourceModel
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

func (r *RateLimitConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RateLimitConfigResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteRateLimitConfig(ctx, state.ApplicationID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete rate-limit config", err.Error())
	}
}

func (r *RateLimitConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("application_id"), req, resp)
}

// upsert sends every budget map, including the empty ones, so the PUT is a full replace.
func (r *RateLimitConfigResource) upsert(ctx context.Context, plan *RateLimitConfigResourceModel, diagnostics *diag.Diagnostics) {
	sources := emptyIfNil(int64Map(ctx, plan.Sources, diagnostics))
	tools := emptyIfNil(int64Map(ctx, plan.Tools, diagnostics))
	tenants := emptyIfNil(int64Map(ctx, plan.Tenants, diagnostics))
	users := emptyIfNil(int64Map(ctx, plan.Users, diagnostics))
	if diagnostics.HasError() {
		return
	}

	config, err := r.client.UpsertRateLimitConfig(ctx, plan.ApplicationID.ValueString(), sources, tools, tenants, users)
	if err != nil {
		diagnostics.AddError("Unable to write rate-limit config", err.Error())
		return
	}

	r.applyConfig(ctx, plan, config, diagnostics)
}

func (r *RateLimitConfigResource) applyConfig(
	ctx context.Context,
	model *RateLimitConfigResourceModel,
	config *client.RateLimitConfig,
	diagnostics *diag.Diagnostics,
) {
	model.ID = types.StringValue(config.ID)
	model.ApplicationID = types.StringValue(config.AppID)
	model.Sources = int64MapValue(ctx, config.Sources, diagnostics)
	model.Tools = int64MapValue(ctx, config.Tools, diagnostics)
	model.Tenants = int64MapValue(ctx, config.Tenants, diagnostics)
	model.Users = int64MapValue(ctx, config.Users, diagnostics)
}

// emptyIfNil turns an absent map into an empty one so the API clears the stored budget
// instead of preserving it.
func emptyIfNil(values map[string]int64) map[string]int64 {
	if values == nil {
		return map[string]int64{}
	}
	return values
}
