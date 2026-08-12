# Expose Frontegg's own tenant-management tools to agents.
resource "agenco_frontegg_tools_source" "tenant_tools" {
  application_id = agenco_application.storefront.id
  is_active      = true
}
