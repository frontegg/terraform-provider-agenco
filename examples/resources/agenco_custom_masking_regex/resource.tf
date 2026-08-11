resource "agenco_custom_masking_regex" "internal_order_id" {
  application_id = agenco_application.storefront.id
  name           = "internal_order_id"
  pattern        = "ORD-[0-9]{10}"
  flags          = "g"
  description    = "Internal order identifiers must not reach the model"
}
