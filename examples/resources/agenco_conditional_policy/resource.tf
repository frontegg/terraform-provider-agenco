# Refunds outside the EU need an approval flow; everything else is allowed.
resource "agenco_conditional_policy" "refund_approval" {
  name    = "Refunds need approval outside the EU"
  enabled = true

  application_ids   = [agenco_application.storefront.id]
  internal_tool_ids = [agenco_tool.refund.id]

  targeting {
    if {
      condition_logic = "and"

      condition {
        attribute = "country"
        op        = "in_list"
        negate    = true
        value     = jsonencode({ list = ["DE", "FR", "NL", "IE"] })
      }
    }

    then {
      result           = "approval_required"
      approval_flow_id = var.refund_approval_flow_id
    }
  }
}

# Nested groups express "(A and B) or C".
resource "agenco_conditional_policy" "step_up_risky" {
  name    = "Step up risky sessions"
  enabled = true

  application_ids   = [agenco_application.storefront.id]
  internal_tool_ids = []

  targeting {
    if {
      condition_logic = "or"

      condition_group {
        condition_logic = "and"

        condition {
          attribute = "user-agent"
          op        = "contains"
          negate    = false
          value     = jsonencode({ list = ["curl"] })
        }

        condition {
          attribute = "tool-name"
          op        = "starts_with"
          negate    = false
          value     = jsonencode({ list = ["delete"] })
        }
      }

      condition {
        attribute = "country"
        op        = "in_list"
        negate    = false
        value     = jsonencode({ list = ["RU", "KP"] })
      }
    }

    then {
      result               = "step_up"
      step_up_action_async = true
    }
  }
}
