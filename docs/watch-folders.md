---
page_title: "qbittorrent_watch_folders Resource - qbittorrent"
subcategory: ""
description: |-
  Authoritative ownership of the complete qBittorrent watched-folder map.
---

# qbittorrent_watch_folders (Resource)

Use this singleton to own qBittorrent's complete watched-folder map. `folders`
is authoritative: adding, updating, or removing entries replaces the server's
whole map. Configure `folders = {}` to clear all watches. Omitting `folders`
relinquishes ownership without changing the server map. Destroy removes only
Terraform state and preserves watches. Import ID is `application`; import reads
and adopts the full current map.

Watched-folder keys and custom save paths are paths on the qBittorrent server.
Modes are `default` (default save path), `watch_folder` (save into the watched
directory), and `custom` (use `save_path`). `save_path` is required only for
`custom`. qBittorrent can silently ignore invalid paths, so the provider
verifies the complete normalized map after writing and reports any mismatch.
Use empty disposable directories when testing this setting: a watched
directory may automatically import torrent files placed in it.

The legacy API does not clear a watched folder's previous save path when its
value changes to integer `1` (`default`). If an existing entry is in
`watch_folder` or `custom` mode, Terraform reports this transition before
sending a write. Remove that entry from the complete `folders` map in one
apply (or use `folders = {}` to clear the collection), then add it again with
`save_mode = "default"` in a later apply. The provider does not perform these
two writes implicitly.

## Example Usage

```terraform
resource "qbittorrent_watch_folders" "application" {
  folders = {
    "/downloads/watch-default" = {
      save_mode = "default"
    }
    "/downloads/watch-here" = {
      save_mode = "watch_folder"
    }
    "/downloads/watch-custom" = {
      save_mode = "custom"
      save_path = "/downloads/complete"
    }
  }
}
```

## Import

```shell
terraform import qbittorrent_watch_folders.application application
```
