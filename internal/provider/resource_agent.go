package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &AgentResource{}
	_ resource.ResourceWithImportState = &AgentResource{}
)

// agentClasses are the trust classes a registered agent can belong to.
var agentClasses = []string{"internal", "external", "unknown"}

func NewAgentResource() resource.Resource {
	return &AgentResource{}
}

// AgentResource manages a registered agent identity and its issued credentials.
type AgentResource struct {
	client *client.Client
}

// AgentResourceModel is the Terraform state for a registered agent.
type AgentResourceModel struct {
	ID            types.String  `tfsdk:"id"`
	ApplicationID types.String  `tfsdk:"application_id"`
	Name          types.String  `tfsdk:"name"`
	AgentClass    types.String  `tfsdk:"agent_class"`
	Autonomous    types.Bool    `tfsdk:"autonomous"`
	Description   types.String  `tfsdk:"description"`
	RedirectURLs  types.List    `tfsdk:"redirect_urls"`
	Tags          types.Set     `tfsdk:"tags"`
	OwnerEmail    types.String  `tfsdk:"owner_email"`
	CredentialsID types.String  `tfsdk:"credentials_id"`
	ClientSecret  types.String  `tfsdk:"client_secret"`
	Status        types.String  `tfsdk:"status"`
	AgentSource   types.String  `tfsdk:"agent_source"`
	TrustScore    types.Float64 `tfsdk:"trust_score"`
	IdentityTier  types.String  `tfsdk:"identity_tier"`
}

func (r *AgentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent"
}

func (r *AgentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Registers an agent identity and issues credentials for it. The API has no update route, " +
			"so any change to a managed attribute replaces the agent and rotates its credentials.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Agent ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application the agent is registered against.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "Agent name, up to 255 characters.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(255),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"agent_class": schema.StringAttribute{
				Description: fmt.Sprintf("Trust class of the agent. One of: %v. Ignored when autonomous is true.", agentClasses),
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(agentClasses...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"autonomous": schema.BoolAttribute{
				Description: "Register the agent as autonomous. Autonomous agents take a description instead of " +
					"an agent class and redirect URLs. Absent from the API's read route, so an imported agent " +
					"cannot recover it.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(false),
					requiresReplaceIfKnownBool(),
				},
			},
			"description": schema.StringAttribute{
				Description: "Description of the agent. Only used when autonomous is true. Absent from the " +
					"API's read route, so an imported agent cannot recover it.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					requiresReplaceIfKnownString(),
				},
			},
			"redirect_urls": schema.ListAttribute{
				Description: "OAuth redirect URLs. Only used when autonomous is false. Absent from the API's " +
					"read route, so an imported agent cannot recover them.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					requiresReplaceIfKnownList(),
				},
			},
			"tags": schema.SetAttribute{
				Description: "Free-form tags attached to the agent.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
			},
			"owner_email": schema.StringAttribute{
				Description: "Email of the person responsible for the agent.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(255),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"credentials_id": schema.StringAttribute{
				Description: "Client ID issued to the agent.",
				Computed:    true,
			},
			"client_secret": schema.StringAttribute{
				Description: "Client secret issued to the agent. Returned only at registration time.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				Description: "Lifecycle status of the agent.",
				Computed:    true,
			},
			"agent_source": schema.StringAttribute{
				Description: "How the agent came to be known: registered, auto-detected or discovered.",
				Computed:    true,
			},
			"trust_score": schema.Float64Attribute{
				Description: "Current trust score, when one has been computed.",
				Computed:    true,
			},
			"identity_tier": schema.StringAttribute{
				Description: "Identity tier derived from the trust score.",
				Computed:    true,
			},
		},
	}
}

func (r *AgentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *AgentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AgentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var agent *client.Agent
	var err error

	if plan.Autonomous.ValueBool() {
		agent, err = r.client.CreateAutonomousAgent(ctx, client.CreateAutonomousAgentRequest{
			AppID:       plan.ApplicationID.ValueString(),
			AgentName:   plan.Name.ValueString(),
			Description: stringPointer(plan.Description),
			Tags:        stringSlice(ctx, plan.Tags, &resp.Diagnostics),
			OwnerEmail:  stringPointer(plan.OwnerEmail),
		})
	} else {
		agentClass := plan.AgentClass.ValueString()
		if agentClass == "" {
			agentClass = "internal"
		}
		agent, err = r.client.CreateAgent(ctx, client.CreateAgentRequest{
			AppID:        plan.ApplicationID.ValueString(),
			AgentName:    plan.Name.ValueString(),
			AgentClass:   agentClass,
			RedirectURLs: stringSlice(ctx, plan.RedirectURLs, &resp.Diagnostics),
			Tags:         stringSlice(ctx, plan.Tags, &resp.Diagnostics),
			OwnerEmail:   stringPointer(plan.OwnerEmail),
		})
	}
	if resp.Diagnostics.HasError() {
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to register agent", err.Error())
		return
	}

	applyAgent(ctx, &plan, agent, true, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AgentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agent, err := r.client.GetAgent(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read agent", err.Error())
		return
	}
	if agent == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	applyAgent(ctx, &state, agent, false, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is unreachable: every configurable attribute forces replacement.
func (r *AgentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Agents cannot be updated in place",
		"The Frontegg agent registry has no update route. Every managed attribute is marked as "+
			"requiring replacement, so reaching this path indicates a provider bug.",
	)
}

func (r *AgentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AgentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteAgent(ctx, state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete agent", err.Error())
	}
}

func (r *AgentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.AddWarning(
		"Three agent attributes cannot be recovered by import",
		"The API's read route does not return autonomous, description or redirect_urls, so whatever "+
			"your configuration sets for them is written to state on the next apply without being "+
			"checked against the agent. It does not replace the agent — credentials are not rotated — "+
			"but if a value differs from how the agent was registered, state will be wrong about it, "+
			"and the API has no update route to reconcile it.",
	)
}

// applyAgent maps the API response into state. includeSecret is true only right after
// registration, the one response that carries the client secret.
func applyAgent(ctx context.Context, model *AgentResourceModel, agent *client.Agent, includeSecret bool, diagnostics *diag.Diagnostics) {
	model.ID = types.StringValue(agent.ID)
	model.ApplicationID = types.StringValue(agent.AppID)
	model.Name = types.StringValue(agent.AgentName)
	model.AgentClass = types.StringValue(agent.AgentClass)
	model.AgentSource = types.StringValue(agent.AgentSource)
	model.CredentialsID = types.StringValue(agent.CredentialsID)
	model.Status = types.StringValue(agent.Status)
	model.OwnerEmail = optionalString(&agent.OwnerEmail)
	model.IdentityTier = optionalString(&agent.IdentityTier)

	if agent.TrustScore != nil {
		model.TrustScore = types.Float64Value(*agent.TrustScore)
	} else {
		model.TrustScore = types.Float64Null()
	}

	if len(agent.Tags) == 0 {
		model.Tags = types.SetNull(types.StringType)
	} else {
		model.Tags = stringSetValue(ctx, agent.Tags, diagnostics)
	}

	// redirect_urls is not returned by the read route, so the configured value is kept.

	if includeSecret {
		model.ClientSecret = types.StringValue(agent.ClientSecret)
	}
}
