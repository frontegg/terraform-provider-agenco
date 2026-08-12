resource "agenco_rbac_policy" "refund_admins" {
  name    = "Only refund admins may issue refunds"
  enabled = true
  type    = "rbac-roles"
  keys    = ["refund-admin", "support-lead"]

  application_ids   = [agenco_application.storefront.id]
  internal_tool_ids = [agenco_tool.refund.id]
}
