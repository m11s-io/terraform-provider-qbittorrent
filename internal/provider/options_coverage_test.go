package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOptionsDispositionMatrixHasVerifiedOwnership(t *testing.T) {
	type entry struct {
		WireKey       string `json:"wire_key"`
		Control       string `json:"control"`
		Disposition   string `json:"disposition"`
		Owner         string `json:"owner"`
		Reason        string `json:"reason"`
		TestReference string `json:"test_reference"`
	}
	var matrix struct {
		Preferences []entry `json:"preferences"`
		Additional  []entry `json:"additional_controls"`
	}
	raw, err := os.ReadFile("../../docs/planning/2026-10-30-options-save-keys.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &matrix); err != nil {
		t.Fatal(err)
	}
	fields := map[string]preferenceField{}
	for _, f := range allPreferenceFields() {
		fields[f.wireKey] = f
	}
	special := map[string]string{"scan_dirs": "qbittorrent_watch_folders", "mail_notification_password": "qbittorrent_credentials", "proxy_password": "qbittorrent_credentials", "dyndns_password": "qbittorrent_credentials", "web_ui_address": "qbittorrent_webui_endpoint", "web_ui_port": "qbittorrent_webui_endpoint", "use_https": "qbittorrent_webui_endpoint", "web_ui_https_cert_path": "qbittorrent_webui_endpoint", "web_ui_https_key_path": "qbittorrent_webui_endpoint", "web_ui_username": "qbittorrent_webui_auth", "web_ui_password": "qbittorrent_webui_auth"}
	seen := map[string]bool{}
	for _, row := range matrix.Preferences {
		if seen[row.WireKey] {
			t.Errorf("duplicate matrix key %s", row.WireKey)
		}
		seen[row.WireKey] = true
		if owner, ok := special[row.WireKey]; ok {
			if row.Disposition != "special resource" || row.Owner != owner {
				t.Errorf("special ownership %s: %+v", row.WireKey, row)
			}
		} else if f, ok := fields[row.WireKey]; ok {
			if f.readOnly {
				if row.Disposition != "computed-only" || row.Owner != "data.qbittorrent_preferences" {
					t.Errorf("read-only ownership %s", row.WireKey)
				}
			} else if row.Owner != "qbittorrent_preferences" || (row.Disposition != "existing" && row.Disposition != "new preference" && row.Disposition != "capability-limited") {
				t.Errorf("scalar ownership %s", row.WireKey)
			}
		} else {
			t.Errorf("unimplemented matrix key %s", row.WireKey)
		}
	}
	if len(matrix.Preferences) == 0 || len(matrix.Additional) == 0 {
		t.Fatal("matrix omits indexed or additional controls")
	}
	for _, row := range append(matrix.Preferences, matrix.Additional...) {
		if row.Disposition == "" || row.Owner == "" || row.Reason == "" || row.TestReference == "" {
			t.Errorf("unaccounted row %+v", row)
			continue
		}
		references := strings.Split(row.TestReference, ";")
		base := filepath.Dir(strings.TrimSpace(references[0]))
		for i, ref := range references {
			ref = strings.TrimSpace(ref)
			if i > 0 && !strings.Contains(ref, "/") {
				ref = filepath.Join(base, ref)
			}
			if _, err := os.Stat(filepath.Join("../..", ref)); err != nil {
				t.Errorf("missing evidence %s: %v", ref, err)
			}
		}
	}
}
