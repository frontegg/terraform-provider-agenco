resource "agenco_agent" "nightly_reconciler" {
  application_id = agenco_application.storefront.id
  name           = "nightly-reconciler"
  autonomous     = true
  description    = "Reconciles settled orders against the ledger every night"
  owner_email    = "platform@example.com"
  tags           = ["batch", "finance"]
}

output "reconciler_client_id" {
  value = agenco_agent.nightly_reconciler.credentials_id
}

output "reconciler_client_secret" {
  value     = agenco_agent.nightly_reconciler.client_secret
  sensitive = true
}
