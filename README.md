# Run Your Own Password Manager Without Worrying about Hackers

Public password managers are high value targets for hackers. OwnWarden makes it easy to run your own while minimizing the attack surface by using WireGuard VPN.


## Goals
* Self-host [Bitwarden](https://github.com/bitwarden) (or compatible server) on Google Cloud. 
* Make the service as highly secure as possible by:
    - Using the proven WireGuard VPN 
    - Using proven Open Source software whenever possible
    - Automatic security updates on all components
    - Continuous Monitoring
* Make it simple to install and configure
* Installation optimized for Google Cloud's ['always free'](https://cloud.google.com/free/docs/free-cloud-features#free-tier-usage-limits) e2-micro compute instance by using [Vaultwarden](https://github.com/dani-garcia/vaultwarden) (Alternative implementation of the Bitwarden server API written in Rust and compatible with upstream Bitwarden clients).


> _Note: if you follow these instructions the end product is a self-hosted instance of Vaultwarden running in the cloud and will be free **unless** you exceed the 1GB egress per month or have egress to China or Australia. Also it has to be deployed in one of the following regions: Oregon: us-west1, Iowa: us-central1, South Carolina: us-east1
---

## Current Features

### Google Cloud Installation 

* Micro e1-micro VM running the security hardened [Google Container Optimized OS](https://cloud.google.com/container-optimized-os/docs/concepts/features-and-benefits).
 
* [VaultWarden](https://github.com/dani-garcia/vaultwarden) API server accessible **ONLY** via [WireGuard](https://www.wireguard.com/) protocol using [Tailscale](https://tailscale.com).
* Scheduled backups of the encrypted password vault stored in SQLite database to Google Cloud Storage
* Automated updates of Operating System and VaultWarden 
   

## Pre-requisites

Before you start, ensure you have the following:

1. A Google Cloud project. Create one by navigating to https://console.cloud.google.com
2. A Tailscale account and [auth key](https://tailscale.com/kb/1085/auth-keys). This is to allow Vaultwarden to connect to you tailscale network.


## Step 1: Clone and Configure Project

Open Cloud Shell in the GCP Console and enter the following command

```bash
$ git clone https://github.com/abhinavrau/ownwarden.git
$ cd ownwarden
```

