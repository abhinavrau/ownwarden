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
  network = element(split("-", var.subnet), 0)
}

// Startup script that reboots VM when update is applied
data "template_file" "default" {
  template = file("${path.module}/startup.sh")
  vars = {
        TZ = var.timezone
    }
}

data template_file "cloud-init" {
  template = file("${path.module}/cloud-config.yaml")

  vars = {
    tailscale_hostname   = var.tailscale_hostname
    tailscale_domain     = var.tailscale_domain
    tailscale_auth_key   = var.tailscale_auth_key
    timezone = var.timezone
  }
}

data "cloudinit_config" "conf" {
  gzip = false
  base64_encode = false

  part {
    content_type = "text/cloud-config"
    content = data.template_file.cloud-init.rendered
    filename = "cloud-config.yaml"
  }
}

resource "google_compute_address" "ip_address" {
  name = "external-ip"
  region = var.region
}

locals {
  access_config = {
    nat_ip       = google_compute_address.ip_address.address
    network_tier = "PREMIUM"
  }
}

module "service_accounts" {
  source        = "terraform-google-modules/service-accounts/google"
  version       = "~> 4.2"
  project_id    = var.project_id
  description   = "Service accont for Terraform"
  prefix        = "ownwarden"
  names         = ["compute"]
  project_roles = [
    "${var.project_id}=>roles/viewer",
    "${var.project_id}=>roles/storage.objectViewer",
     "${var.project_id}=>roles/storage.objectCreator",
    "${var.project_id}=>roles/iam.serviceAccountUser",
    "${var.project_id}=>roles/compute.instanceAdmin.v1",
    "${var.project_id}=>roles/logging.logWriter",
  ]
}

module "instance_template" {
  source  = "terraform-google-modules/vm/google//modules/instance_template"
  version = "8.0.1"

  project_id            = var.project_id
  subnetwork            = var.subnet
  stack_type            = "IPV4_ONLY"
  service_account       = { email = "${module.service_accounts.service_account.email}", 
                            scopes = ["https://www.googleapis.com/auth/logging.write", "https://www.googleapis.com/auth/monitoring.write"]}
  enable_shielded_vm    = true
  machine_type          = "e2-micro"
  metadata = {
    user-data = "${data.cloudinit_config.conf.rendered}"
    google-logging-enabled = true
    google-logging-use-fluentbit = true    
  }
  region                = var.region
  source_image_family   = "cos-105-lts" // Use Google's container optimized OS
  source_image_project  = "cos-cloud"
  tags   = [var.hostname]

}

module "compute_instance" {
  source  = "terraform-google-modules/vm/google//modules/compute_instance"
  version = "8.0.1"

  region              = var.region
  zone                = var.zone
  subnetwork          = var.subnet
  num_instances       = 1
  hostname            = var.hostname
  instance_template   = module.instance_template.self_link
  deletion_protection = false
  access_config       = [local.access_config]

  

}

resource "google_compute_firewall" "allow-ssh" {
  name    = "${local.network}-allow-ssh"
  network =  var.vpc_name
  project = "${var.project_id}"
  description = "Creates firewall rule for ssh"

  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  target_tags   = [var.hostname]
  source_ranges = ["0.0.0.0/0"]
}


