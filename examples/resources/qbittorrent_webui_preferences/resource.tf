resource "qbittorrent_webui_preferences" "behavior" {
  date_format                    = "default"
  color_scheme                   = "auto"
  display_density                = "default"
  use_alt_row_colors             = false
  dblclick_download              = "1"
  dblclick_complete              = "1"
  dblclick_filter                = "1"
  hide_zero_status_filters       = false
  add_new_torrent_dialog_enabled = true
  full_url_tracker_column        = false
  use_virtual_list               = true
}
