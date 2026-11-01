terraform {
  required_providers {
    qbittorrent = {
      source = "registry.terraform.io/m11s-io/qbittorrent"
    }
  }
}

variable "provider_url" {
  type        = string
  description = "Current WebUI URL reachable by the Terraform runner."
}

variable "provider_username" {
  type        = string
  description = "Current WebUI session username."
}

variable "provider_password" {
  type        = string
  sensitive   = true
  ephemeral   = true
  description = "Current WebUI session password."
}

variable "target_webui_url" {
  type        = string
  description = "WebUI URL reachable after the listener change."
}

variable "target_webui_username" {
  type        = string
  description = "Target WebUI username."
}

variable "target_webui_password" {
  type        = string
  sensitive   = true
  ephemeral   = true
  description = "Target WebUI password."
}

provider "qbittorrent" {
  url      = var.provider_url
  username = var.provider_username
  password = var.provider_password
}

resource "qbittorrent_webui_endpoint" "listener" {
  web_ui_address         = "127.0.0.1"
  web_ui_port            = 18081
  use_https              = false
  web_ui_https_cert_path = "/config/webui.crt"
  web_ui_https_key_path  = "/config/webui.key"
  post_update_url        = var.target_webui_url
}

resource "qbittorrent_webui_auth" "admin" {
  username            = var.target_webui_username
  password_wo         = var.target_webui_password
  password_wo_version = 1
}
