resource "agenco_custom_code_tool" "summarize_order" {
  application_id = agenco_application.storefront.id
  name           = "summarize_order"
  description    = "Summarizes an order into a single paragraph"
  runtime        = "NODE_24"

  code_content = file("${path.module}/tools/summarize-order.js")

  input_schema = jsonencode({
    type = "object"
    properties = {
      orderId = { type = "string" }
    }
    required = ["orderId"]
  })
}
