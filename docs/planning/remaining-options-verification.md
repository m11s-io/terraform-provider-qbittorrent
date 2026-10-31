# Remaining Options verification

Implementation follows `docs/superpowers/plans/2026-10-30-remaining-options.md`
and its design, starting at planning commit `f5de2e7` on provider baseline
`a73a3a1` / 0.3.0. Upstream evidence uses qBittorrent commit
`9213075dd5f7b352208361c6a84c1c115efc898e` exclusively.

## Coverage and ownership

The reviewed `2026-10-30-options-save-keys.json` matrix assigns every indexed
save key and additional inventoried control an owner, disposition, reason and
concrete evidence reference. `TestOptionsDispositionMatrixHasVerifiedOwnership`
checks those assignments against the actual registry and verifies evidence
paths. The index is not a claim that every control has runtime effect on every
linked-library/platform build.

Ordinary preferences retain partial ownership and native wire units. Specialized
resources cover the watched-folder map, write-only service passwords, explicit
WebUI maintenance, and individual RSS rules. Passwords/API keys are discarded
from public read models. Write-only values are read from Config, versioned, and
covered by ephemeral plan/state canary checks. Sensitive configured command
text/custom headers may remain in sensitive resource state and machine-readable
plans; these values are excluded from the general data source.

## Disposable verification boundary

The pinned image is
`ghcr.io/linuxserver/qbittorrent@sha256:75ecbb1c89ab7b0a21c4bde40b4820e4f1649d2d56d83b91913422155e8d2a5f`.
It reports qBittorrent 5.3.0 / API 2.16.2; Linux, bitness 64,
libtorrent 2.1.2.0, Boost 1.92.0, Qt 6.12.0, OpenSSL 4.1.0, zlib 1.3.2.
Only loopback WebUI ports are published. Discovery/forwarding are disabled;
ordinary hooks, SMTP/proxy/DynDNS, trackers and RSS downloading are not enabled.
Restart tests use only disposable containers and restore their original values.
Torrent hashes and RSS feed inventory are compared around collection/rule tests.

Listener and authentication transitions use a separate stateful local API
fixture under both Terraform 1.15.9 and OpenTofu 1.13.2. It closes the old listener,
loses the mutation response, delays target verification, checks retained state,
reconnects using the new provider URL/credentials, and counts writes across
refresh, follow-up apply and no-op plans. This proves provider/CLI recovery
behavior, not every native deployment's proxy, certificate or listener setup.
No infrastructure, release tags or live application configuration was changed.

## Runtime limitations

- See `../advanced-libtorrent.md` for exact compile predicates. Key presence
  proves API/readback support, not runtime application. The pinned build does
  not expose `TORRENT_USE_I2P` through buildInfo, so that flag is not claimed tested.
- Common enum subsets deliberately omit disk-I/O value 4 and write-through
  value 2 until their build capabilities can be established safely.
- Disk-cache values above 1536 MiB can normalize on 32-bit servers despite the
  universal 64-bit cap of 33554431 MiB; this build is 64-bit.
- The legacy watched-folder setter cannot reset an existing custom/watch-folder
  destination with integer 1. The resource rejects this transition before
  mutation; explicitly remove/clear the entry in one apply and re-add default
  mode in a second apply. Destroy itself preserves watches.
- WebUI maintenance is a staged targeted operation. Update provider connection
  inputs after it; no Kubernetes Service/port publication or TLS/auth bypass is
  performed automatically. Write-only passwords have no remote drift detection.

## Implementation decisions

1. Added finite float sentinels so ratios support exactly -1 or nonnegative
   values; otherwise valid disabled values would be rejected or negative
   fractions could drift.
2. Allowed bounded native integer/enum transport models to use int/int32 while
   retaining int64 for large/unbounded counters. A wrong width assertion could
   miss a transport bug; literal large-byte fixtures also cover that boundary.
3. Assigned omitted `web_ui_upnp` coverage to ordinary WebUI settings, since it
   is outside the endpoint resource's ownership list. A different ownership
   choice would require a migration; only explicit false is exercised.
4. Diagnosed unsupported legacy default-folder transitions before writing,
   rather than performing an implicit multiwrite reset. This costs an explicit
   staged user transition and avoids uncertain intermediate resets.

## Final checks

Final commands, commit range, independent review and any remaining findings are
recorded below after the last maintenance security regression is integrated.
