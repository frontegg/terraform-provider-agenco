package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// TestProviderSchema exercises the GetProviderSchema RPC, which is where the framework
// validates every resource and data source schema it was handed.
func TestProviderSchema(t *testing.T) {
	server := providerserver.NewProtocol6(New("test")())()

	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema returned an error: %s", err)
	}

	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema diagnostic: %s — %s", diagnostic.Summary, diagnostic.Detail)
		}
	}

	if response.Provider == nil {
		t.Fatal("provider schema is missing")
	}
}

// TestResourceTypeNames asserts every resource is registered, uniquely named and prefixed.
func TestResourceTypeNames(t *testing.T) {
	server := providerserver.NewProtocol6(New("test")())()

	response, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema returned an error: %s", err)
	}

	expectedResources := len(New("test")().Resources(context.Background()))
	if got := len(response.ResourceSchemas); got != expectedResources {
		t.Errorf("expected %d resource schemas, got %d", expectedResources, got)
	}

	expectedDataSources := len(New("test")().DataSources(context.Background()))
	if got := len(response.DataSourceSchemas); got != expectedDataSources {
		t.Errorf("expected %d data source schemas, got %d", expectedDataSources, got)
	}

	for name := range response.ResourceSchemas {
		if !strings.HasPrefix(name, "agenco_") {
			t.Errorf("resource %q is not prefixed with agenco_", name)
		}
	}
	for name := range response.DataSourceSchemas {
		if !strings.HasPrefix(name, "agenco_") {
			t.Errorf("data source %q is not prefixed with agenco_", name)
		}
	}
}

// TestMaskingDetectorRoundTrip guards the generated detector mapping against typos: every
// Terraform name must resolve to an API field and back to the same name.
func TestMaskingDetectorRoundTrip(t *testing.T) {
	names := client.MaskingDetectorNames()
	if len(names) == 0 {
		t.Fatal("no masking detectors are registered")
	}

	for _, name := range names {
		field, ok := client.MaskingDetectorAPIField(name)
		if !ok {
			t.Errorf("detector %q has no API field", name)
			continue
		}
		roundTripped, ok := client.MaskingDetectorFromAPIField(field)
		if !ok {
			t.Errorf("API field %q does not map back to a detector name", field)
			continue
		}
		if roundTripped != name {
			t.Errorf("detector %q round-tripped to %q via field %q", name, roundTripped, field)
		}
	}
}
