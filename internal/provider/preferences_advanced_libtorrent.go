package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var advancedLibtorrentPreferenceFields = []preferenceField{
	{name: "bdecode_depth_limit", wireKey: "bdecode_depth_limit", kind: preferenceInt, tab: "Advanced", description: "Libtorrent bdecode maximum nesting depth; sent as a signed 32-bit integer.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "bdecode_token_limit", wireKey: "bdecode_token_limit", kind: preferenceInt, tab: "Advanced", description: "Libtorrent bdecode maximum token count; sent as a signed 32-bit integer.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "async_io_threads", wireKey: "async_io_threads", kind: preferenceInt, tab: "Advanced", description: "Asynchronous I/O thread count; the native getter clamps to 1..1024.", integerMin: int64ptr(1), integerMax: int64ptr(1024)},
	{name: "hashing_threads", wireKey: "hashing_threads", kind: preferenceInt, tab: "Advanced", description: "Hashing thread count; the native getter clamps to 1..1024 and runtime use is conditional on QBT_USES_LIBTORRENT2.", integerMin: int64ptr(1), integerMax: int64ptr(1024), capability: "libtorrent.hashing_threads"},
	{name: "file_pool_size", wireKey: "file_pool_size", kind: preferenceInt, tab: "Advanced", description: "Maximum open files retained by libtorrent; sent as a signed 32-bit integer.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "checking_memory_use", wireKey: "checking_memory_use", kind: preferenceInt, tab: "Advanced", description: "Memory limit while checking torrents in MiB; native setter clamps the minimum to 1.", integerMin: int64ptr(1), integerMax: int64ptr(2147483647)},
	{name: "disk_cache", wireKey: "disk_cache", kind: preferenceInt, tab: "Advanced", description: "Disk cache size in MiB; native setter retains negative values and caps at 33554431 on 64-bit builds (1536 on 32-bit builds). Runtime cache application is compiled only without QBT_USES_LIBTORRENT2.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(33554431)},
	{name: "disk_cache_ttl", wireKey: "disk_cache_ttl", kind: preferenceInt, tab: "Advanced", description: "Disk cache expiry in seconds; runtime application is compiled only without QBT_USES_LIBTORRENT2.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "disk_queue_size", wireKey: "disk_queue_size", kind: preferenceInt, tab: "Advanced", description: "Disk queue threshold in bytes on the wire (the UI displays KiB); accepts signed 64-bit values.", integerMin: int64ptr(-9223372036854775808), integerMax: int64ptr(9223372036854775807)},
	{name: "disk_io_type", wireKey: "disk_io_type", kind: preferenceInt, tab: "Advanced", description: "Disk I/O implementation; restart required. 0 default, 1 memory-mapped, 2 POSIX, 3 simple pread/pwrite. Upstream 4 pread/pwrite is compiled only when LIBTORRENT_VERSION_NUM >= 20100; the provider accepts the common 0..3 subset.", integerMin: int64ptr(0), integerMax: int64ptr(3), integerEnums: []int64{0, 1, 2, 3}},
	{name: "disk_io_read_mode", wireKey: "disk_io_read_mode", kind: preferenceInt, tab: "Advanced", description: "OS read cache: 0 disabled, 1 enabled.", integerMin: int64ptr(0), integerMax: int64ptr(1), integerEnums: []int64{0, 1}},
	{name: "disk_io_write_mode", wireKey: "disk_io_write_mode", kind: preferenceInt, tab: "Advanced", description: "OS write cache: 0 disabled, 1 enabled. Upstream 2 write-through is compiled only with QBT_USES_LIBTORRENT2; the provider accepts the common 0..1 subset.", integerMin: int64ptr(0), integerMax: int64ptr(1), integerEnums: []int64{0, 1}},
	{name: "enable_coalesce_read_write", wireKey: "enable_coalesce_read_write", kind: preferenceBool, tab: "Advanced", description: "Enable coalescing disk reads and writes; runtime application is compiled only without QBT_USES_LIBTORRENT2.", capability: "libtorrent.coalesce_read_write"},
	{name: "enable_piece_extent_affinity", wireKey: "enable_piece_extent_affinity", kind: preferenceBool, tab: "Advanced", description: "Prefer allocating pieces near each other on disk."},
	{name: "enable_upload_suggestions", wireKey: "enable_upload_suggestions", kind: preferenceBool, tab: "Advanced", description: "Allow upload piece suggestions."},
	{name: "send_buffer_watermark", wireKey: "send_buffer_watermark", kind: preferenceInt, tab: "Advanced", description: "Send buffer watermark in KiB (wire units).", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "send_buffer_low_watermark", wireKey: "send_buffer_low_watermark", kind: preferenceInt, tab: "Advanced", description: "Low send buffer watermark in KiB (wire units).", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "send_buffer_watermark_factor", wireKey: "send_buffer_watermark_factor", kind: preferenceInt, tab: "Advanced", description: "Send buffer watermark growth factor in percent.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "connection_speed", wireKey: "connection_speed", kind: preferenceInt, tab: "Advanced", description: "Outgoing connection attempts per second.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "seeding_outgoing_connections", wireKey: "seeding_outgoing_connections", kind: preferenceBool, tab: "Advanced", description: "Allow outgoing connections while seeding."},
	{name: "socket_send_buffer_size", wireKey: "socket_send_buffer_size", kind: preferenceInt, tab: "Advanced", description: "Socket send buffer in bytes on the wire (the UI displays KiB); 0 selects the system default.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "socket_receive_buffer_size", wireKey: "socket_receive_buffer_size", kind: preferenceInt, tab: "Advanced", description: "Socket receive buffer in bytes on the wire (the UI displays KiB); 0 selects the system default.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "socket_backlog_size", wireKey: "socket_backlog_size", kind: preferenceInt, tab: "Advanced", description: "Listen socket backlog size.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "outgoing_ports_min", wireKey: "outgoing_ports_min", kind: preferenceInt, tab: "Advanced", description: "Minimum outgoing port; 0 disables port restriction. Configure with outgoing_ports_max when managing a range.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "outgoing_ports_max", wireKey: "outgoing_ports_max", kind: preferenceInt, tab: "Advanced", description: "Maximum outgoing port; 0 disables port restriction. Configure with outgoing_ports_min when managing a range.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "upnp_lease_duration", wireKey: "upnp_lease_duration", kind: preferenceInt, tab: "Advanced", description: "UPnP lease duration in seconds; 0 requests a permanent lease.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "peer_tos", wireKey: "peer_tos", kind: preferenceInt, tab: "Advanced", description: "Peer connection DSCP/TOS value.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "utp_tcp_mixed_mode", wireKey: "utp_tcp_mixed_mode", kind: preferenceInt, tab: "Advanced", description: "Mixed uTP/TCP mode: 0 prefer TCP, 1 peer proportional.", integerMin: int64ptr(0), integerMax: int64ptr(1), integerEnums: []int64{0, 1}},
	{name: "hostname_cache_ttl", wireKey: "hostname_cache_ttl", kind: preferenceInt, tab: "Advanced", description: "Hostname resolver cache time to live in seconds; 0 disables the cache lifetime.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "idn_support_enabled", wireKey: "idn_support_enabled", kind: preferenceBool, tab: "Advanced", description: "Resolve internationalized domain names."},
	{name: "enable_multi_connections_from_same_ip", wireKey: "enable_multi_connections_from_same_ip", kind: preferenceBool, tab: "Advanced", description: "Allow multiple peer connections from one IP address."},
	{name: "enable_multi_connections_from_same_peer_id", wireKey: "enable_multi_connections_from_same_peer_id", kind: preferenceBool, tab: "Advanced", description: "Allow multiple peer connections with the same peer ID; runtime application requires LIBTORRENT_VERSION_NUM >= 20013.", capability: "libtorrent.multi_connections_same_peer_id"},
	{name: "validate_https_tracker_certificate", wireKey: "validate_https_tracker_certificate", kind: preferenceBool, tab: "Advanced", description: "Validate HTTPS tracker certificates; keep enabled unless explicitly changing the server's security policy."},
	{name: "ssrf_mitigation", wireKey: "ssrf_mitigation", kind: preferenceBool, tab: "Advanced", description: "Mitigate server-side request forgery in tracker requests; keep enabled unless explicitly changing the server's security policy."},
	{name: "block_peers_on_privileged_ports", wireKey: "block_peers_on_privileged_ports", kind: preferenceBool, tab: "Advanced", description: "Block peer connections to privileged destination ports."},
	{name: "upload_slots_behavior", wireKey: "upload_slots_behavior", kind: preferenceInt, tab: "Advanced", description: "Upload slot behavior: 0 fixed slots, 1 upload-rate-based.", integerMin: int64ptr(0), integerMax: int64ptr(1), integerEnums: []int64{0, 1}},
	{name: "upload_choking_algorithm", wireKey: "upload_choking_algorithm", kind: preferenceInt, tab: "Advanced", description: "Seed choking algorithm: 0 round-robin, 1 fastest upload, 2 anti-leech.", integerMin: int64ptr(0), integerMax: int64ptr(2), integerEnums: []int64{0, 1, 2}},
	{name: "announce_to_all_trackers", wireKey: "announce_to_all_trackers", kind: preferenceBool, tab: "Advanced", description: "Announce to all trackers in each tier."},
	{name: "announce_to_all_tiers", wireKey: "announce_to_all_tiers", kind: preferenceBool, tab: "Advanced", description: "Announce to all tracker tiers."},
	{name: "announce_ip", wireKey: "announce_ip", kind: preferenceString, tab: "Advanced", description: "IP address announced to trackers; empty uses the default. Changes require restart."},
	{name: "announce_port", wireKey: "announce_port", kind: preferenceInt, tab: "Advanced", description: "Port announced to trackers; 0 uses the listening port. Changes require restart; native setter stores signed 32-bit values as supplied.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "max_concurrent_http_announces", wireKey: "max_concurrent_http_announces", kind: preferenceInt, tab: "Advanced", description: "Maximum concurrent HTTP tracker announces.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "stop_tracker_timeout", wireKey: "stop_tracker_timeout", kind: preferenceInt, tab: "Advanced", description: "Stop tracker announce timeout in seconds; 0 disables the timeout.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "peer_turnover", wireKey: "peer_turnover", kind: preferenceInt, tab: "Advanced", description: "Peer turnover disconnect percentage; native setter stores signed 32-bit values as supplied.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "peer_turnover_cutoff", wireKey: "peer_turnover_cutoff", kind: preferenceInt, tab: "Advanced", description: "Peer turnover threshold percentage; native setter stores signed 32-bit values as supplied.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "peer_turnover_interval", wireKey: "peer_turnover_interval", kind: preferenceInt, tab: "Advanced", description: "Peer turnover disconnect interval in seconds; native setter stores signed 32-bit values as supplied.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "request_queue_size", wireKey: "request_queue_size", kind: preferenceInt, tab: "Advanced", description: "Maximum outstanding requests to one peer.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "max_outstanding_block_requests", wireKey: "max_outstanding_block_requests", kind: preferenceInt, tab: "Advanced", description: "Maximum outstanding block requests from one peer.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "dht_bootstrap_nodes", wireKey: "dht_bootstrap_nodes", kind: preferenceString, tab: "Advanced", description: "DHT bootstrap node list; empty input resets to the native default, and the getter reads back the canonical default list."},
	{name: "webtorrent_stun_server", wireKey: "webtorrent_stun_server", kind: preferenceString, tab: "Advanced", description: "WebTorrent NAT traversal STUN server; runtime application requires LIBTORRENT_VERSION_NUM >= 20100.", capability: "libtorrent.webtorrent_stun"},
	{name: "i2p_inbound_quantity", wireKey: "i2p_inbound_quantity", kind: preferenceInt, tab: "Advanced", description: "I2P inbound tunnel quantity, 1..16; runtime use requires QBT_USES_LIBTORRENT2 && TORRENT_USE_I2P.", integerMin: int64ptr(1), integerMax: int64ptr(16), capability: "libtorrent.i2p"},
	{name: "i2p_outbound_quantity", wireKey: "i2p_outbound_quantity", kind: preferenceInt, tab: "Advanced", description: "I2P outbound tunnel quantity, 1..16; runtime use requires QBT_USES_LIBTORRENT2 && TORRENT_USE_I2P.", integerMin: int64ptr(1), integerMax: int64ptr(16), capability: "libtorrent.i2p"},
	{name: "i2p_inbound_length", wireKey: "i2p_inbound_length", kind: preferenceInt, tab: "Advanced", description: "I2P inbound tunnel length, 0..7; runtime use requires QBT_USES_LIBTORRENT2 && TORRENT_USE_I2P.", integerMin: int64ptr(0), integerMax: int64ptr(7), capability: "libtorrent.i2p"},
	{name: "i2p_outbound_length", wireKey: "i2p_outbound_length", kind: preferenceInt, tab: "Advanced", description: "I2P outbound tunnel length, 0..7; runtime use requires QBT_USES_LIBTORRENT2 && TORRENT_USE_I2P.", integerMin: int64ptr(0), integerMax: int64ptr(7), capability: "libtorrent.i2p"},
	{name: "i2p_inbound_length_variance", wireKey: "i2p_inbound_length_variance", kind: preferenceInt, tab: "Advanced", description: "I2P inbound tunnel length variance, -7..7; runtime use requires QBT_USES_LIBTORRENT2 && TORRENT_USE_I2P and LIBTORRENT_VERSION_NUM >= 20012.", integerMin: int64ptr(-7), integerMax: int64ptr(7), capability: "libtorrent.i2p"},
	{name: "i2p_outbound_length_variance", wireKey: "i2p_outbound_length_variance", kind: preferenceInt, tab: "Advanced", description: "I2P outbound tunnel length variance, -7..7; runtime use requires QBT_USES_LIBTORRENT2 && TORRENT_USE_I2P and LIBTORRENT_VERSION_NUM >= 20012.", integerMin: int64ptr(-7), integerMax: int64ptr(7), capability: "libtorrent.i2p"},
}

// validateAdvancedLibtorrentOutgoingPorts checks the range only when both
// configured values are known. Terraform can supply an unknown port until
// apply, in which case the comparison is deferred to apply-time validation.
func validateAdvancedLibtorrentOutgoingPorts(values map[string]attr.Value, d *diag.Diagnostics) {
	minValue, minOK := values["outgoing_ports_min"].(types.Int64)
	maxValue, maxOK := values["outgoing_ports_max"].(types.Int64)
	if !minOK || !maxOK || minValue.IsNull() || maxValue.IsNull() || minValue.IsUnknown() || maxValue.IsUnknown() {
		return
	}
	min, max := minValue.ValueInt64(), maxValue.ValueInt64()
	if min != 0 && max != 0 && min > max {
		d.AddAttributeError(path.Root("outgoing_ports_max"), "Invalid outgoing port range", "outgoing_ports_max must be greater than or equal to outgoing_ports_min when both are non-zero.")
	}
}
