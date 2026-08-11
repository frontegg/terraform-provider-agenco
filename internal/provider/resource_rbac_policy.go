package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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
	_ resource.Resource                = &RbacPolicyResource{}
	_ resource.ResourceWithImportState = &RbacPolicyResource{}
)

// rbacPolicyTypes select whether keys are matched against roles or permissions.
var rbacPolicyTypes = []string{"rbac-roles", "rbac-permissions"}

func NewRbacPolicyResource() resource.Resource {
	return &RbacPolicyResource{}
}

// RbacPolicyResource manages a role or permission based access policy.
type RbacPolicyResource struct {
	client *client.Client
}

// RbacPolicyResourceModel is the Terraform state for an RBAC policy.
type RbacPolicyResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Description            types.String `tfsdk:"description"`
	Enabled                types.Bool   `tfsdk:"enabled"`
	ApplicationIDs         types.Set    `tfsdk:"application_ids"`
	TenantID               types.String `tfsdk:"tenant_id"`
	Slug                   types.String `tfsdk:"slug"`
	Type                   types.String `tfsdk:"type"`
	Keys                   types.Set    `tfsdk:"keys"`
	InternalToolIDs        types.Set    `tfsdk:"internal_tool_ids"`
	CustomCodeToolIDs      types.Set    `tfsdk:"custom_code_tool_ids"`
	CustomIntegrationTools types.Set    `tfsdk:"custom_integration_tools"`
}

func (r *RbacPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_rbac_policy"
}

func (r *RbacPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Description: "Policy ID.",
			Computed:    true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"name": schema.StringAttribute{
			Description: "Policy name.",
			Required:    true,
		},
		"description": schema.StringAttribute{
			Description: "Free-text description of what the policy grants.",
			Optional:    true,
		},
		"enabled": schema.BoolAttribute{
			Description: "Whether the policy is evaluated. Not returned by the API's read route, so the " +
				"configured value is kept in state.",
			Required: true,
		},
		"application_ids": schema.SetAttribute{
			Description: "Applications the policy applies to. Not returned by the API's read route, so the " +
				"configured value is kept in state.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"tenant_id": schema.StringAttribute{
			Description: "Restrict the policy to a single tenant. Omit for a vendor-level policy.",
			Optional:    true,
		},
		"slug": schema.StringAttribute{
			Description: "Restrict the policy to one connector instance by slug.",
			Optional:    true,
			Validators: []validator.String{
				stringvalidator.LengthAtMost(64),
				stringvalidator.RegexMatches(slugPattern, slugValidationMessage),
			},
		},
		"type": schema.StringAttribute{
			Description: fmt.Sprintf("Whether keys are roles or permissions. One of: %v.", rbacPolicyTypes),
			Required:    true,
			Validators: []validator.String{
				stringvalidator.OneOf(rbacPolicyTypes...),
			},
		},
		"keys": schema.SetAttribute{
			Description: "Role or permission keys granted access to the selected tools.",
			Required:    true,
			ElementType: types.StringType,
			Validators: []validator.Set{
				setvalidator.SizeAtLeast(1),
			},
		},
	}
	for name, attribute := range toolTargetAttributes(false) {
		attributes[name] = attribute
	}

	resp.Schema = schema.Schema{
		Description: "Manages an RBAC policy granting tool access to a set of roles or permissions. At least " +
			"one of internal_tool_ids, custom_code_tool_ids or custom_integration_tools is required.",
		Attributes: attributes,
	}
}

func (r *RbacPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *RbacPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RbacPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := r.buildRequest(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.CreateRbacPolicy(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create RBAC policy", err.Error())
		return
	}

	policy, err := r.client.GetRbacPolicy(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read RBAC policy after create", err.Error())
		return
	}
	if policy == nil {
		resp.Diagnostics.AddError(
			"RBAC policy not found after create",
			"The policy was created but cannot be read back. Re-run the plan to reconcile.",
		)
		return
	}

	r.applyPolicy(ctx, &plan, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RbacPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RbacPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy, err := r.client.GetRbacPolicy(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read RBAC policy", err.Error())
		return
	}
	if policy == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyPolicy(ctx, &state, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *RbacPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RbacPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := r.buildRequest(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateRbacPolicy(ctx, plan.ID.ValueString(), request); err != nil {
		resp.Diagnostics.AddError("Unable to update RBAC policy", err.Error())
		return
	}

	policy, err := r.client.GetRbacPolicy(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read RBAC policy after update", err.Error())
		return
	}
	if policy == nil {
		resp.Diagnostics.AddError(
			"RBAC policy not found after update",
			"The policy was updated but cannot be read back. Re-run the plan to reconcile.",
		)
		return
	}

	r.applyPolicy(ctx, &plan, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RbacPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RbacPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeletePolicy(ctx, state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete RBAC policy", err.Error())
	}
}

func (r *RbacPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *RbacPolicyResource) buildRequest(
	ctx context.Context,
	plan RbacPolicyResourceModel,
	diagnostics *diag.Diagnostics,
) client.RbacPolicyRequest {
	internalTools := stringSlice(ctx, plan.InternalToolIDs, diagnostics)
	customCodeTools := stringSlice(ctx, plan.CustomCodeToolIDs, diagnostics)
	integrationTools := stringSlice(ctx, plan.CustomIntegrationTools, diagnostics)

	if len(internalTools) == 0 && len(customCodeTools) == 0 && len(integrationTools) == 0 {
		diagnostics.AddError(
			"At least one tool target is required",
			"Set at least one of internal_tool_ids, custom_code_tool_ids or custom_integration_tools.",
		)
	}

	keys := stringSlice(ctx, plan.Keys, diagnostics)
	if diagnostics.HasError() {
		return client.RbacPolicyRequest{}
	}

	return client.RbacPolicyRequest{
		AppIDs:                 stringSlice(ctx, plan.ApplicationIDs, diagnostics),
		Name:                   plan.Name.ValueString(),
		Description:            stringPointer(plan.Description),
		Enabled:                plan.Enabled.ValueBool(),
		Slug:                   stringPointer(plan.Slug),
		TenantID:               stringPointer(plan.TenantID),
		Type:                   plan.Type.ValueString(),
		Keys:                   keys,
		InternalToolIDs:        internalTools,
		CustomCodeToolIDs:      customCodeTools,
		CustomIntegrationTools: integrationTools,
	}
}

func (r *RbacPolicyResource) applyPolicy(
	ctx context.Context,
	model *RbacPolicyResourceModel,
	policy *client.Policy,
	diagnostics *diag.Diagnostics,
) {
	model.ID = types.StringValue(policy.ID)
	model.Name = types.StringValue(policy.Name)
	model.Description = optionalString(policy.Description)
	model.TenantID = optionalString(policy.TenantID)
	model.Slug = optionalString(policy.Slug)
	model.Keys = nullIfEmptySet(ctx, policy.Keys, diagnostics)
	model.InternalToolIDs = nullIfEmptySet(ctx, policy.InternalToolIDs, diagnostics)
	model.CustomCodeToolIDs = nullIfEmptySet(ctx, policy.CustomCodeToolIDs, diagnostics)
	model.CustomIntegrationTools = nullIfEmptySet(ctx, policy.CustomIntegrationTools, diagnostics)

	// The RBAC read route returns neither enabled nor appIds — GetRbacPolicyResponse projects
	// only the shared fields plus keys. Those two keep their configured values, so drift made
	// outside Terraform in either field will not be detected.
	if policy.Type != "" {
		model.Type = types.StringValue(policy.Type)
	}
}
