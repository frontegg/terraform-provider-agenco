data "agenco_integration_templates" "available" {}

resource "agenco_custom_integration" "slack_sales" {
  application_id          = agenco_application.storefront.id
  integration_template_id = "slack"
  name                    = "Slack — Sales"

  # A slug lets a second Slack workspace live alongside this one.
  slug = "sales"

  api_names = ["chat", "conversations"]

  auth_type     = "oauth"
  client_id     = var.slack_client_id
  client_secret = var.slack_client_secret

  custom_configuration = jsonencode({
    defaultChannel = "#sales"
  })
}
