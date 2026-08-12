package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

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
	_ resource.Resource                = &ToolsImportResource{}
	_ resource.ResourceWithImportState = &ToolsImportResource{}
)

// schemaTypes are the schema formats the API can import tools from.
var schemaTypes = []string{"openapi", "graphql"}

func NewToolsImportResource() resource.Resource {
	return &ToolsImportResource{}
}

// ToolsImportResource imports tools into a source from an OpenAPI or GraphQL document.
type ToolsImportResource struct {
	client *client.Client
}

// ToolsImportResourceModel is the Terraform state for a tools import.
type ToolsImportResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ApplicationID types.String `tfsdk:"application_id"`
	SourceID      types.String `tfsdk:"source_id"`
	SchemaFile    types.String `tfsdk:"schema_file"`
	SchemaType    types.String `tfsdk:"schema_type"`
	SchemaHash    types.String `tfsdk:"schema_hash"`
	ToolsCount    types.Int64  `tfsdk:"tools_count"`
}

func (r *ToolsImportResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tools_import"
}

func (r *ToolsImportResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Imports tools into an MCP source from an OpenAPI or GraphQL document and activates them. " +
			"The import is re-run whenever the file contents change. Destroying the resource deletes the " +
			"tools that were imported into the source.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Synthetic ID, formed as application_id/source_id.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_id": schema.StringAttribute{
				Description: "Application the tools are imported into.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"source_id": schema.StringAttribute{
				Description: "Source the imported tools are attached to.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schema_file": schema.StringAttribute{
				Description: "Path to the OpenAPI or GraphQL document to import.",
				Required:    true,
			},
			"schema_type": schema.StringAttribute{
				Description: fmt.Sprintf("Format of the document. One of: %v.", schemaTypes),
				Required:    true,
				Validators: []validator.String{
					stringvalidator.OneOf(schemaTypes...),
				},
			},
			"schema_hash": schema.StringAttribute{
				Description: "SHA-256 of the imported document, used to detect content changes.",
				Computed:    true,
			},
			"tools_count": schema.Int64Attribute{
				Description: "Number of tools written by the last import.",
				Computed:    true,
			},
		},
	}
}

func (r *ToolsImportResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureClient(req.ProviderData, &resp.Diagnostics)
}

func (r *ToolsImportResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ToolsImportResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.runImport(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read refreshes the hash from disk so a changed document shows up as a diff.
func (r *ToolsImportResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ToolsImportResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// After an import there is no schema_file in state — it lives only in configuration — so
	// there is nothing to hash yet. Leave the hash unset rather than warning about a path the
	// user never gave us.
	if state.SchemaFile.IsNull() || state.SchemaFile.ValueString() == "" {
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	contents, err := os.ReadFile(state.SchemaFile.ValueString())
	if err != nil {
		// A missing file is not drift in the remote object; leave state as-is and let the
		// plan surface the problem when the import next runs.
		resp.Diagnostics.AddWarning(
			"Schema file could not be read",
			fmt.Sprintf("%s: %s. The recorded schema_hash was kept.", state.SchemaFile.ValueString(), err.Error()),
		)
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	state.SchemaHash = types.StringValue(hashContents(contents))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ToolsImportResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ToolsImportResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.runImport(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ToolsImportResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ToolsImportResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	toolType := "REST"
	if state.SchemaType.ValueString() == "graphql" {
		toolType = "GRAPHQL"
	}

	err := r.client.DeleteToolsByType(ctx, state.ApplicationID.ValueString(), toolType)
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete imported tools", err.Error())
	}
}

// ImportState accepts "application_id/source_id" and adopts an import that already happened,
// without calling the API. schema_file and schema_type cannot be recovered — the API stores no
// record of which document produced a tool — so they must be present in configuration, and
// schema_hash stays unset until the next apply re-runs the import.
func (r *ToolsImportResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	segments, err := splitImportID(req.ID, 2)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			fmt.Sprintf("Expected \"application_id/source_id\": %s", err.Error()),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("application_id"), segments[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("source_id"), segments[1])...)

	resp.Diagnostics.AddWarning(
		"Imported tools_import will re-run on the next apply",
		"schema_file and schema_type are not recoverable from the API, so schema_hash could not be "+
			"restored. Add both to your configuration; the next apply re-imports the document and "+
			"upserts the tools. That rewrites tool definitions for this source but creates nothing new "+
			"and deletes nothing.",
	)
}

func (r *ToolsImportResource) runImport(ctx context.Context, plan *ToolsImportResourceModel, diagnostics *diag.Diagnostics) {
	schemaPath := plan.SchemaFile.ValueString()
	contents, err := os.ReadFile(schemaPath)
	if err != nil {
		diagnostics.AddError(
			"Unable to read schema file",
			fmt.Sprintf("%s: %s", schemaPath, err.Error()),
		)
		return
	}

	count, err := r.client.ImportAndUpsertTools(
		ctx,
		plan.ApplicationID.ValueString(),
		plan.SourceID.ValueString(),
		plan.SchemaType.ValueString(),
		contents,
		filepath.Base(schemaPath),
	)
	if err != nil {
		diagnostics.AddError("Unable to import tools", err.Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%s/%s", plan.ApplicationID.ValueString(), plan.SourceID.ValueString()))
	plan.SchemaHash = types.StringValue(hashContents(contents))
	plan.ToolsCount = types.Int64Value(int64(count))
}

func hashContents(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}
