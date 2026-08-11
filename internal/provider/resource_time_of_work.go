package provider

import (
	"context"
	"fmt"

	"github.com/frontegg/terraform-provider-agenco/internal/client"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &TimeOfWorkResource{}
	_ resource.ResourceWithImportState = &TimeOfWorkResource{}
)

// timeOfWorkActions are the enforcement actions applied outside working hours.
var timeOfWorkActions = []string{"deny", "step_up"}

// millisecondsInDay bounds the window offsets, which the API expresses in milliseconds.
const millisecondsInDay = 24 * 60 * 60 * 1000

func NewTimeOfWorkResource() resource.Resource {
	return &TimeOfWorkResource{}
}

// TimeOfWorkResource restricts an application's tool access to working hours.
type TimeOfWorkResource struct {
	client *client.Client
}

// TimeOfWorkWindowModel is one working-hours window.
type TimeOfWorkWindowModel struct {
	WorkingDays       types.Set   `tfsdk:"working_days"`
	StartWorkingHours types.Int64 `tfsdk:"start_working_hours"`
	EndWorkingHours   types.Int64 `tfsdk:"end_working_hours"`
}

// TimeOfWorkResourceModel is the Terraform state for a time-of-work rule.
type TimeOfWorkResourceModel struct {
	ID            types.String            `tfsdk:"id"`
	ApplicationID types.String            `tfsdk:"application_id"`
	Action        types.String            `tfsdk:"action"`
	IsActive      types.Bool              `tfsdk:"is_active"`
	Windows       []TimeOfWorkWindowModel `tfsdk:"window"`
}

func (r *TimeOfWorkResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_time_of_work"
}

func (r *TimeOfWorkResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Restricts an application's tool access to working hours. One rule exists per application; " +
			"every write replaces the full window set.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Time-of-work rule ID.",
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
			"action": schema.StringAttribute{
				Description: fmt.Sprintf("What happens outside the windows. One of: %v.", timeOfWorkActions),
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(timeOfWorkActions...),
				},
			},
			"is_active": schema.BoolAttribute{
				Description: "Whether enforcement is switched on.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
		},
		Blocks: map[string]schema.Block{
			"window": schema.ListNestedBlock{
				Description: "A working-hours window. At least one is required.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"working_days": schema.SetAttribute{
							Description: "Weekdays the window covers, 0 for Sunday through 6 for Saturday. " +
								"A weekday may appear in only one window across the whole rule.",
							Required:    true,
							ElementType: types.Int64Type,
							Validators: []validator.Set{
								setvalidator.SizeAtLeast(1),
								setvalidator.ValueInt64sAre(int64validator.Between(0, 6)),
							},
						},
						"start_working_hours": schema.Int64Attribute{
							Description: "Window start, as milliseconds from midnight (0 to 86399999). " +
								"For example 09:00 is 32400000.",
							Required: true,
							Validators: []validator.Int64{
								int64validator.Between(0, millisecondsInDay-1),
							},
						},
						"end_working_hours": schema.Int64Attribute{
							Description: "Window end, as milliseconds from midnight (0 to 86399999). Must be " +
								"greater than start_working_hours. For example 17:00 is 61200000.",
							Required: true,
							Validators: []validator.Int64{
								int64validator.Between(1, millisecondsInDay-1),
							},
						},
					},
				},
			},
		},
	}
}

func (r *TimeOfWorkResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *TimeOfWorkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TimeOfWorkResourceModel
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

func (r *TimeOfWorkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TimeOfWorkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	timeOfWork, err := r.client.GetTimeOfWork(ctx, state.ApplicationID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read time-of-work rule", err.Error())
		return
	}
	if timeOfWork == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyTimeOfWork(ctx, &state, timeOfWork, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *TimeOfWorkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan TimeOfWorkResourceModel
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

func (r *TimeOfWorkResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TimeOfWorkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteTimeOfWork(ctx, state.ApplicationID.ValueString())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete time-of-work rule", err.Error())
	}
}

func (r *TimeOfWorkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("application_id"), req, resp)
}

func (r *TimeOfWorkResource) upsert(ctx context.Context, plan *TimeOfWorkResourceModel, diagnostics *diag.Diagnostics) {
	if len(plan.Windows) == 0 {
		diagnostics.AddError(
			"At least one window is required",
			"A time-of-work rule needs at least one window block.",
		)
		return
	}

	windows := make([]client.TimeOfWorkWindow, 0, len(plan.Windows))
	seenDays := map[int64]bool{}
	for index, window := range plan.Windows {
		var workingDays []int64
		diagnostics.Append(window.WorkingDays.ElementsAs(ctx, &workingDays, false)...)

		for _, day := range workingDays {
			if seenDays[day] {
				diagnostics.AddError(
					"Overlapping working days",
					fmt.Sprintf("Weekday %d appears in more than one window. The API requires each weekday to "+
						"belong to at most one window.", day),
				)
			}
			seenDays[day] = true
		}

		start := window.StartWorkingHours.ValueInt64()
		end := window.EndWorkingHours.ValueInt64()
		if start >= end {
			diagnostics.AddError(
				"Invalid working-hours window",
				fmt.Sprintf("Window %d has start_working_hours (%d) at or after end_working_hours (%d).",
					index, start, end),
			)
		}

		windows = append(windows, client.TimeOfWorkWindow{
			WorkingDays:       workingDays,
			StartWorkingHours: start,
			EndWorkingHours:   end,
		})
	}
	if diagnostics.HasError() {
		return
	}

	timeOfWork, err := r.client.UpsertTimeOfWork(
		ctx, plan.ApplicationID.ValueString(), plan.Action.ValueString(), plan.IsActive.ValueBool(), windows,
	)
	if err != nil {
		diagnostics.AddError("Unable to write time-of-work rule", err.Error())
		return
	}

	r.applyTimeOfWork(ctx, plan, timeOfWork, diagnostics)
}

func (r *TimeOfWorkResource) applyTimeOfWork(
	ctx context.Context,
	model *TimeOfWorkResourceModel,
	timeOfWork *client.TimeOfWork,
	diagnostics *diag.Diagnostics,
) {
	model.ID = types.StringValue(timeOfWork.ID)
	model.ApplicationID = types.StringValue(timeOfWork.AppID)
	model.Action = types.StringValue(timeOfWork.Action)
	model.IsActive = types.BoolValue(timeOfWork.IsActive)

	windows := make([]TimeOfWorkWindowModel, 0, len(timeOfWork.Windows))
	for _, window := range timeOfWork.Windows {
		workingDays, diags := types.SetValueFrom(ctx, types.Int64Type, window.WorkingDays)
		diagnostics.Append(diags...)
		windows = append(windows, TimeOfWorkWindowModel{
			WorkingDays:       workingDays,
			StartWorkingHours: types.Int64Value(window.StartWorkingHours),
			EndWorkingHours:   types.Int64Value(window.EndWorkingHours),
		})
	}
	model.Windows = windows
}
