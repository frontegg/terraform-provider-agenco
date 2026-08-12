package provider

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &AgencoProvider{}

// regionURLs maps a Frontegg region to its API base URL.
var regionURLs = map[string]string{
	"stg": "https://api.stg.frontegg.com",
	"eu":  "https://api.frontegg.com",
	"us":  "https://api.us.frontegg.com",
	"au":  "https://api.au.frontegg.com",
	"ca":  "https://api.ca.frontegg.com",
	"uk":  "https://api.uk.frontegg.com",
}

const defaultRegion = "eu"

// AgencoProvider is the Frontegg Agenco provider implementation.
type AgencoProvider struct {
	version string
}

// AgencoProviderModel is the provider block configuration.
type AgencoProviderModel struct {
	Region   types.String `tfsdk:"region"`
	BaseURL  types.String `tfsdk:"base_url"`
	ClientID types.String `tfsdk:"client_id"`
	Secret   types.String `tfsdk:"secret"`
}

// New returns a provider factory bound to the given build version.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &AgencoProvider{version: version}
	}
}

func (p *AgencoProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "agenco"
	resp.Version = p.version
}

func (p *AgencoProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manage Frontegg Agenco — applications, MCP configuration and sources, tools, " +
			"connectors, prompts, access policies and runtime governance controls.",
		Attributes: map[string]schema.Attribute{
			"region": schema.StringAttribute{
				Description: fmt.Sprintf("Frontegg region. One of: %s. Defaults to %q. Also read from FRONTEGG_REGION.",
					strings.Join(validRegions(), ", "), defaultRegion),
				Optional: true,
			},
			"base_url": schema.StringAttribute{
				Description: "Full API base URL. Takes precedence over region. Also read from FRONTEGG_BASE_URL.",
				Optional:    true,
			},
			"client_id": schema.StringAttribute{
				Description: "Frontegg vendor client ID. Also read from FRONTEGG_CLIENT_ID.",
				Optional:    true,
			},
			"secret": schema.StringAttribute{
				Description: "Frontegg vendor API key. Also read from FRONTEGG_SECRET.",
				Optional:    true,
				Sensitive:   true,
			},
		},
	}
}

func (p *AgencoProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config AgencoProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.ClientID.IsUnknown() || config.Secret.IsUnknown() {
		resp.Diagnostics.AddError(
			"Unknown Frontegg credentials",
			"client_id and secret must be known at plan time. Move them to variables or environment variables.",
		)
		return
	}

	region := firstNonEmpty(config.Region, os.Getenv("FRONTEGG_REGION"))
	baseURL := firstNonEmpty(config.BaseURL, os.Getenv("FRONTEGG_BASE_URL"))
	clientID := firstNonEmpty(config.ClientID, os.Getenv("FRONTEGG_CLIENT_ID"))
	secret := firstNonEmpty(config.Secret, os.Getenv("FRONTEGG_SECRET"))

	if baseURL == "" {
		if region == "" {
			region = defaultRegion
		}
		resolved, ok := regionURLs[region]
		if !ok {
			resp.Diagnostics.AddAttributeError(
				path.Root("region"),
				"Invalid Frontegg region",
				fmt.Sprintf("Region %q is not recognized. Valid regions are: %s.", region, strings.Join(validRegions(), ", ")),
			)
			return
		}
		baseURL = resolved
	}

	if clientID == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("client_id"),
			"Missing Frontegg client ID",
			"Set client_id in the provider block or the FRONTEGG_CLIENT_ID environment variable.",
		)
	}
	if secret == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("secret"),
			"Missing Frontegg secret",
			"Set secret in the provider block or the FRONTEGG_SECRET environment variable.",
		)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	apiClient := client.NewClient(baseURL, clientID, secret)
	if err := apiClient.Authenticate(ctx); err != nil {
		resp.Diagnostics.AddError(
			"Unable to authenticate with the Frontegg API",
			fmt.Sprintf("Authentication against %s failed: %s", baseURL, err.Error()),
		)
		return
	}

	resp.DataSourceData = apiClient
	resp.ResourceData = apiClient
}

func (p *AgencoProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewApplicationResource,
		NewSaasAppResource,
		NewMcpConfigurationResource,
		NewMcpSourceResource,
		NewFronteggToolsSourceResource,
		NewSourceToolsActiveStatusResource,
		NewToolsImportResource,
		NewToolResource,
		NewToolHookResource,
		NewCustomCodeToolResource,
		NewCustomIntegrationResource,
		NewPromptResource,
		NewCustomMaskingRegexResource,
		NewConditionalPolicyResource,
		NewRbacPolicyResource,
		NewMaskingPolicyResource,
		NewIPRestrictionResource,
		NewIPRestrictionConfigResource,
		NewGeoFenceResource,
		NewGeoFenceConfigResource,
		NewTimeOfWorkResource,
		NewRateLimitConfigResource,
		NewAgentTypeResource,
		NewAgentResource,
		NewAllowedOriginsResource,
		NewIdentityConfigurationResource,
	}
}

func (p *AgencoProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewApplicationDataSource,
		NewToolsDataSource,
		NewIntegrationTemplatesDataSource,
	}
}

// validRegions returns the supported region names in stable order.
func validRegions() []string {
	regions := make([]string, 0, len(regionURLs))
	for region := range regionURLs {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	return regions
}

// firstNonEmpty prefers an explicit configuration value over the environment fallback.
func firstNonEmpty(configured types.String, fallback string) string {
	if !configured.IsNull() && configured.ValueString() != "" {
		return configured.ValueString()
	}
	return fallback
}
