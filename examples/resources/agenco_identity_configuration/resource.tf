# Vendor-wide singleton — declare it at most once across your configuration.
resource "agenco_identity_configuration" "default" {
  default_token_expiration = 3600
}
