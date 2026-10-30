resource "qbittorrent_preferences" "advanced_libtorrent_coverage" {
  bdecode_depth_limit = 100
  bdecode_token_limit = 1000000
  async_io_threads    = 4
  hashing_threads     = 2
  file_pool_size      = 40
  checking_memory_use = 64
  disk_cache          = 64
  disk_cache_ttl      = 60
  # The UI value 102400 KiB is represented directly as 104857600 bytes.
  disk_queue_size = 104857600
  # Version-conditional disk I/O enum values are omitted until linked build info is checked.
  disk_io_type                 = 0
  disk_io_read_mode            = 1
  disk_io_write_mode           = 1
  enable_coalesce_read_write   = false
  enable_piece_extent_affinity = false
  enable_upload_suggestions    = false
  # Watermarks are KiB, while socket buffer fields below are bytes.
  send_buffer_watermark        = 512
  send_buffer_low_watermark    = 128
  send_buffer_watermark_factor = 150
  connection_speed             = 20
  seeding_outgoing_connections = false
  socket_send_buffer_size      = 1048576
  socket_receive_buffer_size   = 1048576
  socket_backlog_size          = 30
  # Zero disables the outgoing port restriction.
  outgoing_ports_min                         = 0
  outgoing_ports_max                         = 0
  upnp_lease_duration                        = 0
  peer_tos                                   = 0
  utp_tcp_mixed_mode                         = 0
  hostname_cache_ttl                         = 0
  idn_support_enabled                        = true
  enable_multi_connections_from_same_ip      = false
  enable_multi_connections_from_same_peer_id = false
  # Keep security controls enabled unless explicitly changing server policy.
  validate_https_tracker_certificate = true
  ssrf_mitigation                    = true
  block_peers_on_privileged_ports    = false
  upload_slots_behavior              = 0
  upload_choking_algorithm           = 0
  announce_to_all_trackers           = false
  announce_to_all_tiers              = false
  announce_ip                        = ""
  announce_port                      = 0
  max_concurrent_http_announces      = 50
  stop_tracker_timeout               = 0
  peer_turnover                      = 4
  peer_turnover_cutoff               = 90
  peer_turnover_interval             = 300
  request_queue_size                 = 2000
  max_outstanding_block_requests     = 500
  # Use the getter's canonical value; empty resets to this default and reads back normalized.
  dht_bootstrap_nodes          = "dht.libtorrent.org:25401, dht.transmissionbt.com:6881, router.bt.ouinet.work:6881"
  webtorrent_stun_server       = ""
  i2p_inbound_quantity         = 3
  i2p_outbound_quantity        = 3
  i2p_inbound_length           = 3
  i2p_outbound_length          = 3
  i2p_inbound_length_variance  = 0
  i2p_outbound_length_variance = 0
}

data "qbittorrent_preferences" "advanced_libtorrent_coverage" {
  depends_on = [qbittorrent_preferences.advanced_libtorrent_coverage]
}
