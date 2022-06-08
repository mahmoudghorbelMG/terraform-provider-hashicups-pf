terraform {
  required_providers {
    hashicups = {
      source  = "hashicorp.com/edu/hashicups-pf"
    }
  }
  required_version = ">= 1.1.0"
}

provider "hashicups" {
  username = "education"
  password = "test123"
  host     = "http://localhost:19090"
}

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
}

resource "hashicups_webappBinding" "citeo-plus-binding" {
  Name = "citeo-plus-binding-resource-name"
  backend_address_pool {
    Name = "default-citeo-plus-be-pool"
    Fqdns = "default-citeo-plus.azurewebsites.net"
  }

}
output "citeo-plus-binding_out" {
  value = hashicups_webappBinding.citeo-plus-binding
}
output "edu_order" {
  value = hashicups_order.edu
}