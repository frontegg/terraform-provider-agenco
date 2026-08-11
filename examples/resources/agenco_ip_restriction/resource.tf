resource "agenco_ip_restriction" "office" {
  application_id = agenco_application.storefront.id
  ip             = "203.0.113.0/24"
  description    = "Amsterdam office"
}

resource "agenco_ip_restriction" "vpn" {
  application_id = agenco_application.storefront.id
  ip             = "198.51.100.7"
  description    = "Corporate VPN egress"
}
