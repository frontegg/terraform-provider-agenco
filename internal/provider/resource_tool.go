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
	_ resource.Resource                = &ToolResource{}
	_ resource.ResourceWithImportState = &ToolResource{}
)

// toolAuthenticationTypes are the authentication modes an imported tool can use.
var toolAuthenticationTypes = []string{"Authenticated", "Unauthenticated"}

func NewToolResource() resource.Resource {
	return &ToolResource{}
}

// ToolResource manages the mutable settings of a tool that already exists, such as one
// created by an import. The API has no create route for individual tools.
type ToolResource struct {
	client *client.Client
}

// ToolResourceModel is the Terraform state for a managed tool override.
type ToolResourceModel struct {
	ID                 types.String `tfsdk:"id"`
	ApplicationID      types.String `tfsdk:"application_id"`
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	IsActive           types.Bool   `tfsdk:"is_active"`
	AuthenticationType types.String `tfsdk:"authentication_type"`
	SourceID           types.String `tfsdk:"source_id"`
	OriginalMethod     types.String `tfsdk:"original_method"`
	OriginalPath       types.String `tfsdk:"original_path"`
	OutputSchema       types.String `tfsdk:"output_schema"`
}

func (r *ToolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tool"
}

func (r *ToolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the mutable settings of an existing tool — typically one produced by " +
			"agenco_tools_import or an MCP proxy source. The API exposes no create route for individual " +
			"tools, so this resource adopts a tool by ID rather than creating it, and destroying the " +
			"resource deletes the tool. Use the agenco_tools data source to discover tool IDs.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "ID of the tool to manage. Immutable.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application the tool belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Tool name exposed to agents.",
				Optional:    true,
				Computed:    true,
			},
			"description": schema.StringAttribute{
				Description: "Tool description exposed to agents.",
				Optional:    true,
				Computed:    true,
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether the tool is exposed through the gateway.",
				Optional:    true,
				Computed:    true,
			},
			"authentication_type": schema.StringAttribute{
				Description: fmt.Sprintf("Authentication mode. One of: %v.", toolAuthenticationTypes),
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(toolAuthenticationTypes...),
				},
			},
			"source_id": schema.StringAttribute{
				Description: "Source the tool is attached to. Set it to move the tool between sources.",
				Optional:    true,
				Computed:    true,
			},
			"original_method": schema.StringAttribute{
				Description: "HTTP method of the upstream operation the tool calls.",
				Optional:    true,
				Computed:    true,
			},
			"original_path": schema.StringAttribute{
				Description: "Path of the upstream operation the tool calls.",
				Optional:    true,
				Computed:    true,
			},
			"output_schema": schema.StringAttribute{
				Description: "JSON Schema object describing the tool output, encoded as a JSON string.",
				Optional:    true,
			},
		},
	}
}

func (r *ToolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

// Create adopts an existing tool by applying the configured overrides to it.
func (r *ToolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ToolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	existing, err := r.client.GetTool(ctx, plan.ApplicationID.ValueString(), plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read tool", err.Error())
		return
	}
	if existing == nil {
		resp.Diagnostics.AddError(
			"Tool not found",
			fmt.Sprintf("No tool with ID %s exists in application %s. This resource manages tools that already "+
				"exist; import a schema with agenco_tools_import first.",
				plan.ID.ValueString(), plan.ApplicationID.ValueString()),
		)
		return
	}

	r.applyOverrides(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ToolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ToolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tool, err := r.client.GetTool(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read tool", err.Error())
		return
	}
	if tool == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	applyTool(&state, tool)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ToolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ToolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.applyOverrides(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ToolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ToolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteTool(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete tool", err.Error())
	}
}

// ImportState accepts "application_id/tool_id".
func (r *ToolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	segments, err := splitImportID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"application_id/tool_id\": %s", err.Error()),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), segments[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), segments[1])...)
}

// applyOverrides patches the tool then re-reads it, since the patch route returns no body.
func (r *ToolResource) applyOverrides(ctx context.Context, plan *ToolResourceModel, diagnostics *diag.Diagnostics) {
	request := client.UpdateToolRequest{
		AppID:              plan.ApplicationID.ValueString(),
		Name:               stringPointer(plan.Name),
		Description:        stringPointer(plan.Description),
		IsActive:           boolPointer(plan.IsActive),
		OriginalMethod:     stringPointer(plan.OriginalMethod),
		OriginalPath:       stringPointer(plan.OriginalPath),
		AuthenticationType: stringPointer(plan.AuthenticationType),
		SourceID:           stringPointer(plan.SourceID),
		OutputSchema:       jsonStringToMap(plan.OutputSchema, "output_schema", diagnostics),
	}
	if diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateTool(ctx, plan.ID.ValueString(), request); err != nil {
		diagnostics.AddError("Unable to update tool", err.Error())
		return
	}

	tool, err := r.client.GetTool(ctx, plan.ApplicationID.ValueString(), plan.ID.ValueString())
	if err != nil {
		diagnostics.AddError("Unable to read tool after update", err.Error())
		return
	}
	if tool == nil {
		diagnostics.AddError(
			"Tool disappeared after update",
			"The tool was updated but can no longer be read. Re-run the plan to reconcile.",
		)
		return
	}

	applyTool(plan, tool)
}

func applyTool(model *ToolResourceModel, tool *client.InternalTool) {
	model.ID = types.StringValue(tool.ID)
	model.Name = types.StringValue(tool.Name)
	model.Description = types.StringValue(tool.Description)
	model.IsActive = types.BoolValue(tool.IsActive)
	model.AuthenticationType = types.StringValue(tool.AuthenticationType)
	model.SourceID = types.StringValue(tool.SourceID)
	model.OriginalMethod = types.StringValue(tool.OriginalMethod)
	model.OriginalPath = types.StringValue(tool.OriginalPath)

	// output_schema is folded into the returned schema object rather than surfaced on its own,
	// so the configured value is kept instead of being reconstructed.
}
