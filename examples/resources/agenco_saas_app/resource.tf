# A name is all this needs. login_url and app_url are derived from the host Frontegg assigns,
# base_url and api_timeout are defaulted, and allow_dcr is sent as true so MCP clients can
# register themselves.
resource "agenco_saas_app" "orders_agent" {
  name = "Orders Agent"
}

# Any value you do supply is used verbatim and never overridden, so point the application at your
# own URLs when it has a browser UI.
resource "agenco_saas_app" "storefront" {
  name      = "Storefront"
  app_url   = "https://storefront.example.com"
  login_url = "https://storefront.example.com/login"
}

# Everything downstream hangs off the exported application_id: a REST source, tools imported from
# an OpenAPI document, and a masking policy over all of them.
resource "agenco_mcp_source" "orders_api" {
  application_id = agenco_saas_app.storefront.application_id
  name           = "Orders API"
  type           = "REST"
  source_url     = "https://api.example.com"
}

resource "agenco_tools_import" "orders_api" {
  application_id = agenco_saas_app.storefront.application_id
  source_id      = agenco_mcp_source.orders_api.id
  schema_file    = "${path.module}/schemas/orders-openapi.json"
  schema_type    = "openapi"
}

resource "agenco_masking_policy" "strip_pii" {
  name              = "Strip PII from tool responses"
  enabled           = true
  application_ids   = [agenco_saas_app.storefront.application_id]
  internal_tool_ids = [] # empty means every tool

  detectors = ["credit_card", "email_address", "phone_number", "us_ssn"]

  targeting {
    then {
      result = "mask"
    }
  }
}
