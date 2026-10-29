terraform {
  required_providers {
    qbittorrent = {
      source  = "m11s-io/qbittorrent"
      version = "~> 0.1"
    }
  }
}

# Set QBITTORRENT_API_KEY, or QBITTORRENT_USERNAME and QBITTORRENT_PASSWORD.
provider "qbittorrent" {
  url = "https://qbittorrent.example.com"
}
