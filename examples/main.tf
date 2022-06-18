terraform {
  required_providers {
    hashicups = {
      source  = "hashicorp.com/edu/hashicups-pf"
    }
  }
}
provider "hashicups" {
/*  username = "education"
  password = "test123"
  host     = "http://localhost:19090"*/
}
/*
resource "hashicups_order" "edu" {
  items = [{
    coffee = {
      id = 3
    }
    quantity = 2
    }, {
    coffee = {
      id = 1
    }
    quantity = 2
    }
  ]
}*/
resource "hashicups_webappBinding" "citeo-plus-binding" {
  name = "mahmoud-backendAddressPool-resource-name"
  agw_name              = "default-app-gateway-mahmoud"
  agw_rg                = "shared-app-gateway"
  backend_address_pool = {
    name = "mahmoud-backendAddressPool-name"
    fqdns = ["fqdn.mahmoud"]
    ip_addresses=["10.2.3.3"]
  }
}
resource "hashicups_webappBinding" "citeo-plus-binding4" {
  name = "mahmoud-backendAddressPool-resource-name4"
  agw_name              = "default-app-gateway-mahmoud"
  agw_rg                = "shared-app-gateway"
  backend_address_pool = {
    name = "mahmoud-backendAddressPool-name4"
    fqdns = ["fqdn.mahmoud.net"]
    ip_addresses=["100.0.0.100"]
  }
}
resource "hashicups_webappBinding" "citeo-plus-binding3" {
  name = "mahmoud-backendAddressPool-resource-name3"
  agw_name              = "default-app-gateway-mahmoud"
  agw_rg                = "shared-app-gateway"
  backend_address_pool = {
    name = "mahmoud-backendAddressPool-name3"
    fqdns = ["fqdn.mahmoud.net"]
    ip_addresses=["100.0.0.100"]
  }
}
resource "hashicups_webappBinding" "citeo-plus-binding2" {
  name = "mahmoud-backendAddressPool-resource-name2"
  agw_name              = "default-app-gateway-mahmoud"
  agw_rg                = "shared-app-gateway"
  backend_address_pool = {
    name = "mahmoud-backendAddressPool-name2"
    fqdns = ["fqdn.mahmoud.net"]
    ip_addresses=["100.0.0.100"]
  }
}
resource "hashicups_webappBinding" "citeo-plus-binding1" {
  name = "mahmoud-backendAddressPool-resource-name1"
  agw_name              = "default-app-gateway-mahmoud"
  agw_rg                = "shared-app-gateway"
  backend_address_pool = {
    name = "mahmoud-backendAddressPool-name1"
    fqdns = ["fqdn.mahmoud.net"]
    ip_addresses=["100.0.0.100"]
  }

}
/*
output "citeo-plus-binding_out" {
  value = hashicups_webappBinding.citeo-plus-binding
}
output "edu_order" {
  value = hashicups_order.edu
}*/
