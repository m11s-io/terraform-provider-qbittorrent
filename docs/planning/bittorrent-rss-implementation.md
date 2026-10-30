# BitTorrent and RSS preferences

The ordinary `qbittorrent_preferences` resource owns these flat settings
partially: configure only the attributes Terraform should manage. Unset values
remain untouched, `false`, zero, empty strings, and supported `-1` sentinels
are explicit writes, and removing an attribute relinquishes ownership without
resetting the server.

## BitTorrent queue and privacy settings

`encryption` accepts `0` (allow), `1` (require), or `2` (disable). The
`max_active_checking_torrents`, `max_active_downloads`, `max_active_uploads`,
and `max_active_torrents` values accept `-1` for unlimited or nonnegative
integers. Slow download/upload thresholds use KiB/s, and the slow inactivity
timer uses seconds. Those three UI inputs require at least 1. The
`dont_count_slow_torrents` boolean follows the checkbox wording directly.

## Share limits and tracker additions

`max_ratio` is the canonical share-ratio input and supports fractional values.
`-1` disables the ratio limit; zero is an enabled limit. Total and inactive
seeding limits use minutes, accept zero, and use `-1` to disable. The three
`*_enabled` flags are computed data-source outputs derived from their numeric
settings. They are not resource inputs and never appear in patches.

`share_limits_mode` accepts `MatchAny` or `MatchAll`. `max_ratio_act` describes
what qBittorrent does when a torrent reaches a configured share limit:

| Value | Consequence |
| --- | --- |
| `0` | Stop the torrent. |
| `1` | Remove the torrent from qBittorrent; leave its downloaded files. |
| `2` | Enable super seeding for the torrent. |
| `3` | Remove the torrent and delete its downloaded files. |

Values `1` and `3` authorize future removal when a torrent reaches its limit;
`3` also authorizes deleting content. The default configured action in examples
is `0`. Applying preferences does not immediately remove a torrent.

`add_trackers` is raw newline-delimited text for trackers appended to new
torrents. `add_trackers_url` configures the fetch URL, and
`add_trackers_url_list` is only the fetched, read-only data-source output. The
provider does not fetch trackers itself or patch the fetched list.

## RSS preferences and rule ownership

RSS refresh interval is in minutes; same-host request delay is in seconds.
The upstream refresh and delay setters store supplied values as-is. The
maximum-articles setter clamps negative values to zero, so the provider accepts
zero and positive values. Smart episode filters remain newline-delimited text,
including intentional blank lines. Keep `rss_processing_enabled` and
`rss_auto_downloading_enabled` false in offline acceptance and examples so no
feeds are fetched and no automatic downloads are started.

These preferences only toggle RSS processing and define the smart episode
filter. The `qbittorrent_rss_rule` resource owns one named auto-download rule
at a time; it does not own this preference resource's RSS controls or replace
the complete rule collection. Feed/folder management and live feed fetching
are separate operations.

## Pinned source behavior

The wire types, sentinels, queue units, action mapping, and RSS setter behavior
were checked against qBittorrent 5.3.0 commit
`9213075dd5f7b352208361c6a84c1c115efc898e`. Ratio values below zero are
normalized upstream to `-1`; seeding minutes below `-1` are normalized to
`-1`. RSS refresh uses a signed 32-bit integer, fetch delay a signed 64-bit
integer, and article count a signed 32-bit integer with a zero lower bound.
