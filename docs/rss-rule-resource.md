# RSS auto-download rule resource

`qbittorrent_rss_rule` manages one rule from qBittorrent's RSS auto-downloader
by its exact rule name. Import an existing rule with that name. A configured
name change replaces the old rule with a new one; creation never adopts or
overwrites an existing rule.

The resource owns only the rule definition. It does not create or delete RSS
feeds, refresh feeds, download articles, manage another rule, or delete torrent
content. `affected_feeds` stores URLs already known to qBittorrent and does not
create those feeds. Match history (`last_match` and
`previously_matched_episodes`) is computed and preserved when the rule changes.

The add-stopped and content-layout fields are nullable to preserve qBittorrent's
optional values. `torrent_params` exposes a typed subset of native
`AddTorrentParams`. TLS certificate/private-key material and file-selection
internals are not supported. The resource submits the exact upstream form
fields: `ruleName` and JSON-string `ruleDef` for create/update, and `ruleName`
for removal.

For a configuration-only rollout, keep `rss_processing_enabled` and
`rss_auto_downloading_enabled` false in `qbittorrent_preferences`, and leave
`affected_feeds` empty. Rule CRUD does not itself fetch a feed or add a torrent.

```hcl
resource "qbittorrent_rss_rule" "example" {
  rule_name      = "Example release rule"
  enabled        = false
  priority       = 0
  use_regex      = true
  must_contain   = "Example"
  must_not_contain = "sample"
  episode_filter = "S[0-9]+E[0-9]+"
  affected_feeds = []
  ignore_days    = 0
  smart_filter   = false

  torrent_params = {
    category     = "tv"
    save_path    = "/srv/qbittorrent/downloads"
    stopped      = true
    content_layout = "Subfolder"
    download_limit = 0
  }
}
```
