# Enable every tool the partner MCP server advertises.
resource "agenco_source_tools_active_status" "partner" {
  application_id = agenco_application.storefront.id
  source_id      = agenco_mcp_source.partner_mcp.id
  is_active      = true
}
