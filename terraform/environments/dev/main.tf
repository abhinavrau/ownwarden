# Copyright 2019 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


locals {
  env = "dev"
}

provider "google" {
  project = var.project_id
}

module "project-services" {
  source  = "terraform-google-modules/project-factory/google//modules/project_services"
  version = "~> 14.2"

  project_id = var.project_id

  activate_apis = [
    "compute.googleapis.com",
    "iam.googleapis.com",
    "storage.googleapis.com",
    "cloudresourcemanager.googleapis.com",
    "cloudbilling.googleapis.com",
    "serviceusage.googleapis.com"
  ]
  disable_services_on_destroy = false
  disable_dependent_services = false  
}


module "vpc" {
  source  = "../../modules/vpc"
  project_id = var.project_id
  env     = local.env
  region = var.region
  subnet_name = var.subnet_name
  vpc_name = var.vpc_name
  subnet_ip = var.subnet_ip
  
  depends_on = [
    module.project-services
  ]
  
}

module "ownwarden_vm" {
  source  = "../../modules/free_tier_vm"
  project_id = var.project_id
  vpc_name = var.vpc_name
  subnet  = module.vpc.subnet
  region = var.region
  zone = var.zone
  timezone =  var.timezone
  hostname = var.vm_hostname
 
  tailscale_hostname = var.tailscale_hostname
  tailscale_domain = var.tailscale_domain
  tailscale_auth_key = var.tailscale_auth_key

  depends_on = [
    module.project-services
  ]
}

