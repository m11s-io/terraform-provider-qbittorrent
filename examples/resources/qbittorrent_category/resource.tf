resource "qbittorrent_category" "datasets" {
  name      = "datasets"
  save_path = "/downloads/datasets"
}

# Referencing the parent orders creation and leaf-first deletion.
resource "qbittorrent_category" "samples" {
  name      = "${qbittorrent_category.datasets.name}/samples"
  save_path = "" # /downloads/datasets/samples, beneath the parent's effective path
}
