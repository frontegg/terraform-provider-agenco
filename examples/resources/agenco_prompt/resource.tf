resource "agenco_prompt" "refund_playbook" {
  application_id = agenco_application.storefront.id
  name           = "refund-playbook"

  prompt = <<-EOT
    You are handling a refund request. Confirm the order is settled before
    calling createRefund, and never refund more than the order total.
  EOT

  tool_ids = [agenco_tool.refund.id]
}
