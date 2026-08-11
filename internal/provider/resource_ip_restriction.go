package provider

import (
	"context"
	"fmt"

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

var (
	_ resource.Resource                = &IPRestrictionResource{}
	_ resource.ResourceWithImportState = &IPRestrictionResource{}
)

func NewIPRestrictionResource() resource.Resource {
	return &IPRestrictionResource{}
}

// IPRestrictionResource manages one IP or CIDR entry in an application's IP list.
type IPRestrictionResource struct {
	client *client.Client
}

// IPRestrictionResourceModel is the Terraform state for an IP restriction entry.
type IPRestrictionResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	IP            types.String `tfsdk:"ip"`
	Description   types.String `tfsdk:"description"`
	IPType        types.String `tfsdk:"ip_type"`
}

func (r *IPRestrictionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ip_restriction"
}

func (r *IPRestrictionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a single IP or CIDR entry for an application. Whether the list allows or blocks " +
			"these addresses is set by agenco_ip_restriction_config.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "IP restriction ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application this restriction applies to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ip": schema.StringAttribute{
				Description: "IPv4 or IPv6 address, or a CIDR range. Immutable after create.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Description: "Free-text description, up to 255 characters.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(255),
				},
			},
			"ip_type": schema.StringAttribute{
				Description: "Whether the API classified the value as IP or CIDR.",
				Computed:    true,
			},
		},
	}
}

func (r *IPRestrictionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *IPRestrictionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan IPRestrictionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	restriction, err := r.client.CreateIPRestriction(
		ctx,
		plan.ApplicationID.ValueString(),
		plan.IP.ValueString(),
		stringPointer(plan.Description),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create IP restriction", err.Error())
		return
	}

	applyIPRestriction(&plan, restriction)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IPRestrictionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state IPRestrictionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	restriction, err := r.client.GetIPRestriction(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read IP restriction", err.Error())
		return
	}
	if restriction == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	applyIPRestriction(&state, restriction)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IPRestrictionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan IPRestrictionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	restriction, err := r.client.UpdateIPRestriction(
		ctx,
		plan.ApplicationID.ValueString(),
		plan.ID.ValueString(),
		stringPointer(plan.Description),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update IP restriction", err.Error())
		return
	}

	applyIPRestriction(&plan, restriction)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IPRestrictionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state IPRestrictionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteIPRestriction(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete IP restriction", err.Error())
	}
}

// ImportState accepts "application_id/restriction_id".
func (r *IPRestrictionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	segments, err := splitImportID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"application_id/restriction_id\": %s", err.Error()),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), segments[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), segments[1])...)
}

func applyIPRestriction(model *IPRestrictionResourceModel, restriction *client.IPRestriction) {
	model.ID = types.StringValue(restriction.ID)
	model.ApplicationID = types.StringValue(restriction.AppID)
	model.IP = types.StringValue(restriction.IP)
	model.IPType = types.StringValue(restriction.IPType)
	model.Description = optionalString(restriction.Description)
}
