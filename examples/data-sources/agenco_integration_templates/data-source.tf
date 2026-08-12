data "agenco_integration_templates" "available" {}

output "template_ids" {
  value = [for template in data.agenco_integration_templates.available.templates : template.id]
}
