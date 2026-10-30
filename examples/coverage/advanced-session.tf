# Advanced qBittorrent session options. Interface selection and embedded tracker
# are left untouched/disabled; choosing SQLite for resume storage requires an
# explicit restart and is covered only by the disposable restart acceptance.
resource "qbittorrent_preferences" "advanced_session" {
  resume_data_storage_type = "Legacy"
  torrent_content_remove_option = "MoveToTrash"
  memory_working_set_limit = 512

  save_resume_data_interval = 10
  save_statistics_interval = 15
  # The UI displays MiB; the provider and API use bytes.
  torrent_file_size_limit = 104857600

  confirm_torrent_recheck = false
  recheck_completed_torrents = false
  app_instance_name = ""
  refresh_interval = 1500
  resolve_peer_host_names = false
  resolve_peer_countries = false
  confirm_remove_all_tags = true
  confirm_remove_tracker_from_all_torrents = true
  reannounce_when_address_changed = false

  enable_embedded_tracker = false
  embedded_tracker_port_forwarding = false
  mark_of_the_web = false
  ignore_ssl_errors = false
  python_executable_path = ""
  start_paused = false
  shutdown_timeout = -1
}

data "qbittorrent_preferences" "advanced_session" {
  depends_on = [qbittorrent_preferences.advanced_session]
}
