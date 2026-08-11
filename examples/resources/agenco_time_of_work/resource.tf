locals {
  # Window offsets are milliseconds from midnight.
  hour = 60 * 60 * 1000
}

resource "agenco_time_of_work" "business_hours" {
  application_id = agenco_application.storefront.id
  action         = "step_up"
  is_active      = true

  # Monday to Thursday, 09:00 to 18:00.
  window {
    working_days        = [1, 2, 3, 4]
    start_working_hours = 9 * local.hour
    end_working_hours   = 18 * local.hour
  }

  # Friday closes earlier. A weekday may appear in only one window.
  window {
    working_days        = [5]
    start_working_hours = 9 * local.hour
    end_working_hours   = 15 * local.hour
  }
}
