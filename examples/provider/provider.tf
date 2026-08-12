terraform {
  required_providers {
    agenco = {
      source  = "frontegg/agenco"
      version = "~> 1.0"
    }
  }
}

# Credentials are read from FRONTEGG_CLIENT_ID and FRONTEGG_SECRET when omitted here.
provider "agenco" {
  region = "eu"
}
