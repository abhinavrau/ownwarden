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


variable "project_id" {
    type = string
}
variable "env" {
    default = "dev"
    type = string
}
variable "region" {
    default = "us-east1"
    type = string
}
variable "zone" {
    default = "us-east1-b"
}
variable "timezone" {
    default = "America/New_York"
    type = string
}
variable "vm_hostname" {
    default = "ownwarden-vm"
    type = string
}
variable "vpc_name" {
    default = "ownwarden-vpc"
    type = string
}
variable "subnet_name" {
    default = "ownwarden-subnet"
    type = string
}
variable "subnet_ip" {
    type = string
    default = "10.10.0.0/24"
}
variable "tailscale_hostname" {
    type = string
}
variable "tailscale_domain" {
    type = string
}
variable "tailscale_auth_key" {
    type = string
    sensitive = true
}
variable "artifact_registry_repo_id" {
    type = string
    default = "ownwarden"
}