#!/usr/bin/env bash
cd ~/terraform-learn/terraform-provider-hashicups-pf
git pull
go install
cd exemples
terraform plan
terraform apply