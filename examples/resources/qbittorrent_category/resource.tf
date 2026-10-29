resource "qbittorrent_category" "movies" {
  name      = "movies"
  save_path = "/downloads/movies"
}

# Referencing the parent orders creation and leaf-first deletion.
resource "qbittorrent_category" "kids" {
  name      = "${qbittorrent_category.movies.name}/kids"
  save_path = "" # /downloads/movies/kids, beneath the parent's effective path
}
