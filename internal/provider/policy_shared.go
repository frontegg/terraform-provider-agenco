package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// policyResults are the outcomes a policy can produce when its conditions match.
var policyResults = []string{"allow", "deny", "step_up", "approval_required", "mask"}

// conditionLogicOperators combine sibling conditions.
var conditionLogicOperators = []string{"and", "or"}

// PolicyConditionModel is one leaf condition inside a targeting block.
type PolicyConditionModel struct {
	Attribute types.String `tfsdk:"attribute"`
	Op        types.String `tfsdk:"op"`
	Negate    types.Bool   `tfsdk:"negate"`
	Value     types.String `tfsdk:"value"`
}

// PolicyConditionGroupModel nests conditions under their own logic operator.
type PolicyConditionGroupModel struct {
	ConditionLogic types.String           `tfsdk:"condition_logic"`
	Conditions     []PolicyConditionModel `tfsdk:"condition"`
}

// PolicyIfModel is the condition side of a targeting rule.
type PolicyIfModel struct {
	ConditionLogic types.String                `tfsdk:"condition_logic"`
	Conditions     []PolicyConditionModel      `tfsdk:"condition"`
	Groups         []PolicyConditionGroupModel `tfsdk:"condition_group"`
}

// PolicyThenModel is the outcome side of a targeting rule.
type PolicyThenModel struct {
	Result            types.String `tfsdk:"result"`
	ApprovalFlowID    types.String `tfsdk:"approval_flow_id"`
	StepUpActionAsync types.Bool   `tfsdk:"step_up_action_async"`
}

// PolicyTargetingModel is the if/then rule attached to a policy.
type PolicyTargetingModel struct {
	If   []PolicyIfModel   `tfsdk:"if"`
	Then []PolicyThenModel `tfsdk:"then"`
}

// targetingBlock is the shared schema for the targeting block on conditional and masking policies.
func targetingBlock() schema.Block {
	conditionBlock := schema.ListNestedBlock{
		Description: "A condition evaluated against the request.",
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"attribute": schema.StringAttribute{
					Description: "Request attribute to evaluate, for example ip, country, roles or tool-name.",
					Required:    true,
				},
				"op": schema.StringAttribute{
					Description: "Comparison operator, for example in_list, equal, matches or between_numeric.",
					Required:    true,
				},
				"negate": schema.BoolAttribute{
					Description: "Whether to invert the comparison result.",
					Required:    true,
				},
				"value": schema.StringAttribute{
					Description: "Comparison operand as a JSON object string. Its shape follows op, for example " +
						"{\"list\":[\"US\"]} for in_list or {\"number\":5} for equal.",
					Required: true,
				},
			},
		},
	}

	return schema.ListNestedBlock{
		Description: "Targeting rule deciding when the policy applies and what it does.",
		NestedObject: schema.NestedBlockObject{
			Blocks: map[string]schema.Block{
				"if": schema.ListNestedBlock{
					Description: "Conditions that must match. Omit to apply the policy unconditionally.",
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"condition_logic": schema.StringAttribute{
								Description: "How the top-level conditions and groups combine. One of: and, or. " +
									"The API defaults it to and when omitted.",
								Optional: true,
								Computed: true,
								Validators: []validator.String{
									stringvalidator.OneOf(conditionLogicOperators...),
								},
							},
						},
						Blocks: map[string]schema.Block{
							"condition": conditionBlock,
							"condition_group": schema.ListNestedBlock{
								Description: "A nested group of conditions with its own logic operator.",
								NestedObject: schema.NestedBlockObject{
									Attributes: map[string]schema.Attribute{
										"condition_logic": schema.StringAttribute{
											Description: "How the conditions in this group combine. One of: and, or.",
											Required:    true,
											Validators: []validator.String{
												stringvalidator.OneOf(conditionLogicOperators...),
											},
										},
									},
									Blocks: map[string]schema.Block{
										"condition": conditionBlock,
									},
								},
							},
						},
					},
				},
				"then": schema.ListNestedBlock{
					Description: "The outcome applied when the conditions match. Exactly one is required.",
					NestedObject: schema.NestedBlockObject{
						Attributes: map[string]schema.Attribute{
							"result": schema.StringAttribute{
								Description: fmt.Sprintf("Policy outcome. One of: %v.", policyResults),
								Required:    true,
								Validators: []validator.String{
									stringvalidator.OneOf(policyResults...),
								},
							},
							"approval_flow_id": schema.StringAttribute{
								Description: "Approval flow to run. Required when result is approval_required.",
								Optional:    true,
							},
							"step_up_action_async": schema.BoolAttribute{
								Description: "Whether a step_up result is triggered asynchronously rather than " +
									"requiring the user to retry. Only meaningful when result is step_up.",
								Optional: true,
							},
						},
					},
				},
			},
		},
	}
}

// buildTargeting converts the Terraform targeting block into the API payload.
func buildTargeting(targeting []PolicyTargetingModel, diagnostics *diag.Diagnostics) *client.PolicyTargeting {
	if len(targeting) == 0 {
		return nil
	}
	if len(targeting) > 1 {
		diagnostics.AddError(
			"Only one targeting block is allowed",
			"A policy accepts a single targeting block.",
		)
		return nil
	}

	rule := targeting[0]
	if len(rule.Then) != 1 {
		diagnostics.AddError(
			"Exactly one then block is required",
			"The targeting block needs exactly one then block describing the policy outcome.",
		)
		return nil
	}

	then := rule.Then[0]
	result := then.Result.ValueString()
	if result == "approval_required" && then.ApprovalFlowID.IsNull() {
		diagnostics.AddError(
			"approval_flow_id is required",
			"A targeting result of approval_required needs an approval_flow_id.",
		)
		return nil
	}

	payload := &client.PolicyTargeting{
		Then: client.PolicyThenBlock{
			Result:            result,
			ApprovalFlowID:    stringPointer(then.ApprovalFlowID),
			StepUpActionAsync: boolPointer(then.StepUpActionAsync),
		},
	}

	if len(rule.If) > 1 {
		diagnostics.AddError(
			"Only one if block is allowed",
			"The targeting block accepts a single if block.",
		)
		return nil
	}
	if len(rule.If) == 0 {
		return payload
	}

	ifBlock := rule.If[0]
	conditions := make([]interface{}, 0, len(ifBlock.Conditions)+len(ifBlock.Groups))
	for _, condition := range ifBlock.Conditions {
		conditions = append(conditions, buildCondition(condition, diagnostics))
	}
	for _, group := range ifBlock.Groups {
		groupConditions := make([]client.PolicyCondition, 0, len(group.Conditions))
		for _, condition := range group.Conditions {
			groupConditions = append(groupConditions, buildCondition(condition, diagnostics))
		}
		conditions = append(conditions, client.PolicyConditionGroup{
			ConditionLogic: group.ConditionLogic.ValueString(),
			Conditions:     groupConditions,
		})
	}
	if diagnostics.HasError() {
		return nil
	}
	if len(conditions) == 0 {
		diagnostics.AddError(
			"The if block needs at least one condition",
			"Add a condition or condition_group block, or drop the if block entirely.",
		)
		return nil
	}

	payload.If = &client.PolicyIfBlock{
		ConditionLogic: ifBlock.ConditionLogic.ValueString(),
		Conditions:     conditions,
	}
	return payload
}

func buildCondition(condition PolicyConditionModel, diagnostics *diag.Diagnostics) client.PolicyCondition {
	return client.PolicyCondition{
		Attribute: condition.Attribute.ValueString(),
		Op:        condition.Op.ValueString(),
		Negate:    condition.Negate.ValueBool(),
		Value:     jsonStringToMap(condition.Value, "targeting condition value", diagnostics),
	}
}

// toolTargetingAttributes are the condition attributes the API injects to scope a policy to
// the tools listed in internal_tool_ids, custom_code_tool_ids and custom_integration_tools.
// "slug" appears in legacy tool wrappers alongside them.
var toolTargetingAttributes = map[string]bool{
	"tool-id":   true,
	"tool-name": true,
}

// flattenTargeting converts an API targeting payload back into Terraform blocks.
//
// The API does not store targeting verbatim. Whenever a policy names any tool, the service
// injects a tool-id/tool-name condition and nests the user's own conditions in a group beside
// it, forcing the top-level logic to "and". This inverts that rewrite so state reflects the
// configuration rather than the server's internal shape.
func flattenTargeting(targeting *client.PolicyTargeting, diagnostics *diag.Diagnostics) []PolicyTargetingModel {
	if targeting == nil {
		return nil
	}

	then := PolicyThenModel{
		Result:            types.StringValue(targeting.Then.Result),
		ApprovalFlowID:    optionalString(targeting.Then.ApprovalFlowID),
		StepUpActionAsync: types.BoolNull(),
	}
	if targeting.Then.StepUpActionAsync != nil {
		then.StepUpActionAsync = types.BoolValue(*targeting.Then.StepUpActionAsync)
	}

	rule := PolicyTargetingModel{Then: []PolicyThenModel{then}}
	if targeting.If == nil {
		return []PolicyTargetingModel{rule}
	}

	userConditions, userLogic := extractUserConditions(targeting.If)
	if len(userConditions) == 0 {
		// Everything in the if block was server-injected tool scoping, so the configuration
		// had no if block of its own.
		return []PolicyTargetingModel{rule}
	}

	ifBlock := PolicyIfModel{ConditionLogic: optionalString(&userLogic)}
	for _, raw := range userConditions {
		asMap, ok := raw.(map[string]interface{})
		if !ok {
			diagnostics.AddError(
				"Unexpected condition shape",
				fmt.Sprintf("The API returned a targeting condition of type %T that the provider cannot decode.", raw),
			)
			return nil
		}

		if isLeafCondition(asMap) {
			ifBlock.Conditions = append(ifBlock.Conditions, flattenCondition(asMap, diagnostics))
			continue
		}

		group := PolicyConditionGroupModel{
			ConditionLogic: types.StringValue(stringField(asMap, "conditionLogic")),
		}
		for _, nestedRaw := range conditionsOf(asMap) {
			nestedMap, ok := nestedRaw.(map[string]interface{})
			if !ok {
				diagnostics.AddError(
					"Unexpected nested condition shape",
					"The API returned a nested targeting condition the provider cannot decode.",
				)
				return nil
			}
			group.Conditions = append(group.Conditions, flattenCondition(nestedMap, diagnostics))
		}
		ifBlock.Groups = append(ifBlock.Groups, group)
	}

	rule.If = []PolicyIfModel{ifBlock}
	return []PolicyTargetingModel{rule}
}

// extractUserConditions peels the server-injected tool scoping off an if block, mirroring
// the service's own extractUserConditions, and returns the caller's conditions and logic.
func extractUserConditions(ifBlock *client.PolicyIfBlock) ([]interface{}, string) {
	conditions := ifBlock.Conditions

	// The wrapped shape the service writes: [toolCondition, userGroup].
	if len(conditions) == 2 {
		first, firstOK := conditions[0].(map[string]interface{})
		second, secondOK := conditions[1].(map[string]interface{})
		if firstOK && secondOK && isToolWrapper(first) && !isLeafCondition(second) {
			return conditionsOf(second), stringField(second, "conditionLogic")
		}
	}

	// Legacy flat shape: the tool condition sits alongside the user's own conditions.
	remaining := make([]interface{}, 0, len(conditions))
	for _, raw := range conditions {
		asMap, ok := raw.(map[string]interface{})
		if ok && isToolWrapper(asMap) {
			continue
		}
		remaining = append(remaining, raw)
	}
	return remaining, ifBlock.ConditionLogic
}

// isToolWrapper reports whether a condition is server-injected tool scoping: either a
// tool-id/tool-name leaf, or a group whose children are all tool conditions (or legacy slug
// conditions) with at least one real tool condition among them.
func isToolWrapper(condition map[string]interface{}) bool {
	if isLeafCondition(condition) {
		return toolTargetingAttributes[stringField(condition, "attribute")]
	}

	children := conditionsOf(condition)
	if len(children) == 0 {
		return false
	}

	hasToolCondition := false
	for _, raw := range children {
		child, ok := raw.(map[string]interface{})
		if !ok || !isLeafCondition(child) {
			return false
		}
		attribute := stringField(child, "attribute")
		if toolTargetingAttributes[attribute] {
			hasToolCondition = true
			continue
		}
		if attribute != "slug" {
			return false
		}
	}
	return hasToolCondition
}

// isLeafCondition distinguishes a condition from a condition group: only leaves carry "op".
func isLeafCondition(condition map[string]interface{}) bool {
	_, hasOp := condition["op"]
	return hasOp
}

func conditionsOf(condition map[string]interface{}) []interface{} {
	nested, _ := condition["conditions"].([]interface{})
	return nested
}

func flattenCondition(raw map[string]interface{}, diagnostics *diag.Diagnostics) PolicyConditionModel {
	negate, _ := raw["negate"].(bool)

	value := types.StringNull()
	if rawValue, ok := raw["value"]; ok && rawValue != nil {
		encoded, err := json.Marshal(rawValue)
		if err != nil {
			diagnostics.AddError(
				"Unable to encode targeting condition value",
				fmt.Sprintf("The API returned a condition value that could not be encoded as JSON: %s", err.Error()),
			)
		} else {
			value = types.StringValue(string(encoded))
		}
	}

	return PolicyConditionModel{
		Attribute: types.StringValue(stringField(raw, "attribute")),
		Op:        types.StringValue(stringField(raw, "op")),
		Negate:    types.BoolValue(negate),
		Value:     value,
	}
}

func stringField(raw map[string]interface{}, key string) string {
	value, _ := raw[key].(string)
	return value
}

// policyToolTargets are the tool-selection attributes every policy type shares.
type policyToolTargets struct {
	InternalToolIDs        types.Set
	CustomCodeToolIDs      types.Set
	CustomIntegrationTools types.Set
}

// toolTargetAttributes returns the shared tool-selection schema attributes.
func toolTargetAttributes(internalToolsRequired bool) map[string]schema.Attribute {
	internalTools := schema.SetAttribute{
		Description: "Internal tool IDs the policy applies to. Pass an empty set to apply to all tools.",
		ElementType: types.StringType,
	}
	if internalToolsRequired {
		internalTools.Required = true
	} else {
		internalTools.Optional = true
	}

	return map[string]schema.Attribute{
		"internal_tool_ids": internalTools,
		"custom_code_tool_ids": schema.SetAttribute{
			Description: "Custom code tool IDs the policy applies to.",
			Optional:    true,
			ElementType: types.StringType,
		},
		"custom_integration_tools": schema.SetAttribute{
			Description: "Connector tool API IDs the policy applies to, formatted " +
				"{integration_id}_{group}_{action}, for example slack_chat_postMessage.",
			Optional:    true,
			ElementType: types.StringType,
		},
	}
}

// readToolTargets resolves the shared tool-selection attributes into API slices.
func readToolTargets(ctx context.Context, targets policyToolTargets, diagnostics *diag.Diagnostics) ([]string, []string, []string) {
	internal := stringSlice(ctx, targets.InternalToolIDs, diagnostics)
	if internal == nil {
		internal = []string{}
	}
	return internal,
		stringSlice(ctx, targets.CustomCodeToolIDs, diagnostics),
		stringSlice(ctx, targets.CustomIntegrationTools, diagnostics)
}

// nullIfEmptySet keeps an unset collection null so it does not churn the plan.
func nullIfEmptySet(ctx context.Context, values []string, diagnostics *diag.Diagnostics) types.Set {
	if len(values) == 0 {
		return types.SetNull(types.StringType)
	}
	return stringSetValue(ctx, values, diagnostics)
}
