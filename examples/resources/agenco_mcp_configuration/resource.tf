# base_url and api_timeout are optional; the gateway replaces base_url with each source's own
# source_url, so it only matters for tools upserted without a source.
resource "agenco_mcp_configuration" "storefront" {
  application_id = agenco_application.storefront.id
}

resource "agenco_mcp_configuration" "tuned" {
  application_id = agenco_application.console.id
  base_url       = "https://api.example.com"
  api_timeout    = 3000

  enable_advanced_tools     = true
  integration_tools_enabled = true

  behavior_risk_threshold = "medium"
  behavior_risk_actions = {
    low    = "observe"
    medium = "step_up"
    high   = "block"
  }

  # Agents that cannot page through tools/list get a bounded first page.
  list_tool_page_size = 200
}
