package provider

import (
	"context"

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
	_ resource.Resource                = &ConditionalPolicyResource{}
	_ resource.ResourceWithImportState = &ConditionalPolicyResource{}
)

func NewConditionalPolicyResource() resource.Resource {
	return &ConditionalPolicyResource{}
}

// ConditionalPolicyResource manages a conditional access policy.
type ConditionalPolicyResource struct {
	client *client.Client
}

// ConditionalPolicyResourceModel is the Terraform state for a conditional policy.
type ConditionalPolicyResourceModel struct {
	ID                     types.String           `tfsdk:"id"`
	Name                   types.String           `tfsdk:"name"`
	Description            types.String           `tfsdk:"description"`
	Enabled                types.Bool             `tfsdk:"enabled"`
	ApplicationIDs         types.Set              `tfsdk:"application_ids"`
	TenantID               types.String           `tfsdk:"tenant_id"`
	Slug                   types.String           `tfsdk:"slug"`
	InternalToolIDs        types.Set              `tfsdk:"internal_tool_ids"`
	CustomCodeToolIDs      types.Set              `tfsdk:"custom_code_tool_ids"`
	CustomIntegrationTools types.Set              `tfsdk:"custom_integration_tools"`
	AllowToAllUsers        types.Bool             `tfsdk:"allow_to_all_users"`
	Metadata               types.Map              `tfsdk:"metadata"`
	Targeting              []PolicyTargetingModel `tfsdk:"targeting"`
}

func (r *ConditionalPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_conditional_policy"
}

func (r *ConditionalPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			Description: "Free-text description of what the policy does.",
			Optional:    true,
		},
		"enabled": schema.BoolAttribute{
			Description: "Whether the policy is evaluated. A disabled policy may omit targeting.",
			Required:    true,
		},
		"application_ids": schema.SetAttribute{
			Description: "Applications the policy applies to. Omit to apply across the vendor.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"tenant_id": schema.StringAttribute{
			Description: "Restrict the policy to a single tenant. Omit for a vendor-level policy.",
			Optional:    true,
		},
		"slug": schema.StringAttribute{
			Description: "Restrict the policy to one connector instance by slug. Omit to cover every instance.",
			Optional:    true,
			Validators: []validator.String{
				stringvalidator.LengthAtMost(64),
				stringvalidator.RegexMatches(slugPattern, slugValidationMessage),
			},
		},
		"allow_to_all_users": schema.BoolAttribute{
			Description: "Whether the policy result applies to all users.",
			Optional:    true,
			Computed:    true,
			Default:     booldefault.StaticBool(false),
		},
		"metadata": schema.MapAttribute{
			Description: "Free-form string metadata stored alongside the policy.",
			Optional:    true,
			ElementType: types.StringType,
		},
	}
	for name, attribute := range toolTargetAttributes(true) {
		attributes[name] = attribute
	}

	resp.Schema = schema.Schema{
		Description: "Manages a conditional access policy — an if/then rule deciding whether a tool call is " +
			"allowed, denied, stepped up or sent for approval.",
		Attributes: attributes,
		Blocks: map[string]schema.Block{
			"targeting": targetingBlock(),
		},
	}
}

func (r *ConditionalPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *ConditionalPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ConditionalPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := r.buildRequest(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.CreateConditionalPolicy(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create conditional policy", err.Error())
		return
	}

	policy, err := r.client.GetConditionalPolicy(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read conditional policy after create", err.Error())
		return
	}
	if policy == nil {
		resp.Diagnostics.AddError(
			"Conditional policy not found after create",
			"The policy was created but cannot be read back. Re-run the plan to reconcile.",
		)
		return
	}

	r.applyPolicy(ctx, &plan, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ConditionalPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ConditionalPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy, err := r.client.GetConditionalPolicy(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read conditional policy", err.Error())
		return
	}
	if policy == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyPolicy(ctx, &state, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ConditionalPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ConditionalPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := r.buildRequest(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateConditionalPolicy(ctx, plan.ID.ValueString(), request); err != nil {
		resp.Diagnostics.AddError("Unable to update conditional policy", err.Error())
		return
	}

	policy, err := r.client.GetConditionalPolicy(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read conditional policy after update", err.Error())
		return
	}
	if policy == nil {
		resp.Diagnostics.AddError(
			"Conditional policy not found after update",
			"The policy was updated but cannot be read back. Re-run the plan to reconcile.",
		)
		return
	}

	r.applyPolicy(ctx, &plan, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ConditionalPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ConditionalPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeletePolicy(ctx, state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete conditional policy", err.Error())
	}
}

func (r *ConditionalPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ConditionalPolicyResource) buildRequest(
	ctx context.Context,
	plan ConditionalPolicyResourceModel,
	diagnostics *diag.Diagnostics,
) client.ConditionalPolicyRequest {
	internalTools, customCodeTools, integrationTools := readToolTargets(ctx, policyToolTargets{
		InternalToolIDs:        plan.InternalToolIDs,
		CustomCodeToolIDs:      plan.CustomCodeToolIDs,
		CustomIntegrationTools: plan.CustomIntegrationTools,
	}, diagnostics)

	metadata := stringMap(ctx, plan.Metadata, diagnostics)
	var metadataPayload map[string]interface{}
	if metadata != nil {
		metadataPayload = map[string]interface{}{}
		for key, value := range metadata {
			metadataPayload[key] = value
		}
	}

	return client.ConditionalPolicyRequest{
		AppIDs:                 stringSlice(ctx, plan.ApplicationIDs, diagnostics),
		Name:                   plan.Name.ValueString(),
		Description:            stringPointer(plan.Description),
		Enabled:                plan.Enabled.ValueBool(),
		Slug:                   stringPointer(plan.Slug),
		TenantID:               stringPointer(plan.TenantID),
		InternalToolIDs:        internalTools,
		CustomCodeToolIDs:      customCodeTools,
		CustomIntegrationTools: integrationTools,
		AllowToAllUsers:        boolPointer(plan.AllowToAllUsers),
		Targeting:              buildTargeting(plan.Targeting, diagnostics),
		Metadata:               metadataPayload,
	}
}

func (r *ConditionalPolicyResource) applyPolicy(
	ctx context.Context,
	model *ConditionalPolicyResourceModel,
	policy *client.Policy,
	diagnostics *diag.Diagnostics,
) {
	model.ID = types.StringValue(policy.ID)
	model.Name = types.StringValue(policy.Name)
	model.Enabled = types.BoolValue(policy.Enabled)
	model.AllowToAllUsers = types.BoolValue(policy.AllowToAllUsers)
	model.Description = optionalString(policy.Description)
	model.TenantID = optionalString(policy.TenantID)
	model.Slug = optionalString(policy.Slug)
	model.ApplicationIDs = nullIfEmptySet(ctx, policy.AppIDs, diagnostics)
	model.InternalToolIDs = stringSetValue(ctx, orEmpty(policy.InternalToolIDs), diagnostics)
	model.CustomCodeToolIDs = nullIfEmptySet(ctx, policy.CustomCodeToolIDs, diagnostics)
	model.CustomIntegrationTools = nullIfEmptySet(ctx, policy.CustomIntegrationTools, diagnostics)
	model.Targeting = flattenTargeting(policy.Targeting, diagnostics)

	if len(policy.Metadata) == 0 {
		model.Metadata = types.MapNull(types.StringType)
	} else {
		metadata := map[string]string{}
		for key, value := range policy.Metadata {
			if stringValue, ok := value.(string); ok {
				metadata[key] = stringValue
			}
		}
		model.Metadata = stringMapValue(ctx, metadata, diagnostics)
	}
}

// orEmpty keeps internal_tool_ids a set rather than null, since the API treats an empty
// list as "all tools" and always returns it.
func orEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
