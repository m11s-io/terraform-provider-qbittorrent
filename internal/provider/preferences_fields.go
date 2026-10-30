package provider

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
)

type preferenceKind string

const (
	preferenceString preferenceKind = "string"
	preferenceBool   preferenceKind = "bool"
	preferenceInt    preferenceKind = "int"
	preferenceFloat  preferenceKind = "float"
)

// preferenceField is the single metadata contract for generic preference
// schema, patch and refresh behavior. A nil bound means the value is open.
type preferenceField struct {
	name, wireKey                              string
	kind                                       preferenceKind
	tab, description                           string
	resourceDescriptionSuffix                  string
	integerMin, integerMax                     *int64
	integerSentinels                           []int64
	integerMultipleOf                          int64
	floatMin, floatMax                         *float64
	stringEnums                                []string
	integerEnums                               []int64
	sensitive, readOnly, excludeFromDataSource bool
	capability                                 string
}

func int64ptr(v int64) *int64       { return &v }
func float64ptr(v float64) *float64 { return &v }

func field(name, tab, kind, description string) preferenceField {
	return preferenceField{name: name, wireKey: name, kind: preferenceKind(kind), tab: tab, description: description}
}

var legacyPreferenceFields = []preferenceField{
	field("locale", "Behavior", "string", "Behavior: user interface language (upstream locale identifier)."),
	field("confirm_torrent_deletion", "Behavior", "bool", "Behavior: confirm torrent deletion."),
	field("status_bar_external_ip", "Behavior", "bool", "Behavior: show external IP in the status bar."),
	field("store_search_jobs", "Behavior", "bool", "Behavior: save search tabs."),
	field("store_search_job_results", "Behavior", "bool", "Behavior: also save search results."),
	field("performance_warning", "Behavior", "bool", "Behavior: log performance warnings."),
	field("file_log_enabled", "Behavior", "bool", "Behavior: enable file logging."),
	field("file_log_path", "Behavior", "string", "Behavior: log directory on the server."),
	field("file_log_backup_enabled", "Behavior", "bool", "Behavior: rotate log files by size."),
	{name: "file_log_max_size", wireKey: "file_log_max_size", kind: preferenceInt, tab: "Behavior", description: "Behavior: rotate logs after this size in KiB, 1..1024000.", integerMin: int64ptr(1), integerMax: int64ptr(1024000)},
	field("file_log_delete_old", "Behavior", "bool", "Behavior: delete old backup logs."),
	{name: "file_log_age", wireKey: "file_log_age", kind: preferenceInt, tab: "Behavior", description: "Behavior: backup log retention count, 1..365.", integerMin: int64ptr(1), integerMax: int64ptr(365)},
	{name: "file_log_age_type", wireKey: "file_log_age_type", kind: preferenceInt, tab: "Behavior", description: "Behavior: retention unit, 0 days, 1 months, 2 years.", integerMin: int64ptr(0), integerMax: int64ptr(2), integerEnums: []int64{0, 1, 2}},
	field("save_path", "Downloads", "string", "Default save path on the server. Use a normalized absolute path."),
	field("temp_path", "Downloads", "string", "Incomplete-download path on the server. Use a normalized absolute path."),
	field("temp_path_enabled", "Downloads", "bool", "Enable the incomplete-download path."),
	field("auto_tmm_enabled", "Downloads", "bool", "Enable automatic torrent management by default. Changes may relocate files."),
	{name: "dl_limit", wireKey: "dl_limit", kind: preferenceInt, tab: "Speed", description: "Global download limit in bytes/s; 0 means unlimited.", resourceDescriptionSuffix: " Rate limits must be multiples of 1024, at most 2147482624.", integerMin: int64ptr(0), integerMax: int64ptr(2147482624), integerMultipleOf: 1024},
	{name: "up_limit", wireKey: "up_limit", kind: preferenceInt, tab: "Speed", description: "Global upload limit in bytes/s; 0 means unlimited.", resourceDescriptionSuffix: " Rate limits must be multiples of 1024, at most 2147482624.", integerMin: int64ptr(0), integerMax: int64ptr(2147482624), integerMultipleOf: 1024},
	{name: "alt_dl_limit", wireKey: "alt_dl_limit", kind: preferenceInt, tab: "Speed", description: "Alternate download limit in bytes/s; 0 means unlimited.", resourceDescriptionSuffix: " Rate limits must be multiples of 1024, at most 2147482624.", integerMin: int64ptr(0), integerMax: int64ptr(2147482624), integerMultipleOf: 1024},
	{name: "alt_up_limit", wireKey: "alt_up_limit", kind: preferenceInt, tab: "Speed", description: "Alternate upload limit in bytes/s; 0 means unlimited.", resourceDescriptionSuffix: " Rate limits must be multiples of 1024, at most 2147482624.", integerMin: int64ptr(0), integerMax: int64ptr(2147482624), integerMultipleOf: 1024},
	field("scheduler_enabled", "Speed", "bool", "Schedule the use of alternative rate limits using the server's local time."),
	{name: "schedule_from_hour", wireKey: "schedule_from_hour", kind: preferenceInt, tab: "Speed", description: "Schedule start hour, 0..23. Configure together with schedule_from_min.", integerMin: int64ptr(0), integerMax: int64ptr(23)},
	{name: "schedule_from_min", wireKey: "schedule_from_min", kind: preferenceInt, tab: "Speed", description: "Schedule start minute, 0..59. Configure together with schedule_from_hour.", integerMin: int64ptr(0), integerMax: int64ptr(59)},
	{name: "schedule_to_hour", wireKey: "schedule_to_hour", kind: preferenceInt, tab: "Speed", description: "Schedule end hour, 0..23. Configure together with schedule_to_min.", integerMin: int64ptr(0), integerMax: int64ptr(23)},
	{name: "schedule_to_min", wireKey: "schedule_to_min", kind: preferenceInt, tab: "Speed", description: "Schedule end minute, 0..59. Configure together with schedule_to_hour.", integerMin: int64ptr(0), integerMax: int64ptr(59)},
	{name: "scheduler_days", wireKey: "scheduler_days", kind: preferenceInt, tab: "Speed", description: "Schedule days: 0 every day, 1 weekdays, 2 weekends, 3 Monday, 4 Tuesday, 5 Wednesday, 6 Thursday, 7 Friday, 8 Saturday, 9 Sunday.", integerMin: int64ptr(0), integerMax: int64ptr(9), integerEnums: []int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
	field("limit_utp_rate", "Speed", "bool", "Apply rate limits to the uTP protocol."),
	field("limit_tcp_overhead", "Speed", "bool", "Apply rate limits to transport overhead."),
	field("limit_lan_peers", "Speed", "bool", "Apply rate limits to peers on the LAN."),
	field("dht", "BitTorrent", "bool", "Enable DHT."),
	field("pex", "BitTorrent", "bool", "Enable peer exchange."),
	field("lsd", "BitTorrent", "bool", "Enable local service discovery."),
	{name: "listen_port", wireKey: "listen_port", kind: preferenceInt, tab: "Connection", description: "Incoming listen port, 1..65535. Disable random-port settings on the server before managing this field.", integerMin: int64ptr(1), integerMax: int64ptr(65535)},
	field("upnp", "Connection", "bool", "Enable UPnP/NAT-PMP port forwarding."),
}

func allPreferenceFields() []preferenceField {
	fields := make([]preferenceField, 0, len(legacyPreferenceFields)+len(downloadsPreferenceFields)+len(connectionPreferenceFields)+len(bittorrentPreferenceFields)+len(rssPreferenceFields)+len(advancedSessionPreferenceFields)+len(webUISettingsPreferenceFields)+len(advancedLibtorrentPreferenceFields))
	for _, group := range [][]preferenceField{legacyPreferenceFields, downloadsPreferenceFields, connectionPreferenceFields, bittorrentPreferenceFields, rssPreferenceFields, advancedSessionPreferenceFields, webUISettingsPreferenceFields, advancedLibtorrentPreferenceFields} {
		fields = append(fields, group...)
	}
	return fields
}

func validatePreferenceFields(fields []preferenceField) error {
	wires := make(map[string]string, len(fields))
	names := make(map[string]string, len(fields))
	for _, f := range fields {
		if f.name == "" || f.wireKey == "" || f.tab == "" || f.description == "" {
			return fmt.Errorf("preference metadata requires name, wire key, tab, and description")
		}
		if prior, ok := wires[f.wireKey]; ok {
			return fmt.Errorf("wire key %q has duplicate ownership by %q and %q", f.wireKey, prior, f.name)
		}
		if prior, ok := names[f.name]; ok {
			return fmt.Errorf("preference name %q has duplicate ownership in %q and %q", f.name, prior, f.tab)
		}
		wires[f.wireKey], names[f.name] = f.name, f.tab
	}
	return nil
}

func fieldWireKey(fields []preferenceField, name string) string {
	for _, f := range fields {
		if f.name == name {
			return f.wireKey
		}
	}
	return name
}

func fieldReadOnly(fields []preferenceField, name string) bool {
	for _, f := range fields {
		if f.name == name {
			return f.readOnly
		}
	}
	return false
}

func findPreferenceField(fields []preferenceField, name string) (preferenceField, bool) {
	for _, f := range fields {
		if f.name == name {
			return f, true
		}
	}
	return preferenceField{}, false
}

func preferenceResourceDescription(f preferenceField) string {
	return f.description + f.resourceDescriptionSuffix
}

func preferencePatchEntry(fields []preferenceField, name string, value any) (string, any, bool) {
	for _, f := range fields {
		if f.name == name {
			if f.readOnly {
				return "", nil, false
			}
			return f.wireKey, value, true
		}
	}
	return name, value, true
}

func configuredCapabilityFields(fields []preferenceField, values map[string]attr.Value) []preferenceField {
	var configured []preferenceField
	for _, f := range fields {
		v, ok := values[f.name]
		if ok && f.capability != "" && !v.IsNull() && !v.IsUnknown() {
			configured = append(configured, f)
		}
	}
	return configured
}

func unsupportedCapabilityFields(fields []preferenceField, remote map[string]json.RawMessage) []preferenceField {
	var unsupported []preferenceField
	for _, f := range fields {
		if _, ok := remote[f.wireKey]; !ok {
			unsupported = append(unsupported, f)
		}
	}
	return unsupported
}
