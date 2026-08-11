resource "agenco_tools_import" "orders_api" {
  application_id = agenco_application.storefront.id
  source_id      = agenco_mcp_source.orders_api.id
  schema_file    = "${path.module}/schemas/orders-openapi.json"
  schema_type    = "openapi"
}
