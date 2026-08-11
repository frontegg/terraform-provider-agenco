# Treat the agenco_geo_fence country list as an allow-list.
resource "agenco_geo_fence_config" "storefront" {
  application_id = agenco_application.storefront.id
  strategy       = "allow"
  is_active      = true
}
