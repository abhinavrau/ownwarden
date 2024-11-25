# Run Your Own Password Manager Without Worrying about Hackers

Internet facing SaaS password managers are high value targets for hackers. OwnWarden makes it easy to run your own open source password manager [VaultWarden](https://github.com/dani-garcia/vaultwarden) protected by the highly secure and battle tested WireGuard overlay network protocol.


## Installation Options

- Synology NAS
- Google Cloud 

## Design

It's a 

## Install on Synology NAS

Before you start, ensure you have the following:

1. Synology NAS with SSH access configured. 
2. A free [Tailscale](https://tailscale.com) account with an [auth key](https://tailscale.com/kb/1085/auth-keys). This is to allow Vaultwarden to connect to you tailscale network.


## Install on Google Cloud 

 **Note:** if you follow these instructions the end product is a self-hosted instance of Vaultwarden 
 running in the cloud and will be free **unless** you exceed the 1GB egress (very unlikely) per month or have egress to China or Australia. Also it has to be deployed in one of the following regions: Oregon: us-west1, Iowa: us-central1, South Carolina: us-east1


* Micro e1-micro VM running the security hardened [Google Container Optimized OS](https://cloud.google.com/container-optimized-os/docs/concepts/features-and-benefits).
* [VaultWarden](https://github.com/dani-garcia/vaultwarden) API server accessible **ONLY** via [WireGuard](https://www.wireguard.com/) protocol using [Tailscale](https://tailscale.com).
* Scheduled backups of the encrypted password vault stored in SQLite database to Google Cloud Storage
* Automated updates of Operating System and VaultWarden 
   

## Pre-requisites

Before you start, ensure you have the following:

1. A Google Cloud project. Create one by navigating to https://console.cloud.google.com 
2. A free [Tailscale](https://tailscale.com) account and create an [auth key](https://tailscale.com/kb/1085/auth-keys). This is to allow Vaultwarden to connect to you tailscale network.
3. Terraform 1.5.7+
4. Google Cloud SDK (gcloud) installed and authenticated to your GCP project



## Step 1: Clone and Configure Project

```bash
$ git clone https://github.com/abhinavrau/ownwarden.git
$ cd ownwarden
```
## Step 2: Configure .env file with enviroment variables for Docker compose  


You will need the following information:

### Tailscale Info
- TAILSCALE_AUTH_KEY - Tailscale Auth key generated using in the tailscale console to add the ownwarden instance to your tailscale netowrk
- TAILSCALE_HOSTNAME - Hostname to use within  tailscale network



## Goals
* Self-host an open source (a Bitwarden Compatibale server) on Google Cloud. 
* Make the service as highly secure as possible by:
    - Using the proven WireGuard VPN 
    - Using proven Open Source software whenever possible
    - Automatic security updates on all components
    - Continuous Monitoring
* Make it simple to install and configure
* Installation optimized for Google Cloud's ['always free'](https://cloud.google.com/free/docs/free-cloud-features#free-tier-usage-limits) e2-micro compute instance by using [Vaultwarden](https://github.com/dani-garcia/vaultwarden) (Alternative implementation of the Bitwarden server API written in Rust and compatible with upstream Bitwarden clients).

