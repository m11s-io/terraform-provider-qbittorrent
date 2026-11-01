package provider

var rssPreferenceFields = []preferenceField{
	field("rss_processing_enabled", "RSS", "bool", "Enable RSS feed processing and fetching."),
	{name: "rss_refresh_interval", wireKey: "rss_refresh_interval", kind: preferenceInt, tab: "RSS", description: "RSS feed refresh interval in minutes. qBittorrent stores this value as supplied.", integerMin: int64ptr(-2147483648), integerMax: int64ptr(2147483647)},
	{name: "rss_fetch_delay", wireKey: "rss_fetch_delay", kind: preferenceInt, tab: "RSS", description: "Delay between requests to the same host in seconds. qBittorrent stores this value as supplied."},
	{name: "rss_max_articles_per_feed", wireKey: "rss_max_articles_per_feed", kind: preferenceInt, tab: "RSS", description: "Maximum articles retained per feed; qBittorrent normalizes negative values to zero.", integerMin: int64ptr(0), integerMax: int64ptr(2147483647)},
	field("rss_auto_downloading_enabled", "RSS", "bool", "Enable RSS auto-download processing. RSS rule definitions are managed separately."),
	field("rss_download_repack_proper_episodes", "RSS", "bool", "Allow REPACK and PROPER episode releases in RSS auto-downloads."),
	field("rss_smart_episode_filters", "RSS", "string", "Newline-delimited RSS smart episode filters; blank lines are preserved."),
}
