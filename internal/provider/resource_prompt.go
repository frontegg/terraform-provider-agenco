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

var (
	_ resource.Resource                = &PromptResource{}
	_ resource.ResourceWithImportState = &PromptResource{}
)

func NewPromptResource() resource.Resource {
	return &PromptResource{}
}

// PromptResource manages a reusable prompt served alongside an application's tools.
type PromptResource struct {
	client *client.Client
}

// PromptResourceModel is the Terraform state for a prompt.
type PromptResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	Name          types.String `tfsdk:"name"`
	Prompt        types.String `tfsdk:"prompt"`
	ToolIDs       types.Set    `tfsdk:"tool_ids"`
}

func (r *PromptResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_prompt"
}

func (r *PromptResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a prompt exposed to agents through the MCP gateway.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Prompt ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application this prompt belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Prompt name.",
				Required:    true,
			},
			"prompt": schema.StringAttribute{
				Description: "Prompt body.",
				Required:    true,
			},
			"tool_ids": schema.SetAttribute{
				Description: "Internal tool IDs this prompt is connected to.",
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *PromptResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *PromptResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan PromptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := client.CreatePromptRequest{
		AppID:   plan.ApplicationID.ValueString(),
		Name:    plan.Name.ValueString(),
		Prompt:  plan.Prompt.ValueString(),
		ToolIDs: stringSlice(ctx, plan.ToolIDs, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	prompt, err := r.client.CreatePrompt(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create prompt", err.Error())
		return
	}

	applyPrompt(ctx, &plan, prompt, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PromptResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state PromptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	prompt, err := r.client.GetPrompt(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read prompt", err.Error())
		return
	}
	if prompt == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	applyPrompt(ctx, &state, prompt, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *PromptResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan PromptResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	promptBody := plan.Prompt.ValueString()
	request := client.UpdatePromptRequest{
		Name:    &name,
		Prompt:  &promptBody,
		ToolIDs: stringSlice(ctx, plan.ToolIDs, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	appID := plan.ApplicationID.ValueString()
	if err := r.client.UpdatePrompt(ctx, appID, plan.ID.ValueString(), request); err != nil {
		resp.Diagnostics.AddError("Unable to update prompt", err.Error())
		return
	}

	prompt, err := r.client.GetPrompt(ctx, appID, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read prompt after update", err.Error())
		return
	}
	if prompt == nil {
		resp.Diagnostics.AddError(
			"Prompt disappeared after update",
			"The prompt was updated but can no longer be read. Re-run the plan to reconcile.",
		)
		return
	}

	applyPrompt(ctx, &plan, prompt, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *PromptResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state PromptResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeletePrompt(ctx, state.ApplicationID.ValueString(), state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete prompt", err.Error())
	}
}

// ImportState accepts "application_id/prompt_id".
func (r *PromptResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	segments, err := splitImportID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"application_id/prompt_id\": %s", err.Error()),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), segments[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), segments[1])...)
}

func applyPrompt(ctx context.Context, model *PromptResourceModel, prompt *client.Prompt, diagnostics *diag.Diagnostics) {
	model.ID = types.StringValue(prompt.ID)
	model.ApplicationID = types.StringValue(prompt.AppID)
	model.Name = types.StringValue(prompt.Name)
	model.Prompt = types.StringValue(prompt.Prompt)

	// The API returns connected tools as "connections"; an empty list reads back as null so an
	// unset tool_ids attribute stays stable across plans.
	if len(prompt.Connections) == 0 {
		model.ToolIDs = types.SetNull(types.StringType)
		return
	}
	model.ToolIDs = stringSetValue(ctx, prompt.Connections, diagnostics)
}
