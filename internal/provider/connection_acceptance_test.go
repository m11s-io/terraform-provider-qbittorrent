package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccConnectionPreferences(t *testing.T) {
	setupAccEnv(t, false)
	config := accProvider() + `
resource "qbittorrent_preferences" "connection" {
  bittorrent_protocol = 1
  max_connec = -1
  max_connec_per_torrent = 128
  max_uploads = -1
  max_uploads_per_torrent = 8
  i2p_enabled = false
  i2p_address = "127.0.0.1"
  i2p_port = 7654
  i2p_mixed_mode = false
  i2p_pex_enabled = false
  proxy_type = "None"
  proxy_ip = "127.0.0.1"
  proxy_port = 9050
  proxy_auth_enabled = false
  proxy_username = "unused"
  proxy_hostname_lookup = false
  proxy_bittorrent = false
  proxy_peer_connections = false
  proxy_rss = false
  proxy_misc = false
  ip_filter_enabled = false
  ip_filter_trackers = false
  banned_ips = ""
  upnp = false
}
data "qbittorrent_preferences" "connection" {
  depends_on = [qbittorrent_preferences.connection]
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		Steps: []resource.TestStep{
			{Config: config, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("qbittorrent_preferences.connection", "max_connec", "-1"),
				resource.TestCheckResourceAttr("data.qbittorrent_preferences.connection", "bittorrent_protocol", "1"),
				resource.TestCheckResourceAttr("data.qbittorrent_preferences.connection", "proxy_auth_enabled", "false"),
				resource.TestCheckResourceAttr("data.qbittorrent_preferences.connection", "proxy_ip", "127.0.0.1"),
				resource.TestCheckResourceAttr("data.qbittorrent_preferences.connection", "banned_ips", ""),
			)},
			{Config: config, PlanOnly: true},
		},
	})
}
