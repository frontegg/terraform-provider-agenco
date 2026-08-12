package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var emptyObject = tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}

// noPriorState is what the framework passes while planning a create.
func noPriorState() tfsdk.State {
	return tfsdk.State{Raw: tftypes.NewValue(emptyObject, nil)}
}

// priorState is what it passes for an object that already exists, imported or otherwise.
func priorState() tfsdk.State {
	return tfsdk.State{Raw: tftypes.NewValue(emptyObject, map[string]tftypes.Value{})}
}

func TestCreateDefaultAppliesOnCreate(t *testing.T) {
	resp := &planmodifier.BoolResponse{PlanValue: types.BoolUnknown()}
	createDefaultBool(true).PlanModifyBool(context.Background(), planmodifier.BoolRequest{
		ConfigValue: types.BoolNull(),
		State:       noPriorState(),
	}, resp)

	if !resp.PlanValue.Equal(types.BoolValue(true)) {
		t.Errorf("PlanValue = %v, want true", resp.PlanValue)
	}
}

// The whole point of the modifier: an imported object keeps whatever it already had, instead of
// being rewritten to the provider's default on the first apply.
func TestCreateDefaultLeavesExistingObjectAlone(t *testing.T) {
	resp := &planmodifier.BoolResponse{PlanValue: types.BoolValue(false)}
	createDefaultBool(true).PlanModifyBool(context.Background(), planmodifier.BoolRequest{
		ConfigValue: types.BoolNull(),
		State:       priorState(),
	}, resp)

	if !resp.PlanValue.Equal(types.BoolValue(false)) {
		t.Errorf("PlanValue = %v, want the prior value false to be kept", resp.PlanValue)
	}
}

func TestCreateDefaultNeverOverridesConfiguration(t *testing.T) {
	resp := &planmodifier.BoolResponse{PlanValue: types.BoolValue(false)}
	createDefaultBool(true).PlanModifyBool(context.Background(), planmodifier.BoolRequest{
		ConfigValue: types.BoolValue(false),
		State:       noPriorState(),
	}, resp)

	if !resp.PlanValue.Equal(types.BoolValue(false)) {
		t.Errorf("PlanValue = %v, want the configured value false", resp.PlanValue)
	}
}

func TestCreateDefaultStringAndInt64(t *testing.T) {
	stringResp := &planmodifier.StringResponse{PlanValue: types.StringUnknown()}
	createDefaultString("disabled").PlanModifyString(context.Background(), planmodifier.StringRequest{
		ConfigValue: types.StringNull(),
		State:       noPriorState(),
	}, stringResp)

	if !stringResp.PlanValue.Equal(types.StringValue("disabled")) {
		t.Errorf("PlanValue = %v, want disabled", stringResp.PlanValue)
	}

	int64Resp := &planmodifier.Int64Response{PlanValue: types.Int64Unknown()}
	createDefaultInt64(5000).PlanModifyInt64(context.Background(), planmodifier.Int64Request{
		ConfigValue: types.Int64Null(),
		State:       noPriorState(),
	}, int64Resp)

	if !int64Resp.PlanValue.Equal(types.Int64Value(5000)) {
		t.Errorf("PlanValue = %v, want 5000", int64Resp.PlanValue)
	}
}
