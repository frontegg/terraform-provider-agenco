data "agenco_tools" "orders" {
  application_id = data.agenco_application.storefront.id
  source_id      = agenco_mcp_source.orders_api.id
}

output "order_tool_names" {
  value = [for tool in data.agenco_tools.orders.tools : tool.name]
}
