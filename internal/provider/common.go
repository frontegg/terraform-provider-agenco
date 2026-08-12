package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// slugPattern mirrors the API's slug rule for connector and source instance discriminators.
var slugPattern = regexp.MustCompile(`^[a-z0-9-]+$`)

const slugValidationMessage = "must be lowercase kebab-case (^[a-z0-9-]+$)"

// configureClient extracts the shared API client from provider data.
func configureClient(providerData any, diagnostics *diag.Diagnostics) *client.Client {
	if providerData == nil {
		return nil
	}
	apiClient, ok := providerData.(*client.Client)
	if !ok {
		diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *client.Client, got %T. This is a provider bug.", providerData),
		)
		return nil
	}
	return apiClient
}

// stringPointer returns nil for null or unknown values so optional fields stay absent.
func stringPointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	stringValue := value.ValueString()
	return &stringValue
}

// boolPointer returns nil for null or unknown values so optional fields stay absent.
func boolPointer(value types.Bool) *bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	boolValue := value.ValueBool()
	return &boolValue
}

// int64Pointer returns nil for null or unknown values so optional fields stay absent.
func int64Pointer(value types.Int64) *int64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	intValue := value.ValueInt64()
	return &intValue
}

// optionalString converts an API pointer into a Terraform value, mapping empty to null.
func optionalString(value *string) types.String {
	if value == nil || *value == "" {
		return types.StringNull()
	}
	return types.StringValue(*value)
}

// optionalInt64 converts an API pointer into a Terraform value.
func optionalInt64(value *int64) types.Int64 {
	if value == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*value)
}

// stringSlice reads a Terraform list or set of strings into a Go slice.
func stringSlice(ctx context.Context, value attrCollection, diagnostics *diag.Diagnostics) []string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var result []string
	diagnostics.Append(value.ElementsAs(ctx, &result, false)...)
	return result
}

// attrCollection is the shared surface of types.List and types.Set used by stringSlice.
type attrCollection interface {
	IsNull() bool
	IsUnknown() bool
	ElementsAs(ctx context.Context, target interface{}, allowUnhandled bool) diag.Diagnostics
}

// stringListValue builds a Terraform list, preserving null for absent collections.
func stringListValue(ctx context.Context, values []string, diagnostics *diag.Diagnostics) types.List {
	if values == nil {
		return types.ListNull(types.StringType)
	}
	list, diags := types.ListValueFrom(ctx, types.StringType, values)
	diagnostics.Append(diags...)
	return list
}

// stringSetValue builds a Terraform set, preserving null for absent collections.
func stringSetValue(ctx context.Context, values []string, diagnostics *diag.Diagnostics) types.Set {
	if values == nil {
		return types.SetNull(types.StringType)
	}
	set, diags := types.SetValueFrom(ctx, types.StringType, values)
	diagnostics.Append(diags...)
	return set
}

// jsonStringToMap decodes a JSON object held in a Terraform string attribute.
func jsonStringToMap(value types.String, attribute string, diagnostics *diag.Diagnostics) map[string]interface{} {
	if value.IsNull() || value.IsUnknown() || strings.TrimSpace(value.ValueString()) == "" {
		return nil
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(value.ValueString()), &decoded); err != nil {
		diagnostics.AddError(
			fmt.Sprintf("Invalid JSON in %s", attribute),
			fmt.Sprintf("%s must be a JSON object: %s", attribute, err.Error()),
		)
		return nil
	}
	return decoded
}

// int64Map reads a Terraform map of numbers into a Go map.
func int64Map(ctx context.Context, value types.Map, diagnostics *diag.Diagnostics) map[string]int64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := map[string]int64{}
	diagnostics.Append(value.ElementsAs(ctx, &result, false)...)
	return result
}

// int64MapValue builds a Terraform map of numbers, mapping an empty map to null.
func int64MapValue(ctx context.Context, values map[string]int64, diagnostics *diag.Diagnostics) types.Map {
	if len(values) == 0 {
		return types.MapNull(types.Int64Type)
	}
	result, diags := types.MapValueFrom(ctx, types.Int64Type, values)
	diagnostics.Append(diags...)
	return result
}

// stringMap reads a Terraform map of strings into a Go map.
func stringMap(ctx context.Context, value types.Map, diagnostics *diag.Diagnostics) map[string]string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := map[string]string{}
	diagnostics.Append(value.ElementsAs(ctx, &result, false)...)
	return result
}

// stringMapValue builds a Terraform map of strings, mapping an empty map to null.
func stringMapValue(ctx context.Context, values map[string]string, diagnostics *diag.Diagnostics) types.Map {
	if len(values) == 0 {
		return types.MapNull(types.StringType)
	}
	result, diags := types.MapValueFrom(ctx, types.StringType, values)
	diagnostics.Append(diags...)
	return result
}

// boolMap reads a Terraform map of booleans into a Go map.
func boolMap(ctx context.Context, value types.Map, diagnostics *diag.Diagnostics) map[string]bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	result := map[string]bool{}
	diagnostics.Append(value.ElementsAs(ctx, &result, false)...)
	return result
}

// boolMapValue builds a Terraform map of booleans, mapping an empty map to null.
func boolMapValue(ctx context.Context, values map[string]bool, diagnostics *diag.Diagnostics) types.Map {
	if len(values) == 0 {
		return types.MapNull(types.BoolType)
	}
	result, diags := types.MapValueFrom(ctx, types.BoolType, values)
	diagnostics.Append(diags...)
	return result
}

// splitImportID parses a composite import ID such as "app_id/resource_id".
func splitImportID(id string, parts int) ([]string, error) {
	segments := strings.Split(id, "/")
	if len(segments) != parts {
		return nil, fmt.Errorf("expected %d segments separated by '/', got %d", parts, len(segments))
	}
	for _, segment := range segments {
		if segment == "" {
			return nil, fmt.Errorf("import ID segments must not be empty")
		}
	}
	return segments, nil
}
