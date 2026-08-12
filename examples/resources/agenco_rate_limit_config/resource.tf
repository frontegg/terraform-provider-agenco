# Every budget is in requests per minute. Omitted maps are cleared on apply.
resource "agenco_rate_limit_config" "storefront" {
  application_id = agenco_application.storefront.id

  sources = {
    (agenco_mcp_source.partner_mcp.id) = 120
  }

  tools = {
    (agenco_tool.refund.id) = 10
  }

  tenants = {
    "11112222-3333-4444-5555-666677778888" = 600
  }
}
