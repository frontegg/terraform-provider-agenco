resource "agenco_geo_fence" "eu_only" {
  application_id = agenco_application.storefront.id
  countries      = ["DE", "FR", "NL", "IE", "ES", "IT"]
  description    = "EU-only access"
}
