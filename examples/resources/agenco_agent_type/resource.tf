# Claude Code is allowed for everyone.
resource "agenco_agent_type" "claude_code" {
  application_id = agenco_application.storefront.id
  agent_type     = "claude-code"
  all_users      = true
  is_active      = true
}

# ChatGPT is limited to two directory groups.
resource "agenco_agent_type" "chatgpt" {
  application_id = agenco_application.storefront.id
  agent_type     = "chat-gpt"
  all_users      = false
  is_active      = true
  group_ids      = [var.engineering_group_id, var.support_group_id]
}
