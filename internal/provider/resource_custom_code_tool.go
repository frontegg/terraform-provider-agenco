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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &CustomCodeToolResource{}
	_ resource.ResourceWithImportState = &CustomCodeToolResource{}
)

// codeRuntimes are the execution runtimes the API accepts for vendor-supplied code.
var codeRuntimes = []string{"NODE_20", "NODE_24"}

func NewCustomCodeToolResource() resource.Resource {
	return &CustomCodeToolResource{}
}

// CustomCodeToolResource manages a tool backed by vendor-supplied code.
type CustomCodeToolResource struct {
	client *client.Client
}

// CustomCodeToolResourceModel is the Terraform state for a custom code tool.
type CustomCodeToolResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	ApplicationID         types.String `tfsdk:"application_id"`
	Name                  types.String `tfsdk:"name"`
	Description           types.String `tfsdk:"description"`
	CodeContent           types.String `tfsdk:"code_content"`
	Runtime               types.String `tfsdk:"runtime"`
	InputSchema           types.String `tfsdk:"input_schema"`
	IsActive              types.Bool   `tfsdk:"is_active"`
	AttachedIntegrationID types.String `tfsdk:"attached_integration_id"`
	Scopes                types.List   `tfsdk:"scopes"`
	CustomCodeID          types.String `tfsdk:"custom_code_id"`
}

func (r *CustomCodeToolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_code_tool"
}

func (r *CustomCodeToolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a custom code tool — a tool whose body is code you supply rather than an upstream API call.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Custom code tool ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application this tool belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Tool name. Alphanumeric plus underscores, dashes and periods.",
				Required:    true,
			},
			"description": schema.StringAttribute{
				Description: "Tool description shown to agents.",
				Optional:    true,
			},
			"code_content": schema.StringAttribute{
				Description: "Source code executed when the tool is called. Up to 100 KB.",
				Required:    true,
			},
			"runtime": schema.StringAttribute{
				Description: fmt.Sprintf("Execution runtime. One of: %v. Immutable after create.", codeRuntimes),
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(codeRuntimes...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"input_schema": schema.StringAttribute{
				Description: "JSON Schema object describing the tool inputs, encoded as a JSON string.",
				Required:    true,
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether the tool is exposed through the gateway.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"attached_integration_id": schema.StringAttribute{
				Description: "Custom integration whose credentials this tool authenticates with.",
				Optional:    true,
			},
			"scopes": schema.ListAttribute{
				Description: "OAuth scopes merged with the attached integration's scopes.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"custom_code_id": schema.StringAttribute{
				Description: "ID of the stored code revision backing this tool.",
				Computed:    true,
			},
		},
	}
}

func (r *CustomCodeToolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *CustomCodeToolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CustomCodeToolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := client.CreateCustomCodeToolRequest{
		AppID:                 plan.ApplicationID.ValueString(),
		Name:                  plan.Name.ValueString(),
		Description:           stringPointer(plan.Description),
		CodeContent:           plan.CodeContent.ValueString(),
		Runtime:               plan.Runtime.ValueString(),
		InputSchema:           jsonStringToMap(plan.InputSchema, "input_schema", &resp.Diagnostics),
		AttachedIntegrationID: stringPointer(plan.AttachedIntegrationID),
		Scopes:                stringSlice(ctx, plan.Scopes, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	tool, err := r.client.CreateCustomCodeTool(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create custom code tool", err.Error())
		return
	}

	// is_active is not accepted on create; apply it with a follow-up patch when it differs.
	if !plan.IsActive.IsNull() && !plan.IsActive.ValueBool() {
		isActive := false
		tool, err = r.client.UpdateCustomCodeTool(ctx, tool.ID, client.UpdateCustomCodeToolRequest{
			AppID:    plan.ApplicationID.ValueString(),
			IsActive: &isActive,
		})
		if err != nil {
			resp.Diagnostics.AddError("Unable to deactivate custom code tool after create", err.Error())
			return
		}
	}

	r.applyTool(ctx, &plan, tool, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CustomCodeToolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CustomCodeToolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tool, err := r.client.GetCustomCodeTool(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read custom code tool", err.Error())
		return
	}
	if tool == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyTool(ctx, &state, tool, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *CustomCodeToolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan CustomCodeToolResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	codeContent := plan.CodeContent.ValueString()
	request := client.UpdateCustomCodeToolRequest{
		AppID:                 plan.ApplicationID.ValueString(),
		Name:                  &name,
		Description:           stringPointer(plan.Description),
		CodeContent:           &codeContent,
		InputSchema:           jsonStringToMap(plan.InputSchema, "input_schema", &resp.Diagnostics),
		IsActive:              boolPointer(plan.IsActive),
		AttachedIntegrationID: stringPointer(plan.AttachedIntegrationID),
		Scopes:                stringSlice(ctx, plan.Scopes, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	tool, err := r.client.UpdateCustomCodeTool(ctx, plan.ID.ValueString(), request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update custom code tool", err.Error())
		return
	}

	r.applyTool(ctx, &plan, tool, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *CustomCodeToolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CustomCodeToolResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteCustomCodeTool(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete custom code tool", err.Error())
	}
}

// ImportState accepts "application_id/tool_id".
func (r *CustomCodeToolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

func (r *CustomCodeToolResource) applyTool(
	ctx context.Context,
	model *CustomCodeToolResourceModel,
	tool *client.CustomCodeTool,
	diagnostics *diag.Diagnostics,
) {
	model.ID = types.StringValue(tool.ID)
	model.ApplicationID = types.StringValue(tool.AppID)
	model.Name = types.StringValue(tool.Name)
	model.IsActive = types.BoolValue(tool.IsActive)
	model.CustomCodeID = types.StringValue(tool.CustomCodeID)
	model.Description = optionalString(&tool.Description)
	model.AttachedIntegrationID = optionalString(tool.AttachedIntegrationID)

	if len(tool.Scopes) == 0 {
		model.Scopes = types.ListNull(types.StringType)
	} else {
		model.Scopes = stringListValue(ctx, tool.Scopes, diagnostics)
	}

	// code_content is never returned (it lives in the custom-code service) and the returned
	// schema is the input schema augmented with server-side fields, so neither round-trips.
	// Both keep their configured values rather than reporting a permanent diff.
}
