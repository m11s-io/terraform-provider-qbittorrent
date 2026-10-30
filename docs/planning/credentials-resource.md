# `qbittorrent_credentials`

`qbittorrent_credentials` manages qBittorrent's SMTP, proxy, and DynDNS passwords without reading them back. Import ID: `application`. Destroy removes Terraform state only and does not clear any remote password.

Set each password together with its corresponding version counter. Increment only that counter to rotate that password. The resource sends a password on create or when its counter increases, and does not resend it for unrelated changes or an unchanged counter. The counter is retained in Terraform state; the password is a sensitive write-only input and is not stored in state, plan values, or refresh results.

Set both attributes for a password to `null` to relinquish management without changing the server value. To clear a stored password, set its write-only input to an empty string and increment its version. This works only where the qBittorrent setter accepts an empty value. The resource does not claim to verify a password because qBittorrent does not expose one-way-safe readback semantics for this contract.

Use ephemeral sensitive variables or another ephemeral input source for password values. Do not place literal passwords in configuration examples or version control. Ordinary qBittorrent preference reads can include SMTP, proxy, and DynDNS passwords and the WebUI API key; this provider's generated read model discards them. Custom HTTP headers are also excluded from the generic preferences data source because header values can contain secrets.

The DynDNS “Register” and email test controls are external actions and are not run during apply. This resource never sends email, registers a domain, rotates API keys, or performs account setup.

The managed WebUI authentication-failure count, ban duration, session timeout, and session-count limit use the native signed 32-bit range (`-2147483648..2147483647`). qBittorrent's setters store these values without a narrower lower bound; nonpositive authentication-failure counts disable the counter and nonpositive session-count limits mean unlimited sessions.

## Example

```terraform
variable "smtp_password" {
  type      = string
  sensitive = true
  ephemeral = true
}

resource "qbittorrent_credentials" "application" {
  smtp_password_wo         = var.smtp_password
  smtp_password_wo_version = 1
}
```

## Attributes

| Name | Description |
| --- | --- |
| `smtp_password_wo` | Write-only SMTP password. Empty string requests clearing it. |
| `smtp_password_wo_version` | Nonnegative version; increase to send the SMTP password. |
| `proxy_password_wo` | Write-only proxy password. Empty string requests clearing it. |
| `proxy_password_wo_version` | Nonnegative version; increase to send the proxy password. |
| `dyndns_password_wo` | Write-only DynDNS password. Empty string requests clearing it. |
| `dyndns_password_wo_version` | Nonnegative version; increase to send the DynDNS password. |

