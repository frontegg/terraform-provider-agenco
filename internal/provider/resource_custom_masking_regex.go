package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &CustomMaskingRegexResource{}
	_ resource.ResourceWithImportState = &CustomMaskingRegexResource{}
)

func NewCustomMaskingRegexResource() resource.Resource {
	return &CustomMaskingRegexResource{}
}

// CustomMaskingRegexResource manages a vendor-defined masking pattern.
type CustomMaskingRegexResource struct {
	client *client.Client
}

// CustomMaskingRegexResourceModel is the Terraform state for a custom masking regex.
type CustomMaskingRegexResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	Name          types.String `tfsdk:"name"`
	Pattern       types.String `tfsdk:"pattern"`
	Flags         types.String `tfsdk:"flags"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	Description   types.String `tfsdk:"description"`
}

func (r *CustomMaskingRegexResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_masking_regex"
}

func (r *CustomMaskingRegexResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a custom masking regex. Reference its ID from a masking policy's " +
			"custom_masking_regex_ids to apply it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Custom masking regex ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application this regex belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Regex name.",
				Required:    true,
			},
			"pattern": schema.StringAttribute{
				Description: "Regular expression used to detect the value to mask.",
				Required:    true,
			},
			"flags": schema.StringAttribute{
				Description: "Regular expression flags, up to 4 characters, for example \"gi\".",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(4),
				},
			},
			"enabled": schema.BoolAttribute{
				Description: "Whether the regex is applied.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"description": schema.StringAttribute{
				Description: "Free-text description of what the regex matches.",
				Optional:    true,
			},
		},
	}
}

func (r *CustomMaskingRegexResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *CustomMaskingRegexResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CustomMaskingRegexResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := client.CustomMaskingRegexRequest{
		AppID:       plan.ApplicationID.ValueString(),
		Name:        stringPointer(plan.Name),
		Pattern:     stringPointer(plan.Pattern),
		Flags:       stringPointer(plan.Flags),
		Enabled:     boolPointer(plan.Enabled),
		Description: stringPointer(plan.Description),
	}

	regex, err := r.client.CreateMaskingRegex(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create custom masking regex", err.Error())
		return
	}

	applyMaskingRegex(&plan, regex)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CustomMaskingRegexResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CustomMaskingRegexResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	regex, err := r.client.GetMaskingRegex(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read custom masking regex", err.Error())
		return
	}
	if regex == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	applyMaskingRegex(&state, regex)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CustomMaskingRegexResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CustomMaskingRegexResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := client.CustomMaskingRegexRequest{
		Name:        stringPointer(plan.Name),
		Pattern:     stringPointer(plan.Pattern),
		Flags:       stringPointer(plan.Flags),
		Enabled:     boolPointer(plan.Enabled),
		Description: stringPointer(plan.Description),
	}

	regex, err := r.client.UpdateMaskingRegex(ctx, plan.ApplicationID.ValueString(), plan.ID.ValueString(), request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update custom masking regex", err.Error())
		return
	}

	applyMaskingRegex(&plan, regex)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CustomMaskingRegexResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CustomMaskingRegexResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteMaskingRegex(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete custom masking regex", err.Error())
	}
}

// ImportState accepts "application_id/regex_id".
func (r *CustomMaskingRegexResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	segments, err := splitImportID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"application_id/regex_id\": %s", err.Error()),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), segments[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), segments[1])...)
}

func applyMaskingRegex(model *CustomMaskingRegexResourceModel, regex *client.CustomMaskingRegex) {
	model.ID = types.StringValue(regex.ID)
	model.ApplicationID = types.StringValue(regex.AppID)
	model.Name = types.StringValue(regex.Name)
	model.Pattern = types.StringValue(regex.Pattern)
	model.Enabled = types.BoolValue(regex.Enabled)
	model.Flags = optionalString(&regex.Flags)
	model.Description = optionalString(regex.Description)
}
