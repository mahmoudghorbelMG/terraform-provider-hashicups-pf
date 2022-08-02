#!/usr/bin/env bash
cd ..
git pull
go install
cd exemples
terraform plan
terraform apply