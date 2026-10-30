# Downloads preference implementation

The `qbittorrent_preferences` resource now owns the scalar Downloads settings
from the qBittorrent 5.3.0 preferences API. Existing `save_path`, `temp_path`,
`temp_path_enabled`, and `auto_tmm_enabled` remain registered in the legacy
field group; this implementation adds the other scalar preferences. The
`scan_dirs` map remains owned by the dedicated watched-folders resource, and
`mail_notification_password` remains in the credentials resource boundary.

The additions include torrent-add behavior and enums, torrent management and
backup settings, newline-delimited excluded-file patterns, SMTP notification
settings except the password, and added/finished external-program templates.
False, zero, and empty strings are explicit updates. Null values are omitted
from a patch, which relinquishes ownership while retaining the server value.
Paths are server paths and are not checked against the provider host's local
filesystem. Excluded patterns remain a raw newline-delimited string.

The API trims surrounding whitespace from both external-program templates when
it stores them. The resource marks these two inputs sensitive and the general
preferences data source omits them. Read-after-write reports the server's
trimmed value as a normalization diagnostic. Configuring a template does not
run it; qBittorrent runs it only when the corresponding enable flag is active
and a matching torrent event occurs. Likewise, ordinary preference apply does
not send a test email; `mail_notification_enabled` only configures future
completion notifications.

The integration fixture configures both notification and external-program
enable flags as false. It stores nonsecret SMTP settings and disabled command
templates on a disposable qBittorrent instance without sending email or
executing hooks.

## Upstream references

The mapping uses qBittorrent `release-5.3.0`, commit
`9213075dd5f7b352208361c6a84c1c115efc898e`:

- `src/webui/api/appcontroller.cpp:175-240` reads the preference keys.
- `src/webui/api/appcontroller.cpp:600-734` applies keys independently and
  trims the two external-program templates before storing them.
- `src/webui/www/private/views/preferences.html:169-472` defines the Downloads
  controls; lines 3007-3061 translate controls into API keys.
- `src/base/bittorrent/torrentcontentlayout.h`,
  `src/base/bittorrent/torrent.h`, and `src/base/net/smtpencryptiontype.h`
  define the string enum choices.

`scan_dirs` and the write-only SMTP password are intentionally not described
by this scalar group. The test-email button is an action, not persistent
configuration.
