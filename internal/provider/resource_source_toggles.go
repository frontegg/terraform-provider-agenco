package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---- Frontegg tenant tools source ----

var (
	_ resource.Resource                = &FronteggToolsSourceResource{}
	_ resource.ResourceWithImportState = &FronteggToolsSourceResource{}
)

func NewFronteggToolsSourceResource() resource.Resource {
	return &FronteggToolsSourceResource{}
}

// FronteggToolsSourceResource toggles the built-in Frontegg tenant tools source.
type FronteggToolsSourceResource struct {
	client *client.Client
}

// FronteggToolsSourceResourceModel is the Terraform state for the Frontegg tools source.
type FronteggToolsSourceResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	IsActive      types.Bool   `tfsdk:"is_active"`
	Name          types.String `tfsdk:"name"`
	SourceURL     types.String `tfsdk:"source_url"`
}

func (r *FronteggToolsSourceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_frontegg_tools_source"
}

func (r *FronteggToolsSourceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Enables or disables the built-in Frontegg tenant tools source for an application. " +
			"Frontegg provisions the source itself; this resource only controls whether it is active. " +
			"Destroying the resource disables the source.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Source ID assigned by Frontegg.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application the source belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether the Frontegg tenant tools source is enabled.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "Source name assigned by Frontegg.",
				Computed:    true,
			},
			"source_url": schema.StringAttribute{
				Description: "Source URL assigned by Frontegg.",
				Computed:    true,
			},
		},
	}
}

func (r *FronteggToolsSourceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *FronteggToolsSourceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FronteggToolsSourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	source, err := r.client.ToggleFronteggTenantToolsSource(
		ctx, plan.ApplicationID.ValueString(), plan.IsActive.ValueBool(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to toggle Frontegg tools source", err.Error())
		return
	}

	plan.ID = types.StringValue(source.ID)
	plan.Name = types.StringValue(source.Name)
	plan.SourceURL = types.StringValue(source.SourceURL)
	plan.IsActive = types.BoolValue(source.Enabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FronteggToolsSourceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FronteggToolsSourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	source, err := r.client.GetSource(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Frontegg tools source", err.Error())
		return
	}
	if source == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Name = types.StringValue(source.Name)
	state.SourceURL = types.StringValue(source.SourceURL)
	state.IsActive = types.BoolValue(source.Enabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *FronteggToolsSourceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FronteggToolsSourceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	source, err := r.client.ToggleFronteggTenantToolsSource(
		ctx, plan.ApplicationID.ValueString(), plan.IsActive.ValueBool(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to toggle Frontegg tools source", err.Error())
		return
	}

	plan.ID = types.StringValue(source.ID)
	plan.Name = types.StringValue(source.Name)
	plan.SourceURL = types.StringValue(source.SourceURL)
	plan.IsActive = types.BoolValue(source.Enabled)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete disables the source rather than removing it, since Frontegg owns its lifecycle.
func (r *FronteggToolsSourceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FronteggToolsSourceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.ToggleFronteggTenantToolsSource(ctx, state.ApplicationID.ValueString(), false)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to disable Frontegg tools source", err.Error())
	}
}

func (r *FronteggToolsSourceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

// ---- Bulk tool activation for a source ----

var _ resource.Resource = &SourceToolsActiveStatusResource{}

func NewSourceToolsActiveStatusResource() resource.Resource {
	return &SourceToolsActiveStatusResource{}
}

// SourceToolsActiveStatusResource enables or disables every tool imported from a source.
type SourceToolsActiveStatusResource struct {
	client *client.Client
}

// SourceToolsActiveStatusResourceModel is the Terraform state for the bulk activation call.
type SourceToolsActiveStatusResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	SourceID      types.String `tfsdk:"source_id"`
	IsActive      types.Bool   `tfsdk:"is_active"`
	UpdatedCount  types.Int64  `tfsdk:"updated_count"`
}

func (r *SourceToolsActiveStatusResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_source_tools_active_status"
}

func (r *SourceToolsActiveStatusResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Sets the active status of every tool imported from one MCP source. This wraps a bulk " +
			"action rather than a stored object: the API offers no way to read the aggregate back, so the " +
			"desired state is re-applied on change and destroying the resource is a no-op. Per-tool control " +
			"lives in agenco_tool.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Synthetic ID, formed as application_id/source_id.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application the source belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"source_id": schema.StringAttribute{
				Description: "Source whose tools are toggled.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether every tool from the source is enabled.",
				Required:    true,
			},
			"updated_count": schema.Int64Attribute{
				Description: "Number of tools changed by the last apply.",
				Computed:    true,
			},
		},
	}
}

func (r *SourceToolsActiveStatusResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *SourceToolsActiveStatusResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan SourceToolsActiveStatusResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read is a no-op: the API has no route returning the aggregate status of a source's tools.
func (r *SourceToolsActiveStatusResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state SourceToolsActiveStatusResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *SourceToolsActiveStatusResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan SourceToolsActiveStatusResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete leaves the tools as they are; the underlying call has no inverse.
func (r *SourceToolsActiveStatusResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func (r *SourceToolsActiveStatusResource) apply(
	ctx context.Context,
	plan *SourceToolsActiveStatusResourceModel,
	diagnostics *diag.Diagnostics,
) {
	count, err := r.client.SetSourceToolsActiveStatus(
		ctx, plan.ApplicationID.ValueString(), plan.SourceID.ValueString(), plan.IsActive.ValueBool(),
	)
	if err != nil {
		diagnostics.AddError("Unable to set tool active status", err.Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s/%s", plan.ApplicationID.ValueString(), plan.SourceID.ValueString()))
	plan.UpdatedCount = types.Int64Value(count)
}
