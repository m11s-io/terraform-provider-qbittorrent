# Settings coverage

The [coverage reference](../api/options-coverage.json) maps qBittorrent Options
save keys to resource ownership and tests. It is based on the upstream commit
recorded in the file. The generated [preferences resource](resources/preferences.md)
and [data source](data-sources/preferences.md) document the Terraform interface.

Some settings depend on the server platform or linked libtorrent build. Reading
back a stored setting does not establish that it is active at runtime. See
[Advanced libtorrent settings](advanced-libtorrent.md) for supported enum subsets,
compile-time conditions, and restart requirements.

## Action boundaries

The WebUI's random-port button selects a value for `listen_port`; the provider
manages the selected value, without rerandomizing it during apply. Sending a test
email, rotating or deleting an API key, copying a key, and opening DynDNS account
registration are explicit actions, not persistent preferences. They are not
performed by ordinary Terraform applies. API-key values are excluded from
public read models and Terraform state.

## Separate resource ownership

- [Watched folders](watch-folders.md) own the complete `scan_dirs` collection.
- [Service passwords](credentials.md) use write-only inputs and version counters.
- [WebUI maintenance](webui-maintenance.md) owns endpoint and authentication transitions.
- [RSS rules](rss-rule-resource.md) own individual rule definitions and preserve match history.
