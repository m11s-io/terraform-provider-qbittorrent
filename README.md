# Terraform provider for qBittorrent

Manage qBittorrent settings with Terraform: application preferences, categories,
watched folders, RSS rules, and explicit WebUI maintenance.

[Terraform Registry](https://registry.terraform.io/providers/m11s-io/qbittorrent/latest)
· [Provider documentation](docs/index.md)
· [Examples](examples)

## Requirements

- qBittorrent **5.3.0**, Web API **2.16.2**. Other versions are rejected.
- A reachable WebUI with an existing API key or username and password.
- Terraform **1.11 or later** for resources with write-only password inputs.

OpenTofu is also tested for compatibility.

## Quick start

```hcl
terraform {
  required_providers {
    qbittorrent = {
      source  = "m11s-io/qbittorrent"
      version = "~> 0.3"
    }
  }
}

provider "qbittorrent" {
  url = "https://qbittorrent.example.com"
}

resource "qbittorrent_preferences" "application" {
  save_path = "/downloads"
  up_limit  = 1048576 # bytes per second; 0 means unlimited
}

resource "qbittorrent_category" "movies" {
  name      = "movies"
  save_path = "/downloads/movies"
}
```

Provide credentials through the environment, then run `terraform init` and
`terraform plan`:

- `QBITTORRENT_API_KEY` for Bearer authentication; or
- `QBITTORRENT_USERNAME` and `QBITTORRENT_PASSWORD` for a session.

Do not configure both authentication methods. `QBITTORRENT_URL` can provide the
WebUI URL instead of `url`. Explicit provider attributes override the corresponding
environment variables. TLS certificates are verified.

## Resources

| Resource | Purpose |
| --- | --- |
| [qbittorrent_preferences](docs/resources/preferences.md) | Application settings with partial ownership |
| [qbittorrent_webui_preferences](docs/resources/webui_preferences.md) | Server-stored WebUI appearance and interaction settings |
| [qbittorrent_category](docs/resources/category.md) | Download categories |
| [qbittorrent_watch_folders](docs/resources/watch_folders.md) | The complete watched-folder collection |
| [qbittorrent_rss_rule](docs/resources/rss_rule.md) | One named RSS auto-download rule |
| [qbittorrent_credentials](docs/resources/credentials.md) | Write-only SMTP, proxy, and DynDNS passwords |
| [qbittorrent_webui_endpoint](docs/resources/webui_endpoint.md) | Explicit listener and HTTPS maintenance |
| [qbittorrent_webui_auth](docs/resources/webui_auth.md) | Explicit WebUI credential maintenance |

Use the [preferences data source](docs/data-sources/preferences.md) to read
supported nonsecret settings. See [settings by tab](docs/preferences-by-tab.md)
for coverage and [examples](examples) for complete configurations.

## Ownership and sensitive values

Only configured preferences are managed. Omitted or `null` attributes are left
unchanged; `false`, `0`, and empty strings are explicit values. Removing an
attribute relinquishes ownership. Destroying a preferences resource removes its
state without resetting the server. Use one owner per server setting.

Watched folders use whole-map ownership, and RSS rules are managed individually.
Category deletion preserves torrents and files, but automatic torrent management
may relocate content. Paths always refer to the server's filesystem.

Use ephemeral sensitive inputs for write-only passwords. Custom commands and
HTTP headers may contain secrets and remain in sensitive resource state and
machine-readable plans; protect your state accordingly. The general preferences
data source excludes them.

Run WebUI endpoint and authentication changes as separate targeted operations,
then update the provider connection. Read the [maintenance guide](docs/webui-maintenance.md)
before changing connection settings.

## Contributing

See [development and testing](docs/development.md),
[disposable acceptance tests](tests/acceptance/README.md), and the
[OpenAPI specification](api/qbittorrent.yaml).

## License

[Mozilla Public License 2.0](LICENSE).
