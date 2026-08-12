# Treat the agenco_ip_restriction entries as an allow-list.
resource "agenco_ip_restriction_config" "storefront" {
  application_id = agenco_application.storefront.id
  strategy       = "allow"
  is_active      = true
}
