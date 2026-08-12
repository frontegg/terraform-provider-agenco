package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// A schema-level Default is applied on every plan where configuration leaves the attribute null,
// including for an object that was imported rather than created. That silently rewrites whatever
// the object already held — an application imported with DPoP enforced would be downgraded to
// disabled on the next apply, and an inactive one reactivated. These modifiers apply the default
// only when there is no prior state, so a created object gets the default and an imported one keeps
// its own value. For an existing object the framework's Optional+Computed behaviour takes over and
// carries the prior state value into the plan.
//
// The trade-off is that removing an attribute from configuration no longer reverts it to the
// default; the last applied value is kept instead.

// Some attributes are create-only on the API and absent from its read routes, so import cannot
// recover them and they land null in state. A plain RequiresReplace then reads that null as a change
// and destroys the object to correct a value the API never said was wrong — which for an agent means
// rotating its credentials. These force replacement only when the prior value is actually known.
//
// The cost is that state can record a create-only value that does not match the live object until it
// is next created. Each affected resource says so in the warning it emits on import.

const replaceIfKnownDescription = "Forces replacement when changed, unless the prior value is " +
	"unknown because the resource was imported."

func requiresReplaceIfKnownString() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(
			ctx context.Context,
			req planmodifier.StringRequest,
			resp *stringplanmodifier.RequiresReplaceIfFuncResponse,
		) {
			resp.RequiresReplace = !req.StateValue.IsNull()
		},
		replaceIfKnownDescription,
		replaceIfKnownDescription,
	)
}

func requiresReplaceIfKnownBool() planmodifier.Bool {
	return boolplanmodifier.RequiresReplaceIf(
		func(
			ctx context.Context,
			req planmodifier.BoolRequest,
			resp *boolplanmodifier.RequiresReplaceIfFuncResponse,
		) {
			resp.RequiresReplace = !req.StateValue.IsNull()
		},
		replaceIfKnownDescription,
		replaceIfKnownDescription,
	)
}

func requiresReplaceIfKnownList() planmodifier.List {
	return listplanmodifier.RequiresReplaceIf(
		func(
			ctx context.Context,
			req planmodifier.ListRequest,
			resp *listplanmodifier.RequiresReplaceIfFuncResponse,
		) {
			resp.RequiresReplace = !req.StateValue.IsNull()
		},
		replaceIfKnownDescription,
		replaceIfKnownDescription,
	)
}

func createDefaultBool(value bool) planmodifier.Bool { return createOnlyBoolDefault{value: value} }

func createDefaultString(value string) planmodifier.String {
	return createOnlyStringDefault{value: value}
}

func createDefaultInt64(value int64) planmodifier.Int64 {
	return createOnlyInt64Default{value: value}
}

type createOnlyBoolDefault struct{ value bool }

func (m createOnlyBoolDefault) Description(context.Context) string {
	return fmt.Sprintf("Defaults to %t when the resource is created.", m.value)
}

func (m createOnlyBoolDefault) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m createOnlyBoolDefault) PlanModifyBool(
	ctx context.Context,
	req planmodifier.BoolRequest,
	resp *planmodifier.BoolResponse,
) {
	if req.ConfigValue.IsNull() && req.State.Raw.IsNull() {
		resp.PlanValue = types.BoolValue(m.value)
	}
}

type createOnlyStringDefault struct{ value string }

func (m createOnlyStringDefault) Description(context.Context) string {
	return fmt.Sprintf("Defaults to %q when the resource is created.", m.value)
}

func (m createOnlyStringDefault) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m createOnlyStringDefault) PlanModifyString(
	ctx context.Context,
	req planmodifier.StringRequest,
	resp *planmodifier.StringResponse,
) {
	if req.ConfigValue.IsNull() && req.State.Raw.IsNull() {
		resp.PlanValue = types.StringValue(m.value)
	}
}

type createOnlyInt64Default struct{ value int64 }

func (m createOnlyInt64Default) Description(context.Context) string {
	return fmt.Sprintf("Defaults to %d when the resource is created.", m.value)
}

func (m createOnlyInt64Default) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m createOnlyInt64Default) PlanModifyInt64(
	ctx context.Context,
	req planmodifier.Int64Request,
	resp *planmodifier.Int64Response,
) {
	if req.ConfigValue.IsNull() && req.State.Raw.IsNull() {
		resp.PlanValue = types.Int64Value(m.value)
	}
}
