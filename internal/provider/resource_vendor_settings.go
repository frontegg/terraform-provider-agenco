package provider

import (
	"context"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ---- Allowed origins ----

var (
	_ resource.Resource                = &AllowedOriginsResource{}
	_ resource.ResourceWithImportState = &AllowedOriginsResource{}
)

func NewAllowedOriginsResource() resource.Resource {
	return &AllowedOriginsResource{}
}

// AllowedOriginsResource manages the vendor-level CORS allow-list.
type AllowedOriginsResource struct {
	client *client.Client
}

// AllowedOriginsResourceModel is the Terraform state for the allowed-origins list.
type AllowedOriginsResourceModel struct {
	ID      types.String `tfsdk:"id"`
	Origins types.Set    `tfsdk:"origins"`
}

func (r *AllowedOriginsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allowed_origins"
}

func (r *AllowedOriginsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the vendor-level CORS allow-list. This is a singleton for the whole vendor, so " +
			"declare it at most once. Destroying the resource clears the list.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Vendor ID.",
				Computed:    true,
			},
			"origins": schema.SetAttribute{
				Description: "Origins allowed to call the Frontegg API, for example https://app.example.com.",
				Required:    true,
				ElementType: types.StringType,
			},
		},
	}
}

func (r *AllowedOriginsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *AllowedOriginsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AllowedOriginsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	origins := stringSlice(ctx, plan.Origins, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.UpdateAllowedOrigins(ctx, origins)
	if err != nil {
		resp.Diagnostics.AddError("Unable to set allowed origins", err.Error())
		return
	}

	plan.ID = types.StringValue(config.ID)
	plan.Origins = stringSetValue(ctx, orEmpty(config.AllowedOrigins), &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AllowedOriginsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AllowedOriginsResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.GetVendorConfig(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read vendor configuration", err.Error())
		return
	}

	state.ID = types.StringValue(config.ID)
	state.Origins = stringSetValue(ctx, orEmpty(config.AllowedOrigins), &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AllowedOriginsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AllowedOriginsResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	origins := stringSlice(ctx, plan.Origins, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.UpdateAllowedOrigins(ctx, origins)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update allowed origins", err.Error())
		return
	}

	plan.ID = types.StringValue(config.ID)
	plan.Origins = stringSetValue(ctx, orEmpty(config.AllowedOrigins), &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete clears the allow-list, which is the closest inverse of managing it.
func (r *AllowedOriginsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if _, err := r.client.UpdateAllowedOrigins(ctx, []string{}); err != nil {
		resp.Diagnostics.AddError("Unable to clear allowed origins", err.Error())
	}
}

// ImportState adopts the vendor's existing allow-list without writing to it. There is one per
// vendor, so the ID is not used to look anything up — Read resolves it from the credentials and
// overwrites whatever was passed. Pass the vendor ID for readability.
func (r *AllowedOriginsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ---- Identity configuration ----

var (
	_ resource.Resource                = &IdentityConfigurationResource{}
	_ resource.ResourceWithImportState = &IdentityConfigurationResource{}
)

func NewIdentityConfigurationResource() resource.Resource {
	return &IdentityConfigurationResource{}
}

// IdentityConfigurationResource manages the vendor's token settings.
type IdentityConfigurationResource struct {
	client *client.Client
}

// IdentityConfigurationResourceModel is the Terraform state for the identity configuration.
type IdentityConfigurationResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	DefaultTokenExpiration types.Int64  `tfsdk:"default_token_expiration"`
}

func (r *IdentityConfigurationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_identity_configuration"
}

func (r *IdentityConfigurationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages the vendor's identity configuration. This is a singleton for the whole vendor, " +
			"so declare it at most once. The API has no delete route, so destroying the resource only drops " +
			"it from Terraform state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Identity configuration ID.",
				Computed:    true,
			},
			"default_token_expiration": schema.Int64Attribute{
				Description: "Default access-token lifetime in seconds.",
				Required:    true,
			},
		},
	}
}

func (r *IdentityConfigurationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *IdentityConfigurationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan IdentityConfigurationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.UpdateIdentityConfiguration(ctx, plan.DefaultTokenExpiration.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Unable to write identity configuration", err.Error())
		return
	}

	plan.ID = types.StringValue(config.ID)
	plan.DefaultTokenExpiration = types.Int64Value(config.DefaultTokenExpiration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IdentityConfigurationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state IdentityConfigurationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.GetIdentityConfiguration(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read identity configuration", err.Error())
		return
	}

	state.ID = types.StringValue(config.ID)
	state.DefaultTokenExpiration = types.Int64Value(config.DefaultTokenExpiration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IdentityConfigurationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan IdentityConfigurationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	config, err := r.client.UpdateIdentityConfiguration(ctx, plan.DefaultTokenExpiration.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Unable to update identity configuration", err.Error())
		return
	}

	plan.ID = types.StringValue(config.ID)
	plan.DefaultTokenExpiration = types.Int64Value(config.DefaultTokenExpiration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete drops the resource from state; the API exposes no delete route.
func (r *IdentityConfigurationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"Identity configuration was not deleted remotely",
		"The Frontegg API has no delete route for the identity configuration. It has been removed from "+
			"Terraform state but the stored settings are unchanged.",
	)
}

// ImportState adopts the vendor's existing configuration without writing to it. There is one per
// vendor, so the ID is not used to look anything up — Read resolves it from the credentials and
// overwrites whatever was passed. Pass the configuration ID for readability.
func (r *IdentityConfigurationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
