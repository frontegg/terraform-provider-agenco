resource "agenco_application" "storefront" {
  name      = "Storefront"
  app_url   = "https://storefront.example.com"
  login_url = "https://storefront.example.com/login"

  description = "Customer-facing storefront exposed to agents"
}
