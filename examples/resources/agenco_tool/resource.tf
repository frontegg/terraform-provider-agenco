data "agenco_tools" "orders" {
  application_id = agenco_application.storefront.id
  source_id      = agenco_mcp_source.orders_api.id
}

locals {
  refund_tool_id = one([
    for tool in data.agenco_tools.orders.tools : tool.id if tool.name == "createRefund"
  ])
}

# Tighten a single imported tool without touching the rest of the import.
resource "agenco_tool" "refund" {
  id             = local.refund_tool_id
  application_id = agenco_application.storefront.id

  description         = "Issues a refund against a settled order"
  authentication_type = "Authenticated"
  is_active           = true
}
