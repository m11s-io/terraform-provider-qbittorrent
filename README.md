# Terraform / OpenTofu provider for qBittorrent

Manage selected application preferences and download categories on **qBittorrent
5.3.0**, Web API **2.16.2**, using the Terraform Plugin Framework. Other versions
are rejected. This provider does not bootstrap credentials or manage torrents.

```hcl
terraform {
  required_providers {
    qbittorrent = {
      source = "m11s-io/qbittorrent"
    }
  }
}

provider "qbittorrent" {
  url = "https://qbittorrent.example.com"
}

resource "qbittorrent_preferences" "application" {
  dht      = false
  up_limit = 1048576 # bytes/s; multiples of 1024, 0 is unlimited
}

resource "qbittorrent_category" "movies" {
  name      = "movies"
  save_path = "/downloads/movies"
}
```

Set `QBITTORRENT_API_KEY` for Bearer authentication, or both
`QBITTORRENT_USERNAME` and `QBITTORRENT_PASSWORD` for cookie sessions. Do not mix
auth methods, including environment variables. `QBITTORRENT_URL` can supply the
URL. Explicit configuration takes precedence over the corresponding environment
variable, including explicit empty strings. Sensitive provider inputs are marked
sensitive; prefer environment variables to avoid writing credentials into HCL.
Cookies are isolated per provider instance and never stored in Terraform state.
TLS verification, server Host validation and CSRF protection remain enabled.

Only configured preferences are managed. Unset fields, including imported
settings, remain unowned; false and zero are explicit values. Removing an
attribute relinquishes ownership and does not reset it. Destroying preferences
only removes Terraform state. Use one preferences resource per server; overlapping
ownership across states is unsupported. The read-only data source exposes only
the supported nonsecret fields, never API keys, password hashes or cookies.

Category names are resource IDs; renaming requires replacement. Nested category
ancestors must already exist. Reference the parent resource name in the child name
to order creation and leaf-first deletion. Empty save paths append the category
basename beneath the parent effective path; nonempty relative paths resolve
beneath the global default save path. Destroy reassigns
torrents to the parent category (empty for top-level names). It preserves torrents
and files, although automatic torrent management may relocate content. Upstream
also removes subcategories, so the provider refuses deletion while descendants
exist. Delete descendants first; avoid concurrent hierarchy changes.

Paths refer to the **server** filesystem. Use normalized paths. If the server
normalizes or rejects a configured field, apply reports the affected attribute;
correct configuration to match the server. HTTP operations have a 15-second
timeout. Reads may reauthenticate once on an expired session; writes are never
replayed automatically. Rerun apply after an expired-session write failure.

```sh
terraform import qbittorrent_preferences.application application
terraform import qbittorrent_category.movies movies
```

See [generated provider/resource docs](docs/index.md), [development contract](docs/development.md),
and [disposable acceptance tests](tests/acceptance/README.md). Build with Go 1.27.3:
`make test race vet build docs`.

## Release status

Source and release scaffolding do not imply Registry publication. Registry signup,
GPG public-key enrollment, and the first release require an authorized human
account/action. infra/github owns this repository's LICENSE and signing secrets.
Do not regenerate LICENSE through provider automation. No release is published
by development/acceptance workflows.
