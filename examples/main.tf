terraform {
  required_providers {
    hashicups = {
      source  = "hashicorp.com/edu/hashicups-pf"
    }
  }
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
  name = "citeo-plus-binding-resource-name"
  backend_address_pool = {
    name = "default-citeo-plus-be-pool"
    fqdns = ["default-citeo-plus.azurewebsites.net"]
  }

}
output "citeo-plus-binding_out" {
  value = hashicups_webappBinding.citeo-plus-binding
}
output "edu_order" {
  value = hashicups_order.edu
}
