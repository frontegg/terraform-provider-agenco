# A name is all this needs. app_url and login_url are derived from the host Frontegg assigns,
# giving the application its own OAuth endpoints, and base_url and api_timeout are defaulted.
# Right for an agent-only application with no browser UI.
resource "agenco_saas_app" "agent_only" {
  name = "Orders Agent"
}

# Supplied values are never overridden, so point the application at your own URLs when it has a
# browser UI, and at your API when you upsert tools with no source.
resource "agenco_saas_app" "storefront" {
  name      = "Storefront"
  app_url   = "https://storefront.example.com"
  login_url = "https://storefront.example.com/login"
  base_url  = "https://api.example.com"

  behavior_risk_threshold = "medium"
  behavior_risk_actions = {
    low    = "observe"
    medium = "step_up"
    high   = "block"
  }
}

# Sources, tools and policies attach through the exported application_id.
resource "agenco_mcp_source" "orders_api" {
  application_id = agenco_saas_app.agent_only.application_id
  name           = "Orders API"
  type           = "REST"
  source_url     = "https://api.example.com"
}
