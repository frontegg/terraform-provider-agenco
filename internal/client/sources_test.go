package client

import (
	"encoding/json"
	"testing"
)

// A source as the API returns it, field-for-field with AppMcpConfigurationSourceResponse.
const sourceResponse = `{
	"id": "59e61384-5e18-4b7f-bf1a-b7663774924a",
	"vendorId": "5ead690e-3d11-4940-81dd-0426445790fc",
	"appId": "2e75fb53-3d73-47ee-9fcc-369f9773e1a4",
	"name": "Orders API",
	"slug": null,
	"type": "REST",
	"sourceUrl": "https://example.com",
	"secret": "s3cr3t",
	"apiTimeout": 5000,
	"enabled": true,
	"isLocal": true,
	"twoStepCallback": false,
	"overrideHeaders": [{"key": "content-type", "value": "application/json"}],
	"externalAuthorizationUrl": null,
	"scopes": [],
	"clientId": null,
	"clientSecret": null
}`

// type and is_local both carry RequiresReplace in the resource schema. Any of them missing from
// this struct reads back as a zero value after import, which plans as destroy-and-recreate rather
// than as no change — so decoding them is a correctness requirement, not a completeness one.
func TestSourceDecodesReplacementForcingFields(t *testing.T) {
	var source Source
	if err := json.Unmarshal([]byte(sourceResponse), &source); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if source.Type != "REST" {
		t.Errorf("Type = %q, want REST", source.Type)
	}
	if !source.IsLocal {
		t.Error("IsLocal = false, want true")
	}
}

func TestSourceDecodesIdentityFields(t *testing.T) {
	var source Source
	if err := json.Unmarshal([]byte(sourceResponse), &source); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	cases := []struct {
		field string
		got   string
		want  string
	}{
		{"ID", source.ID, "59e61384-5e18-4b7f-bf1a-b7663774924a"},
		{"AppID", source.AppID, "2e75fb53-3d73-47ee-9fcc-369f9773e1a4"},
		{"VendorID", source.VendorID, "5ead690e-3d11-4940-81dd-0426445790fc"},
		{"Name", source.Name, "Orders API"},
		{"SourceURL", source.SourceURL, "https://example.com"},
	}
	for _, testCase := range cases {
		if testCase.got != testCase.want {
			t.Errorf("%s = %q, want %q", testCase.field, testCase.got, testCase.want)
		}
	}

	if source.APITimeout != 5000 {
		t.Errorf("APITimeout = %d, want 5000", source.APITimeout)
	}
	if len(source.OverrideHeaders) != 1 || source.OverrideHeaders[0].Key != "content-type" {
		t.Errorf("OverrideHeaders = %+v, want one content-type header", source.OverrideHeaders)
	}
}
