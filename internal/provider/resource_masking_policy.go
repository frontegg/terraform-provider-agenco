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
	_ resource.Resource                = &MaskingPolicyResource{}
	_ resource.ResourceWithImportState = &MaskingPolicyResource{}
)

func NewMaskingPolicyResource() resource.Resource {
	return &MaskingPolicyResource{}
}

// MaskingPolicyResource manages a data-masking policy.
type MaskingPolicyResource struct {
	client *client.Client
}

// MaskingPolicyResourceModel is the Terraform state for a masking policy.
type MaskingPolicyResourceModel struct {
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
	Detectors              types.Set              `tfsdk:"detectors"`
	CustomMaskingRegexIDs  types.Set              `tfsdk:"custom_masking_regex_ids"`
	Targeting              []PolicyTargetingModel `tfsdk:"targeting"`
}

func (r *MaskingPolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_masking_policy"
}

func (r *MaskingPolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
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
			Description: "Free-text description of what the policy masks.",
			Optional:    true,
		},
		"enabled": schema.BoolAttribute{
			Description: "Whether the policy is evaluated.",
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
			Description: "Restrict the policy to one connector instance by slug.",
			Optional:    true,
			Validators: []validator.String{
				stringvalidator.LengthAtMost(64),
				stringvalidator.RegexMatches(slugPattern, slugValidationMessage),
			},
		},
		"detectors": schema.SetAttribute{
			Description: fmt.Sprintf("Built-in PII detectors to enable, for example credit_card, email_address "+
				"or us_ssn. %d detectors are supported; see the resource documentation for the full list.",
				len(client.MaskingDetectorNames())),
			Optional:    true,
			ElementType: types.StringType,
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(stringvalidator.OneOf(client.MaskingDetectorNames()...)),
			},
		},
		"custom_masking_regex_ids": schema.SetAttribute{
			Description: "IDs of agenco_custom_masking_regex resources to apply in addition to the detectors.",
			Optional:    true,
			ElementType: types.StringType,
		},
	}
	for name, attribute := range toolTargetAttributes(true) {
		attributes[name] = attribute
	}

	resp.Schema = schema.Schema{
		Description: "Manages a data-masking policy. Enable built-in PII detectors by name and reference " +
			"custom masking regexes by ID.",
		Attributes: attributes,
		Blocks: map[string]schema.Block{
			"targeting": targetingBlock(),
		},
	}
}

func (r *MaskingPolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *MaskingPolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MaskingPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := r.buildRequest(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := r.client.CreateMaskingPolicy(ctx, request)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create masking policy", err.Error())
		return
	}

	policy, err := r.client.GetMaskingPolicy(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read masking policy after create", err.Error())
		return
	}
	if policy == nil {
		resp.Diagnostics.AddError(
			"Masking policy not found after create",
			"The policy was created but cannot be read back. Re-run the plan to reconcile.",
		)
		return
	}

	r.applyPolicy(ctx, &plan, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MaskingPolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state MaskingPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy, err := r.client.GetMaskingPolicy(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read masking policy", err.Error())
		return
	}
	if policy == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyPolicy(ctx, &state, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *MaskingPolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan MaskingPolicyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	request := r.buildRequest(ctx, plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateMaskingPolicy(ctx, plan.ID.ValueString(), request); err != nil {
		resp.Diagnostics.AddError("Unable to update masking policy", err.Error())
		return
	}

	policy, err := r.client.GetMaskingPolicy(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read masking policy after update", err.Error())
		return
	}
	if policy == nil {
		resp.Diagnostics.AddError(
			"Masking policy not found after update",
			"The policy was updated but cannot be read back. Re-run the plan to reconcile.",
		)
		return
	}

	r.applyPolicy(ctx, &plan, policy, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MaskingPolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state MaskingPolicyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeletePolicy(ctx, state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete masking policy", err.Error())
	}
}

func (r *MaskingPolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *MaskingPolicyResource) buildRequest(
	ctx context.Context,
	plan MaskingPolicyResourceModel,
	diagnostics *diag.Diagnostics,
) client.MaskingPolicyRequest {
	internalTools, customCodeTools, integrationTools := readToolTargets(ctx, policyToolTargets{
		InternalToolIDs:        plan.InternalToolIDs,
		CustomCodeToolIDs:      plan.CustomCodeToolIDs,
		CustomIntegrationTools: plan.CustomIntegrationTools,
	}, diagnostics)

	configuration := map[string]interface{}{}
	for _, detector := range stringSlice(ctx, plan.Detectors, diagnostics) {
		field, ok := client.MaskingDetectorAPIField(detector)
		if !ok {
			diagnostics.AddError(
				"Unknown masking detector",
				fmt.Sprintf("Detector %q is not supported by this provider version.", detector),
			)
			continue
		}
		configuration[field] = true
	}
	if regexIDs := stringSlice(ctx, plan.CustomMaskingRegexIDs, diagnostics); len(regexIDs) > 0 {
		configuration["customMaskingRegexIds"] = regexIDs
	}
	if diagnostics.HasError() {
		return client.MaskingPolicyRequest{}
	}

	return client.MaskingPolicyRequest{
		ConditionalPolicyRequest: client.ConditionalPolicyRequest{
			AppIDs:                 stringSlice(ctx, plan.ApplicationIDs, diagnostics),
			Name:                   plan.Name.ValueString(),
			Description:            stringPointer(plan.Description),
			Enabled:                plan.Enabled.ValueBool(),
			Slug:                   stringPointer(plan.Slug),
			TenantID:               stringPointer(plan.TenantID),
			InternalToolIDs:        internalTools,
			CustomCodeToolIDs:      customCodeTools,
			CustomIntegrationTools: integrationTools,
			Targeting:              buildTargeting(plan.Targeting, diagnostics),
		},
		PolicyConfiguration: configuration,
	}
}

func (r *MaskingPolicyResource) applyPolicy(
	ctx context.Context,
	model *MaskingPolicyResourceModel,
	policy *client.Policy,
	diagnostics *diag.Diagnostics,
) {
	model.ID = types.StringValue(policy.ID)
	model.Name = types.StringValue(policy.Name)
	model.Enabled = types.BoolValue(policy.Enabled)
	model.Description = optionalString(policy.Description)
	model.TenantID = optionalString(policy.TenantID)
	model.Slug = optionalString(policy.Slug)
	model.ApplicationIDs = nullIfEmptySet(ctx, policy.AppIDs, diagnostics)
	model.InternalToolIDs = stringSetValue(ctx, orEmpty(policy.InternalToolIDs), diagnostics)
	model.CustomCodeToolIDs = nullIfEmptySet(ctx, policy.CustomCodeToolIDs, diagnostics)
	model.CustomIntegrationTools = nullIfEmptySet(ctx, policy.CustomIntegrationTools, diagnostics)
	model.Targeting = flattenTargeting(policy.Targeting, diagnostics)

	detectors := make([]string, 0)
	regexIDs := make([]string, 0)
	for field, value := range policy.PolicyConfiguration {
		if field == "customMaskingRegexIds" {
			rawIDs, _ := value.([]interface{})
			for _, rawID := range rawIDs {
				if id, ok := rawID.(string); ok {
					regexIDs = append(regexIDs, id)
				}
			}
			continue
		}
		if enabled, ok := value.(bool); ok && enabled {
			if name, known := client.MaskingDetectorFromAPIField(field); known {
				detectors = append(detectors, name)
			}
		}
	}

	model.Detectors = nullIfEmptySet(ctx, detectors, diagnostics)
	model.CustomMaskingRegexIDs = nullIfEmptySet(ctx, regexIDs, diagnostics)
}
