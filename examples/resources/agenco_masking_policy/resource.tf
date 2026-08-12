resource "agenco_masking_policy" "strip_pii" {
  name    = "Strip PII from tool responses"
  enabled = true

  application_ids   = [agenco_application.storefront.id]
  internal_tool_ids = []

  detectors = [
    "credit_card",
    "cvv_cvc",
    "email_address",
    "phone_number",
    "us_ssn",
    "iban_code",
  ]

  custom_masking_regex_ids = [agenco_custom_masking_regex.internal_order_id.id]

  targeting {
    then {
      result = "mask"
    }
  }
}
