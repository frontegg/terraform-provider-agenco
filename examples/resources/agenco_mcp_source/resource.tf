# A REST source whose tools come from an imported OpenAPI document.
resource "agenco_mcp_source" "orders_api" {
  application_id = agenco_application.storefront.id
  name           = "Orders API"
  type           = "REST"
  source_url     = "https://api.example.com"

  # The dashboard sets content-type on REST sources by default.
  override_headers {
    key   = "content-type"
    value = "application/json"
  }

  override_headers {
    key   = "X-Api-Key"
    value = var.orders_api_key
  }
}

# An OAuth-protected upstream MCP server, proxied per user.
resource "agenco_mcp_source" "partner_mcp" {
  application_id             = agenco_application.storefront.id
  name                       = "Partner MCP"
  type                       = "MCP_PROXY"
  source_url                 = "https://mcp.partner.example.com/mcp"
  external_authorization_url = "https://auth.partner.example.com"
  scopes                     = ["read:tools", "write:tools"]
  two_step_callback          = true
}
