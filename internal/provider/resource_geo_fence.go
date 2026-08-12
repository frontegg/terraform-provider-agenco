package provider

import (
	"context"

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
	_ resource.Resource                = &GeoFenceResource{}
	_ resource.ResourceWithImportState = &GeoFenceResource{}
)

func NewGeoFenceResource() resource.Resource {
	return &GeoFenceResource{}
}

// GeoFenceResource manages the country list of an application's geo-fence.
type GeoFenceResource struct {
	client *client.Client
}

// GeoFenceResourceModel is the Terraform state for a geo-fence.
type GeoFenceResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	Countries     types.Set    `tfsdk:"countries"`
	Description   types.String `tfsdk:"description"`
}

func (r *GeoFenceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_geo_fence"
}

func (r *GeoFenceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the country list of an application's geo-fence. One geo-fence exists per " +
			"application; whether the list allows or blocks is set by agenco_geo_fence_config.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Geo-fence ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application this geo-fence applies to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"countries": schema.SetAttribute{
				Description: "Two-letter ISO 3166-1 alpha-2 country codes.",
				Required:    true,
				ElementType: types.StringType,
			},
			"description": schema.StringAttribute{
				Description: "Free-text description, up to 255 characters. Not returned by the API, so the " +
					"configured value is kept in state.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(255),
				},
			},
		},
	}
}

func (r *GeoFenceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *GeoFenceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan GeoFenceResourceModel
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

func (r *GeoFenceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state GeoFenceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fence, err := r.client.GetGeoFence(ctx, state.ApplicationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read geo-fence", err.Error())
		return
	}
	if fence == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.ID = types.StringValue(fence.ID)
	state.ApplicationID = types.StringValue(fence.AppID)
	state.Countries = stringSetValue(ctx, fence.Countries, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *GeoFenceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan GeoFenceResourceModel
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

func (r *GeoFenceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GeoFenceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteGeoFence(ctx, state.ApplicationID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete geo-fence", err.Error())
	}
}

func (r *GeoFenceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("application_id"), req, resp)
}

// upsert writes the geo-fence; the API replaces the whole country list on every POST.
func (r *GeoFenceResource) upsert(ctx context.Context, plan *GeoFenceResourceModel, diagnostics *diag.Diagnostics) {
	countries := stringSlice(ctx, plan.Countries, diagnostics)
	if diagnostics.HasError() {
		return
	}

	fence, err := r.client.UpsertGeoFence(ctx, plan.ApplicationID.ValueString(), countries, stringPointer(plan.Description))
	if err != nil {
		diagnostics.AddError("Unable to write geo-fence", err.Error())
		return
	}

	plan.ID = types.StringValue(fence.ID)
	plan.ApplicationID = types.StringValue(fence.AppID)
	plan.Countries = stringSetValue(ctx, fence.Countries, diagnostics)
}
