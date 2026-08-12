package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &ApplicationResource{}
	_ resource.ResourceWithImportState = &ApplicationResource{}
)

// applicationAccessTypes are the access models an application can use.
var applicationAccessTypes = []string{"FREE_ACCESS", "MANAGED_ACCESS"}

// applicationTypes are the client form factors an application can represent.
var applicationTypes = []string{"web", "mobile-ios", "mobile-android", "agent", "other"}

// applicationFrontendStacks are the frontend stacks the API recognizes.
var applicationFrontendStacks = []string{
	"react", "vue", "angular", "next.js", "vanilla.js", "ionic", "flutter",
	"react-native", "kotlin", "swift",
}

func NewApplicationResource() resource.Resource {
	return &ApplicationResource{}
}

// ApplicationResource manages a Frontegg application.
type ApplicationResource struct {
	client *client.Client
}

// ApplicationResourceModel is the Terraform state for an application.
type ApplicationResourceModel struct {
	ID            types.String `tfsdk:"id"`
	VendorID      types.String `tfsdk:"vendor_id"`
	AllowCimd     types.Bool   `tfsdk:"allow_cimd"`
	DPoPEnforce   types.String `tfsdk:"dpop_enforcement_type"`
	Name          types.String `tfsdk:"name"`
	AppURL        types.String `tfsdk:"app_url"`
	LoginURL      types.String `tfsdk:"login_url"`
	LogoURL       types.String `tfsdk:"logo_url"`
	AccessType    types.String `tfsdk:"access_type"`
	IsDefault     types.Bool   `tfsdk:"is_default"`
	IsActive      types.Bool   `tfsdk:"is_active"`
	Type          types.String `tfsdk:"type"`
	FrontendStack types.String `tfsdk:"frontend_stack"`
	Description   types.String `tfsdk:"description"`
	AllowDcr      types.Bool   `tfsdk:"allow_dcr"`
	AppHost       types.String `tfsdk:"app_host"`
}

func (r *ApplicationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

func (r *ApplicationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a Frontegg application. Every other Agenco resource is scoped to an application.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Application ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vendor_id": schema.StringAttribute{
				Description: "Vendor that owns the application.",
				Computed:    true,
			},
			"name": schema.StringAttribute{
				Description: "Display name of the application.",
				Required:    true,
			},
			"app_url": schema.StringAttribute{
				Description: "URL the application is served from.",
				Required:    true,
			},
			"login_url": schema.StringAttribute{
				Description: "URL users are sent to in order to log in.",
				Required:    true,
			},
			"logo_url": schema.StringAttribute{
				Description: "URL of the application logo.",
				Optional:    true,
			},
			"access_type": schema.StringAttribute{
				Description: fmt.Sprintf("Access model for the application. One of: %v. Defaults to "+
					"FREE_ACCESS, which is what the Frontegg portal sends when onboarding an application. "+
					"MANAGED_ACCESS requires tenants to be assigned to the application explicitly, through "+
					"the tenant-assignments API this provider does not cover.", applicationAccessTypes),
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("FREE_ACCESS"),
				Validators: []validator.String{
					stringvalidator.OneOf(applicationAccessTypes...),
				},
			},
			"is_default": schema.BoolAttribute{
				Description: "Whether this is the vendor's default application. Defaults to false.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether the application is active. Defaults to true.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"type": schema.StringAttribute{
				Description: fmt.Sprintf("Client form factor. One of: %v. Defaults to web.", applicationTypes),
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("web"),
				Validators: []validator.String{
					stringvalidator.OneOf(applicationTypes...),
				},
			},
			"frontend_stack": schema.StringAttribute{
				Description: fmt.Sprintf("Frontend stack the application is built with. One of: %v. Defaults "+
					"to react.", applicationFrontendStacks),
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("react"),
				Validators: []validator.String{
					stringvalidator.OneOf(applicationFrontendStacks...),
				},
			},
			"description": schema.StringAttribute{
				Description: "Free-text description of the application.",
				Optional:    true,
			},
			"allow_dcr": schema.BoolAttribute{
				Description: "Whether OAuth Dynamic Client Registration is allowed for this application, which " +
					"is how MCP clients register themselves. Defaults to true, matching the state the Frontegg " +
					"portal leaves an onboarded application in — note this is the opposite of the API's own " +
					"default of false. Set it to false to require pre-registered OAuth clients.",
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"app_host": schema.StringAttribute{
				Description: "Host assigned to the application by Frontegg.",
				Computed:    true,
			},
			"allow_cimd": schema.BoolAttribute{
				Description: "Whether client ID metadata document clients are allowed. Read-only here: the " +
					"applications API returns it but the create and update payloads are not documented to " +
					"accept it, so it is reported rather than managed.",
				Computed: true,
			},
			"dpop_enforcement_type": schema.StringAttribute{
				Description: "How DPoP proof-of-possession is enforced for tokens issued to this application. " +
					"Read-only here, for the same reason as allow_cimd.",
				Computed: true,
			},
		},
	}
}

func (r *ApplicationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *ApplicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ApplicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	application, err := r.client.CreateApplication(ctx, applicationRequest(plan))
	if err != nil {
		resp.Diagnostics.AddError("Unable to create application", err.Error())
		return
	}

	applyApplication(&plan, application)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ApplicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ApplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	application, err := r.client.GetApplication(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read application", err.Error())
		return
	}
	if application == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	applyApplication(&state, application)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ApplicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ApplicationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.UpdateApplication(ctx, plan.ID.ValueString(), applicationRequest(plan)); err != nil {
		resp.Diagnostics.AddError("Unable to update application", err.Error())
		return
	}

	application, err := r.client.GetApplication(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read application after update", err.Error())
		return
	}
	if application == nil {
		resp.Diagnostics.AddError(
			"Application disappeared after update",
			"The application was updated but can no longer be read. Re-run the plan to reconcile.",
		)
		return
	}

	applyApplication(&plan, application)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ApplicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ApplicationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteApplication(ctx, state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete application", err.Error())
	}
}

func (r *ApplicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func applicationRequest(plan ApplicationResourceModel) client.ApplicationRequest {
	return client.ApplicationRequest{
		Name:          plan.Name.ValueString(),
		AppURL:        plan.AppURL.ValueString(),
		LoginURL:      plan.LoginURL.ValueString(),
		LogoURL:       plan.LogoURL.ValueString(),
		AccessType:    plan.AccessType.ValueString(),
		IsDefault:     boolPointer(plan.IsDefault),
		IsActive:      boolPointer(plan.IsActive),
		Type:          plan.Type.ValueString(),
		FrontendStack: plan.FrontendStack.ValueString(),
		Description:   plan.Description.ValueString(),
		AllowDcr:      boolPointer(plan.AllowDcr),
	}
}

func applyApplication(model *ApplicationResourceModel, application *client.Application) {
	model.ID = types.StringValue(application.ID)
	model.VendorID = types.StringValue(application.VendorID)
	model.Name = types.StringValue(application.Name)
	model.AppURL = types.StringValue(application.AppURL)
	model.LoginURL = types.StringValue(application.LoginURL)
	model.AccessType = types.StringValue(application.AccessType)
	model.IsDefault = types.BoolValue(application.IsDefault)
	model.IsActive = types.BoolValue(application.IsActive)
	model.Type = types.StringValue(application.Type)
	model.FrontendStack = types.StringValue(application.FrontendStack)
	model.AllowDcr = types.BoolValue(application.AllowDcr)
	model.AppHost = types.StringValue(application.AppHost)
	model.AllowCimd = types.BoolValue(application.AllowCimd)
	model.DPoPEnforce = types.StringValue(application.DPoPEnforce)

	// logo_url and description are optional-only; keep them null when the API returns nothing
	// so an unset attribute does not read back as an empty string and churn the plan.
	model.LogoURL = optionalString(&application.LogoURL)
	model.Description = optionalString(&application.Description)
}
