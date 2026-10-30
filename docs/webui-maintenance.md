# WebUI maintenance

Run endpoint and credential changes as separate targeted applies. The provider
configuration is a snapshot for each Terraform process; after a successful
transition, set `QBITTORRENT_URL` / `QBITTORRENT_USERNAME` /
`QBITTORRENT_PASSWORD` to the target values before the next apply. Keep TLS
verification and qBittorrent's authentication, CSRF, and host checks enabled.

## Change the listener

```hcl
resource "qbittorrent_webui_endpoint" "listener" {
  web_ui_address        = "127.0.0.1"
  web_ui_port           = 18081
  use_https             = false
  web_ui_https_cert_path = "/config/webui.crt"
  web_ui_https_key_path  = "/config/webui.key"

  # Address reachable by the provider after qBittorrent applies this update.
  post_update_url = "http://127.0.0.1:18081"
}
```

Apply only this maintenance resource:

```sh
terraform apply -target=qbittorrent_webui_endpoint.listener
```

The provider sends the preference patch once, then uses a separate client to
reconnect and verify the target settings. It never retries the mutation. If
reconnect fails, verify the qBittorrent listener and set the provider URL to
`post_update_url` manually before a targeted follow-up apply. Do not change a
Kubernetes Service or publish a port automatically.

## Change WebUI credentials

```hcl
resource "qbittorrent_webui_auth" "admin" {
  username             = "operator"
  password_wo          = var.webui_password
  password_wo_version  = 1
}
```

Apply only the auth resource. The password must be at least six characters;
qBittorrent requires usernames to have at least three characters and no colon.
After success, update the provider credentials before a later apply. Bearer
API-key authentication cannot change its own WebUI username or password.
