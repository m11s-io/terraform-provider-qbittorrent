# Downloads preference coverage. The external programs and email notification
# settings below are stored only; the corresponding enable flags stay false.
resource "qbittorrent_preferences" "downloads" {
  torrent_content_layout = "Subfolder"
  add_to_top_of_queue    = false
  add_stopped_enabled    = false
  torrent_stop_condition = "None"
  merge_trackers         = false
  auto_delete_mode       = 0
  preallocate_all        = false
  incomplete_files_ext   = false
  use_unwanted_folder    = false

  torrent_changed_tmm_enabled       = true
  save_path_changed_tmm_enabled     = true
  category_changed_tmm_enabled      = true
  use_category_paths_in_manual_mode = false

  torrent_files_backup_enabled              = false
  torrent_files_backup_dir                  = "/downloads/torrent-files"
  torrent_files_finished_backup_dir_enabled = false
  torrent_files_finished_backup_dir         = "/downloads/torrent-files/finished"
  remove_torrent_file_backup                = false

  excluded_file_names_enabled = true
  excluded_file_names         = "*.tmp\n*.part"

  mail_notification_enabled         = false
  mail_notification_sender          = "qbittorrent@example.invalid"
  mail_notification_email           = "operator@example.invalid"
  mail_notification_smtp            = "smtp.example.invalid:587"
  mail_notification_encryption_type = "STARTTLS"
  mail_notification_auth_enabled    = false
  mail_notification_username        = "qbittorrent"

  autorun_on_torrent_added_enabled = false
  autorun_on_torrent_added_program = "/usr/bin/printf added"
  autorun_enabled                  = false
  autorun_program                  = "/usr/bin/printf finished"
}
