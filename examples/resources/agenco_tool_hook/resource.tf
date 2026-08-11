# Filter what tools/list returns before agents ever see it.
resource "agenco_tool_hook" "list_filter" {
  application_id = agenco_application.storefront.id
  hook_type      = "LIST_TOOLS"
  runtime        = "NODE_24"
  fail_method    = "CLOSE"
  timeout        = 5

  code = file("${path.module}/hooks/filter-list-tools.js")
}
