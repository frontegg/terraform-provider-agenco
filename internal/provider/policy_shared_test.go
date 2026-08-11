package provider

import (
	"testing"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// condition builds a leaf condition in the shape the API returns.
func condition(attribute, op string, value interface{}) map[string]interface{} {
	return map[string]interface{}{
		"attribute": attribute,
		"op":        op,
		"negate":    false,
		"value":     value,
	}
}

// group builds a condition group in the shape the API returns.
func group(logic string, conditions ...interface{}) map[string]interface{} {
	return map[string]interface{}{
		"conditionLogic": logic,
		"conditions":     conditions,
	}
}

func targetingWith(ifBlock *client.PolicyIfBlock) *client.PolicyTargeting {
	return &client.PolicyTargeting{
		If:   ifBlock,
		Then: client.PolicyThenBlock{Result: "deny"},
	}
}

// TestFlattenTargetingUnwrapsServerRewrite covers the shape the service writes when a policy
// names tools: a tool condition beside a group holding the caller's own conditions.
func TestFlattenTargetingUnwrapsServerRewrite(t *testing.T) {
	stored := targetingWith(&client.PolicyIfBlock{
		ConditionLogic: "and",
		Conditions: []interface{}{
			condition("tool-id", "in_list", map[string]interface{}{"list": []interface{}{"tool-1"}}),
			group("or",
				condition("country", "in_list", map[string]interface{}{"list": []interface{}{"US"}}),
				condition("user-agent", "contains", map[string]interface{}{"list": []interface{}{"curl"}}),
			),
		},
	})

	var diagnostics diag.Diagnostics
	result := flattenTargeting(stored, &diagnostics)

	if diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diagnostics.Errors())
	}
	if len(result) != 1 || len(result[0].If) != 1 {
		t.Fatalf("expected one targeting rule with one if block, got %#v", result)
	}

	ifBlock := result[0].If[0]
	if got := ifBlock.ConditionLogic.ValueString(); got != "or" {
		t.Errorf("expected the user group's logic %q to surface as condition_logic, got %q", "or", got)
	}
	if len(ifBlock.Conditions) != 2 {
		t.Errorf("expected 2 user conditions, got %d", len(ifBlock.Conditions))
	}
	if len(ifBlock.Groups) != 0 {
		t.Errorf("expected the wrapper group to be unwrapped, got %d groups", len(ifBlock.Groups))
	}
	if got := ifBlock.Conditions[0].Attribute.ValueString(); got != "country" {
		t.Errorf("expected first user condition to be country, got %q", got)
	}
}

// TestFlattenTargetingDropsToolOnlyIfBlock covers a policy that names tools but configured no
// conditions of its own: everything in the if block is server-injected.
func TestFlattenTargetingDropsToolOnlyIfBlock(t *testing.T) {
	stored := targetingWith(&client.PolicyIfBlock{
		Conditions: []interface{}{
			condition("tool-id", "in_list", map[string]interface{}{"list": []interface{}{"tool-1"}}),
		},
	})

	var diagnostics diag.Diagnostics
	result := flattenTargeting(stored, &diagnostics)

	if diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diagnostics.Errors())
	}
	if len(result) != 1 {
		t.Fatalf("expected one targeting rule, got %d", len(result))
	}
	if len(result[0].If) != 0 {
		t.Errorf("expected no if block when only tool scoping was stored, got %#v", result[0].If)
	}
}

// TestFlattenTargetingFiltersLegacyFlatToolCondition covers older policies where the tool
// condition sits alongside the caller's conditions instead of wrapping them.
func TestFlattenTargetingFiltersLegacyFlatToolCondition(t *testing.T) {
	stored := targetingWith(&client.PolicyIfBlock{
		ConditionLogic: "and",
		Conditions: []interface{}{
			condition("tool-name", "in_list", map[string]interface{}{"list": []interface{}{"createRefund"}}),
			condition("country", "in_list", map[string]interface{}{"list": []interface{}{"DE"}}),
		},
	})

	var diagnostics diag.Diagnostics
	result := flattenTargeting(stored, &diagnostics)

	if diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diagnostics.Errors())
	}
	ifBlock := result[0].If[0]
	if len(ifBlock.Conditions) != 1 {
		t.Fatalf("expected the tool condition to be filtered out, got %d conditions", len(ifBlock.Conditions))
	}
	if got := ifBlock.Conditions[0].Attribute.ValueString(); got != "country" {
		t.Errorf("expected the surviving condition to be country, got %q", got)
	}
	if got := ifBlock.ConditionLogic.ValueString(); got != "and" {
		t.Errorf("expected the top-level logic to be preserved as %q, got %q", "and", got)
	}
}

// TestFlattenTargetingRecognizesMixedToolWrapperGroup covers policies spanning internal and
// custom-integration tools, where the injected wrapper is an OR-group of tool conditions.
func TestFlattenTargetingRecognizesMixedToolWrapperGroup(t *testing.T) {
	stored := targetingWith(&client.PolicyIfBlock{
		ConditionLogic: "and",
		Conditions: []interface{}{
			group("or",
				condition("tool-id", "in_list", map[string]interface{}{"list": []interface{}{"tool-1"}}),
				condition("tool-name", "in_list", map[string]interface{}{"list": []interface{}{"slack_chat_postMessage"}}),
				condition("slug", "in_list", map[string]interface{}{"list": []interface{}{"sales"}}),
			),
			group("and",
				condition("country", "in_list", map[string]interface{}{"list": []interface{}{"DE"}}),
			),
		},
	})

	var diagnostics diag.Diagnostics
	result := flattenTargeting(stored, &diagnostics)

	if diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diagnostics.Errors())
	}
	ifBlock := result[0].If[0]
	if len(ifBlock.Conditions) != 1 || len(ifBlock.Groups) != 0 {
		t.Fatalf("expected the tool wrapper group to be stripped and the user group unwrapped, got %#v", ifBlock)
	}
	if got := ifBlock.Conditions[0].Attribute.ValueString(); got != "country" {
		t.Errorf("expected the surviving condition to be country, got %q", got)
	}
}

// TestFlattenTargetingKeepsGenuineUserGroups guards against over-eager stripping: a group of
// non-tool conditions must survive as a condition_group.
func TestFlattenTargetingKeepsGenuineUserGroups(t *testing.T) {
	stored := targetingWith(&client.PolicyIfBlock{
		ConditionLogic: "or",
		Conditions: []interface{}{
			group("and",
				condition("user-agent", "contains", map[string]interface{}{"list": []interface{}{"curl"}}),
				condition("country", "in_list", map[string]interface{}{"list": []interface{}{"RU"}}),
			),
			condition("ip", "in_list", map[string]interface{}{"list": []interface{}{"203.0.113.1"}}),
		},
	})

	var diagnostics diag.Diagnostics
	result := flattenTargeting(stored, &diagnostics)

	if diagnostics.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diagnostics.Errors())
	}
	ifBlock := result[0].If[0]
	if len(ifBlock.Groups) != 1 {
		t.Fatalf("expected the user group to survive, got %d groups", len(ifBlock.Groups))
	}
	if len(ifBlock.Conditions) != 1 {
		t.Fatalf("expected the leaf condition to survive, got %d conditions", len(ifBlock.Conditions))
	}
	if got := ifBlock.Groups[0].ConditionLogic.ValueString(); got != "and" {
		t.Errorf("expected the group's logic to be preserved, got %q", got)
	}
	if got := ifBlock.ConditionLogic.ValueString(); got != "or" {
		t.Errorf("expected the top-level logic to be preserved, got %q", got)
	}
}

// TestFlattenTargetingWithoutIfBlock covers an unconditional policy.
func TestFlattenTargetingWithoutIfBlock(t *testing.T) {
	var diagnostics diag.Diagnostics

	if result := flattenTargeting(nil, &diagnostics); result != nil {
		t.Errorf("expected nil for absent targeting, got %#v", result)
	}

	result := flattenTargeting(&client.PolicyTargeting{
		Then: client.PolicyThenBlock{Result: "mask"},
	}, &diagnostics)

	if len(result) != 1 || len(result[0].If) != 0 || len(result[0].Then) != 1 {
		t.Fatalf("expected one rule with a then block and no if block, got %#v", result)
	}
	if got := result[0].Then[0].Result.ValueString(); got != "mask" {
		t.Errorf("expected result mask, got %q", got)
	}
	if diagnostics.HasError() {
		t.Errorf("unexpected diagnostics: %v", diagnostics.Errors())
	}
}

// TestBuildTargetingRejectsApprovalWithoutFlow guards the validation that the API would
// otherwise reject at apply time.
func TestBuildTargetingRejectsApprovalWithoutFlow(t *testing.T) {
	var diagnostics diag.Diagnostics

	buildTargeting([]PolicyTargetingModel{{
		Then: []PolicyThenModel{{
			Result:         types.StringValue("approval_required"),
			ApprovalFlowID: types.StringNull(),
		}},
	}}, &diagnostics)

	if !diagnostics.HasError() {
		t.Error("expected an error when result is approval_required without approval_flow_id")
	}
}
