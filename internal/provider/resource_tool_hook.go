package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
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
	_ resource.Resource                = &ToolHookResource{}
	_ resource.ResourceWithImportState = &ToolHookResource{}
)

// hookTypes are the gateway operations a pre-hook can intercept.
var hookTypes = []string{"LIST_TOOLS", "CALL_TOOL"}

// hookFailMethods decide what happens when the hook itself fails.
var hookFailMethods = []string{"OPEN", "CLOSE"}

const (
	minHookTimeoutSeconds = 5
	maxHookTimeoutSeconds = 10
)

func NewToolHookResource() resource.Resource {
	return &ToolHookResource{}
}

// ToolHookResource manages the pre-hook code run for an application on one gateway operation.
type ToolHookResource struct {
	client *client.Client
}

// ToolHookResourceModel is the Terraform state for a tool hook.
type ToolHookResourceModel struct {
	ID              types.String `tfsdk:"id"`
	ApplicationID   types.String `tfsdk:"application_id"`
	HookType        types.String `tfsdk:"hook_type"`
	Code            types.String `tfsdk:"code"`
	Runtime         types.String `tfsdk:"runtime"`
	FailMethod      types.String `tfsdk:"fail_method"`
	IsActive        types.Bool   `tfsdk:"is_active"`
	Timeout         types.Int64  `tfsdk:"timeout"`
	InternalToolIDs types.Set    `tfsdk:"internal_tool_ids"`
}

func (r *ToolHookResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tool_hook"
}

func (r *ToolHookResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the pre-hook code the gateway runs for an application. One hook exists per " +
			"application and hook type.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Hook ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application this hook belongs to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"hook_type": schema.StringAttribute{
				Description: fmt.Sprintf("Gateway operation the hook intercepts. One of: %v. Immutable after "+
					"create, because the API keys the hook on it.", hookTypes),
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(hookTypes...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"code": schema.StringAttribute{
				Description: "Hook source code, up to 100 KB.",
				Required:    true,
			},
			"runtime": schema.StringAttribute{
				Description: fmt.Sprintf("Execution runtime. One of: %v. Immutable after create, because the "+
					"API accepts it only on create. It is absent from the read route, so an imported hook "+
					"cannot recover it.", codeRuntimes),
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					createDefaultString("NODE_24"),
					requiresReplaceIfKnownString(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf(codeRuntimes...),
				},
			},
			"fail_method": schema.StringAttribute{
				Description: fmt.Sprintf("What happens when the hook errors. OPEN lets the call through, CLOSE "+
					"blocks it. One of: %v.", hookFailMethods),
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(hookFailMethods...),
				},
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether the hook runs.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(true),
				},
			},
			"timeout": schema.Int64Attribute{
				Description: fmt.Sprintf("Hook timeout in seconds, between %d and %d.",
					minHookTimeoutSeconds, maxHookTimeoutSeconds),
				Optional: true,
				Validators: []validator.Int64{
					int64validator.Between(minHookTimeoutSeconds, maxHookTimeoutSeconds),
				},
			},
			"internal_tool_ids": schema.SetAttribute{
				Description: "Tools the hook applies to. Required when hook_type is CALL_TOOL.",
				Optional:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *ToolHookResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *ToolHookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ToolHookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	toolIDs := r.readToolIDs(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	hook, err := r.client.CreateToolHook(ctx, client.CreateToolHookRequest{
		AppID:           plan.ApplicationID.ValueString(),
		IsActive:        plan.IsActive.ValueBool(),
		HookType:        plan.HookType.ValueString(),
		Code:            plan.Code.ValueString(),
		Runtime:         plan.Runtime.ValueString(),
		FailMethod:      plan.FailMethod.ValueString(),
		Timeout:         int64Pointer(plan.Timeout),
		InternalToolIDs: toolIDs,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create tool hook", err.Error())
		return
	}

	r.applyHook(ctx, &plan, hook, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ToolHookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ToolHookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hook, err := r.client.GetToolHook(ctx, state.ApplicationID.ValueString(), state.HookType.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read tool hook", err.Error())
		return
	}
	if hook == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyHook(ctx, &state, hook, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ToolHookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ToolHookResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	toolIDs := r.readToolIDs(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	code := plan.Code.ValueString()
	failMethod := plan.FailMethod.ValueString()
	hook, err := r.client.UpdateToolHook(ctx, client.UpdateToolHookRequest{
		AppID:           plan.ApplicationID.ValueString(),
		HookType:        plan.HookType.ValueString(),
		IsActive:        boolPointer(plan.IsActive),
		Code:            &code,
		FailMethod:      &failMethod,
		Timeout:         int64Pointer(plan.Timeout),
		InternalToolIDs: toolIDs,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update tool hook", err.Error())
		return
	}

	r.applyHook(ctx, &plan, hook, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ToolHookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ToolHookResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteToolHook(ctx, state.ApplicationID.ValueString(), state.HookType.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete tool hook", err.Error())
	}
}

// ImportState accepts "application_id/hook_type", since the hook is keyed on the operation.
func (r *ToolHookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	segments, err := splitImportID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"application_id/hook_type\": %s", err.Error()),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), segments[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("hook_type"), segments[1])...)

	resp.Diagnostics.AddWarning(
		"runtime cannot be recovered by import",
		"The prehook the API returns does not carry a runtime, so the configured value is written to "+
			"state on the next apply without being checked against the hook. That apply does not "+
			"replace the hook, but if the value differs from the runtime it was created with, state "+
			"will be wrong about it.",
	)
}

// readToolIDs resolves internal_tool_ids and enforces that CALL_TOOL hooks name their tools.
func (r *ToolHookResource) readToolIDs(
	ctx context.Context,
	plan ToolHookResourceModel,
	diagnostics *diag.Diagnostics,
) []string {
	toolIDs := stringSlice(ctx, plan.InternalToolIDs, diagnostics)
	if plan.HookType.ValueString() == "CALL_TOOL" && len(toolIDs) == 0 {
		diagnostics.AddError(
			"internal_tool_ids is required for CALL_TOOL hooks",
			"A CALL_TOOL hook must name the tools it applies to.",
		)
	}
	return toolIDs
}

func (r *ToolHookResource) applyHook(
	ctx context.Context,
	model *ToolHookResourceModel,
	hook *client.ToolHook,
	diagnostics *diag.Diagnostics,
) {
	model.ID = types.StringValue(hook.ID)
	model.HookType = types.StringValue(hook.HookType)
	model.IsActive = types.BoolValue(hook.Prehook.IsActive)

	if hook.Prehook.FailMethod != "" {
		model.FailMethod = types.StringValue(hook.Prehook.FailMethod)
	}
	if hook.Prehook.Timeout != nil {
		model.Timeout = types.Int64Value(*hook.Prehook.Timeout)
	}

	if len(hook.InternalToolIDs) == 0 {
		model.InternalToolIDs = types.SetNull(types.StringType)
	} else {
		model.InternalToolIDs = stringSetValue(ctx, hook.InternalToolIDs, diagnostics)
	}

	// The hook body lives in the prehook service and is not returned here, so code keeps its
	// configured value.
}
