data "qbittorrent_preferences" "current" {}

output "current_save_path" {
  value = data.qbittorrent_preferences.current.save_path
}
