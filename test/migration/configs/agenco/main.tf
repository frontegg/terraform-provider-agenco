terraform {
  required_providers {
    agenco = {
      source = "frontegg/agenco"
    }
  }
}

# No version constraint: the provider is supplied by dev_overrides in dev.tfrc, which bypasses
# both the registry and the lock file.
provider "agenco" {}

variable "name_prefix" {
  description = "Unique prefix so repeated runs do not collide."
  type        = string
}

locals {
  # Frontegg validates that URLs resolve in DNS, so every URL in the fixture has to be a real host.
  # example.com is the reserved documentation domain and does resolve; its subdomains do not.
  url = "https://example.com"

  # Must select the same tool the agentlink stage did, so the sort is not optional.
  rbac_tool_ids = slice(sort([for tool in data.agenco_tools.migration.tools : tool.id]), 0, 1)
}

# Everything the agentlink stage set explicitly — type, allow_dcr, frontend_stack, access_type,
# is_active, is_default — is deliberately left out here. agenco defaults type to web and allow_dcr
# to true, the opposite of the imported values, so if any default were applied on update rather than
# only on create, this application would be rewritten. That is what assert-preserved checks.
resource "agenco_application" "migration" {
  name      = "${var.name_prefix}-app"
  app_url   = local.url
  login_url = local.url
}

resource "agenco_mcp_configuration" "migration" {
  application_id = agenco_application.migration.id
  base_url       = local.url
  api_timeout    = 5000
}

# Renamed from agentlink_source. vendor_id is computed now, so it is not set here.
resource "agenco_mcp_source" "migration" {
  application_id = agenco_application.migration.id
  name           = "${var.name_prefix}-source"
  type           = "REST"
  source_url     = local.url
  api_timeout    = 5000
}

resource "agenco_tools_import" "migration" {
  application_id = agenco_application.migration.id
  source_id      = agenco_mcp_source.migration.id
  schema_file    = "${path.module}/openapi.json"
  schema_type    = "openapi"
}

data "agenco_tools" "migration" {
  application_id = agenco_application.migration.id
  source_id      = agenco_mcp_source.migration.id

  depends_on = [agenco_tools_import.migration]
}

# app_ids became application_ids, and the per-detector booleans became one detectors set.
resource "agenco_masking_policy" "migration" {
  name              = "${var.name_prefix}-masking"
  enabled           = true
  application_ids   = [agenco_application.migration.id]
  internal_tool_ids = []

  detectors = ["credit_card", "email_address"]
}

resource "agenco_rbac_policy" "migration" {
  name              = "${var.name_prefix}-rbac"
  enabled           = true
  application_ids   = [agenco_application.migration.id]
  type              = "rbac-roles"
  keys              = ["Admin"]
  internal_tool_ids = local.rbac_tool_ids
}

resource "agenco_conditional_policy" "migration" {
  name              = "${var.name_prefix}-conditional"
  enabled           = true
  application_ids   = [agenco_application.migration.id]
  internal_tool_ids = []
}

output "migration_ids" {
  description = "IDs after migration; the script asserts these match the agentlink run."
  value = {
    application_id = agenco_application.migration.id
    source_id      = agenco_mcp_source.migration.id
    masking_id     = agenco_masking_policy.migration.id
    rbac_id        = agenco_rbac_policy.migration.id
    conditional_id = agenco_conditional_policy.migration.id
  }
}
