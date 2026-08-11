# Vendor-wide singleton — declare it at most once across your configuration.
resource "agenco_allowed_origins" "default" {
  origins = [
    "https://storefront.example.com",
    "https://admin.example.com",
  ]
}
