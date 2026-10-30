# Terraform 1.10+ ephemeral sensitive variables keep password inputs out of
# plan and state values. Supply values with ephemeral variables at run time.
variable "smtp_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

variable "proxy_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

variable "dyndns_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

resource "qbittorrent_preferences" "webui" {
  # Raw upstream delimiters are retained. Empty lists are explicit values.
  bypass_local_auth                         = false
  bypass_auth_subnet_whitelist_enabled      = false
  bypass_auth_subnet_whitelist              = ""
  web_ui_max_auth_fail_count               = 5
  web_ui_ban_duration                      = 3600
  web_ui_session_timeout                   = 3600
  web_ui_sessions_count_limit              = 15
  web_ui_clickjacking_protection_enabled   = true
  web_ui_csrf_protection_enabled           = true
  web_ui_secure_cookie_enabled             = false
  web_ui_host_header_validation_enabled    = true
  web_ui_domain_list                       = "qbittorrent.example.com"
  web_ui_use_custom_http_headers_enabled   = false
  web_ui_custom_http_headers               = ""
  web_ui_reverse_proxy_enabled             = false
  web_ui_reverse_proxies_list              = ""
  dyndns_enabled                           = false
  dyndns_service                           = 0
  dyndns_domain                            = "downloads.example.com"
  dyndns_username                          = "automation"
}

resource "qbittorrent_credentials" "application" {
  smtp_password_wo         = var.smtp_password
  smtp_password_wo_version = 1
  proxy_password_wo        = var.proxy_password
  proxy_password_wo_version = 1
  dyndns_password_wo       = var.dyndns_password
  dyndns_password_wo_version = 1
}

