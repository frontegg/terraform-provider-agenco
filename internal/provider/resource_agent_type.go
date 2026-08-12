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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &AgentTypeResource{}
	_ resource.ResourceWithImportState = &AgentTypeResource{}
)

// aiPlatforms are the client platforms the gateway can recognise and gate on.
var aiPlatforms = []string{
	"chat-gpt", "claude", "claude-code", "cursor", "gemini", "perplexity", "grok",
	"mcp-gateway-test-application", "unknown",
}

func NewAgentTypeResource() resource.Resource {
	return &AgentTypeResource{}
}

// AgentTypeResource gates which AI platforms may reach an application, and for whom.
type AgentTypeResource struct {
	client *client.Client
}

// AgentTypeResourceModel is the Terraform state for an agent-type rule.
type AgentTypeResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	AgentType     types.String `tfsdk:"agent_type"`
	AllUsers      types.Bool   `tfsdk:"all_users"`
	IsActive      types.Bool   `tfsdk:"is_active"`
	GroupIDs      types.Set    `tfsdk:"group_ids"`
	Description   types.String `tfsdk:"description"`
}

func (r *AgentTypeResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_type"
}

func (r *AgentTypeResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Controls which AI platform may reach an application, and for which directory groups. " +
			"One rule exists per application and platform.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Agent-type rule ID.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application this rule applies to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"agent_type": schema.StringAttribute{
				Description: fmt.Sprintf("AI platform this rule covers. One of: %v. Immutable after create, "+
					"because the API keys the rule on it.", aiPlatforms),
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(aiPlatforms...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"all_users": schema.BoolAttribute{
				Description: "Whether the rule applies to every user. Set false and list group_ids to narrow it.",
				Required:    true,
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether the rule is enforced.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					createDefaultBool(true),
				},
			},
			"group_ids": schema.SetAttribute{
				Description: "Directory group IDs the rule applies to. Required when all_users is false.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"description": schema.StringAttribute{
				Description: "Free-text description of the rule.",
				Optional:    true,
			},
		},
	}
}

func (r *AgentTypeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *AgentTypeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan AgentTypeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.upsert(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AgentTypeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state AgentTypeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentType, err := r.client.GetAgentTypeByPlatform(
		ctx, state.ApplicationID.ValueString(), state.AgentType.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read agent-type rule", err.Error())
		return
	}
	if agentType == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyAgentType(ctx, &state, agentType, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *AgentTypeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan AgentTypeResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.upsert(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *AgentTypeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state AgentTypeResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteAgentType(ctx, state.ID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete agent-type rule", err.Error())
	}
}

// ImportState accepts "application_id/agent_type", since the rule is keyed on the platform.
func (r *AgentTypeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	segments, err := splitImportID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"application_id/agent_type\": %s", err.Error()),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), segments[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("agent_type"), segments[1])...)
}

func (r *AgentTypeResource) upsert(ctx context.Context, plan *AgentTypeResourceModel, diagnostics *diag.Diagnostics) {
	groupIDs := stringSlice(ctx, plan.GroupIDs, diagnostics)
	if diagnostics.HasError() {
		return
	}
	if !plan.AllUsers.ValueBool() && len(groupIDs) == 0 {
		diagnostics.AddError(
			"group_ids is required when all_users is false",
			"Either set all_users to true or list at least one directory group in group_ids.",
		)
		return
	}

	agentType, err := r.client.UpsertAgentType(ctx, client.AgentType{
		AppID:       plan.ApplicationID.ValueString(),
		AgentType:   plan.AgentType.ValueString(),
		AllUsers:    plan.AllUsers.ValueBool(),
		IsActive:    plan.IsActive.ValueBool(),
		GroupIDs:    groupIDs,
		Description: stringPointer(plan.Description),
	})
	if err != nil {
		diagnostics.AddError("Unable to write agent-type rule", err.Error())
		return
	}

	r.applyAgentType(ctx, plan, agentType, diagnostics)
}

func (r *AgentTypeResource) applyAgentType(
	ctx context.Context,
	model *AgentTypeResourceModel,
	agentType *client.AgentType,
	diagnostics *diag.Diagnostics,
) {
	model.ID = types.StringValue(agentType.ID)
	model.ApplicationID = types.StringValue(agentType.AppID)
	model.AgentType = types.StringValue(agentType.AgentType)
	model.AllUsers = types.BoolValue(agentType.AllUsers)
	model.IsActive = types.BoolValue(agentType.IsActive)

	if len(agentType.GroupIDs) == 0 {
		model.GroupIDs = types.SetNull(types.StringType)
	} else {
		model.GroupIDs = stringSetValue(ctx, agentType.GroupIDs, diagnostics)
	}

	// The API response DTO omits description in its entity mapping, so the configured
	// value is kept rather than being nulled on every read.
}
