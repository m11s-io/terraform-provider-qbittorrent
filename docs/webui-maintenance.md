# WebUI maintenance

Run endpoint and credential changes as separate targeted applies. The provider
configuration is a snapshot for each Terraform process; after a successful
transition, set `QBITTORRENT_URL` / `QBITTORRENT_USERNAME` /
`QBITTORRENT_PASSWORD` to the target values before the next apply. Keep TLS
verification and qBittorrent's authentication, CSRF, and host checks enabled.

## Change the listener

See [the standalone staged maintenance example](../examples/maintenance/webui.tf).
It declares required provider and target credentials as variables and marks
password variables ephemeral and sensitive.

Apply only this maintenance resource:

```sh
terraform apply -target=qbittorrent_webui_endpoint.listener
```

The provider checks current values before sending a patch, sends each mutation
at most once per apply, and uses a separate client to verify the target
settings. If reconnect fails, Terraform retains the planned state with a
warning instead of tainting the resource. Set the provider URL to
`post_update_url` and refresh. Refresh reads the actual listener values; review
the resulting targeted plan and apply any remaining changes. It does not
replay a successful or partially applied write automatically. Do not change a
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
After success, update the provider credentials before a later apply. If
reauthentication fails, configure credentials that currently work and refresh;
the resource reads the nonsecret username and never reads the password back.
Increment the password version only for an intentional retry or rotation.
Bearer API-key authentication cannot change its own WebUI username or
password.
