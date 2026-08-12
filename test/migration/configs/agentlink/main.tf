terraform {
  required_providers {
    agentlink = {
      source  = "frontegg/agentlink"
      version = "0.4.7"
    }
    # Present only for the agenco_tools data source below. Every managed object in this stage is
    # created by agentlink.
    agenco = {
      source = "frontegg/agenco"
    }
  }
}

# Credentials and region come from FRONTEGG_CLIENT_ID, FRONTEGG_SECRET and FRONTEGG_REGION.
provider "agentlink" {}
provider "agenco" {}

variable "name_prefix" {
  description = "Unique prefix so repeated runs do not collide."
  type        = string
}

locals {
  # Frontegg validates that URLs resolve in DNS, so every URL in the fixture has to be a real host.
  # example.com is the reserved documentation domain and does resolve; its subdomains do not.
  url = "https://example.com"

  # sort() makes the choice independent of the order the API lists tools in, which matters because
  # the agenco stage re-reads the same list and has to pick the same tool.
  rbac_tool_ids = slice(sort([for tool in data.agenco_tools.migration.tools : tool.id]), 0, 1)
}

# Every value agenco defaults differently is pinned here to agentlink's own default, so the
# migration starts from the state a real agentlink user is actually in.
resource "agentlink_application" "migration" {
  name           = "${var.name_prefix}-app"
  app_url        = local.url
  login_url      = local.url
  type           = "agent"
  frontend_stack = "react"
  access_type    = "FREE_ACCESS"
  is_active      = true
  is_default     = false
  allow_dcr      = false
}

resource "agentlink_mcp_configuration" "migration" {
  application_id = agentlink_application.migration.id
  base_url       = local.url
  api_timeout    = 5000
}

resource "agentlink_source" "migration" {
  application_id = agentlink_application.migration.id
  name           = "${var.name_prefix}-source"
  type           = "REST"
  source_url     = local.url
  api_timeout    = 5000
}

resource "agentlink_tools_import" "migration" {
  application_id = agentlink_application.migration.id
  source_id      = agentlink_source.migration.id
  schema_file    = "${path.module}/openapi.json"
  schema_type    = "openapi"
}

# agentlink rejects an empty internal_tool_ids on RBAC policies but exposes no data source for
# tools, so the IDs are read through agenco.
data "agenco_tools" "migration" {
  application_id = agentlink_application.migration.id
  source_id      = agentlink_source.migration.id

  depends_on = [agentlink_tools_import.migration]
}

resource "agentlink_masking_policy" "migration" {
  name              = "${var.name_prefix}-masking"
  enabled           = true
  app_ids           = [agentlink_application.migration.id]
  internal_tool_ids = []

  policy_configuration = {
    credit_card   = true
    email_address = true
  }
}

resource "agentlink_rbac_policy" "migration" {
  name              = "${var.name_prefix}-rbac"
  enabled           = true
  app_ids           = [agentlink_application.migration.id]
  type              = "rbac-roles"
  keys              = ["Admin"]
  internal_tool_ids = local.rbac_tool_ids
}

# targeting is deliberately omitted: agentlink accepts the attribute but discards it, so there is
# never any targeting on the live object to carry across.
resource "agentlink_conditional_policy" "migration" {
  name              = "${var.name_prefix}-conditional"
  enabled           = true
  app_ids           = [agentlink_application.migration.id]
  internal_tool_ids = []
}

output "migration_ids" {
  description = "IDs the migration step imports into agenco."
  value = {
    application_id = agentlink_application.migration.id
    source_id      = agentlink_source.migration.id
    masking_id     = agentlink_masking_policy.migration.id
    rbac_id        = agentlink_rbac_policy.migration.id
    conditional_id = agentlink_conditional_policy.migration.id
  }
}
