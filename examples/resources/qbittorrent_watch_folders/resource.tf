resource "qbittorrent_watch_folders" "application" {
  folders = {
    "/downloads/watch-default" = {
      save_mode = "default"
    }
    "/downloads/watch-here" = {
      save_mode = "watch_folder"
    }
    "/downloads/watch-custom" = {
      save_mode = "custom"
      save_path = "/downloads/complete"
    }
  }
}
