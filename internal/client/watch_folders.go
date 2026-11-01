package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/m11s-io/terraform-provider-qbittorrent/internal/client/openapi"
)

type WatchFolderMode string

const (
	WatchFolderModeDefault     WatchFolderMode = "default"
	WatchFolderModeWatchFolder WatchFolderMode = "watch_folder"
	WatchFolderModeCustom      WatchFolderMode = "custom"
)

// WatchFolder is the Terraform-facing typed form of one legacy scan_dirs entry.
type WatchFolder struct {
	SaveMode WatchFolderMode `json:"save_mode"`
	SavePath string          `json:"save_path,omitempty"`
}

// EncodeWatchFolders converts the typed collection into qBittorrent's mixed
// integer/string map. The returned map is a complete replacement, including
// when empty. Null/omitted ownership is handled by the resource before calling.
func EncodeWatchFolders(folders map[string]WatchFolder) (map[string]any, error) {
	wire := make(map[string]any, len(folders))
	for watchedPath, folder := range folders {
		if watchedPath == "" {
			return nil, errors.New("watched folder path must not be empty")
		}
		switch folder.SaveMode {
		case WatchFolderModeDefault:
			if folder.SavePath != "" {
				return nil, fmt.Errorf("watched folder %q: save_path is only valid with save_mode custom", watchedPath)
			}
			wire[watchedPath] = 1
		case WatchFolderModeWatchFolder:
			if folder.SavePath != "" {
				return nil, fmt.Errorf("watched folder %q: save_path is only valid with save_mode custom", watchedPath)
			}
			wire[watchedPath] = 0
		case WatchFolderModeCustom:
			if folder.SavePath == "" {
				return nil, fmt.Errorf("watched folder %q: save_path is required with save_mode custom", watchedPath)
			}
			if folder.SavePath == watchedPath {
				// qBittorrent returns 0 for this path relationship.
				wire[watchedPath] = 0
			} else {
				wire[watchedPath] = folder.SavePath
			}
		default:
			return nil, fmt.Errorf("watched folder %q: unsupported save_mode %q", watchedPath, folder.SaveMode)
		}
	}
	return wire, nil
}

// ValidateWatchFolders checks the complete Terraform-facing map without
// performing a server request.
func ValidateWatchFolders(folders map[string]WatchFolder) error {
	_, err := EncodeWatchFolders(folders)
	return err
}

// DecodeWatchFolders validates the mixed wire union. A nil/omitted value is an
// error so callers cannot confuse it with an explicit empty replacement.
func DecodeWatchFolders(raw json.RawMessage) (map[string]WatchFolder, error) {
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, errors.New("qBittorrent omitted scan_dirs")
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil || entries == nil {
		return nil, errors.New("qBittorrent returned an invalid scan_dirs object")
	}
	folders := make(map[string]WatchFolder, len(entries))
	for watchedPath, rawValue := range entries {
		if watchedPath == "" {
			return nil, errors.New("qBittorrent returned an empty watched folder path")
		}
		var value any
		decoder := json.NewDecoder(bytes.NewReader(rawValue))
		decoder.UseNumber()
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("qBittorrent returned an invalid scan_dirs value for %q", watchedPath)
		}
		switch v := value.(type) {
		case json.Number:
			n, err := v.Int64()
			if err != nil {
				return nil, fmt.Errorf("qBittorrent returned a non-integer scan_dirs value for %q", watchedPath)
			}
			switch n {
			case 0:
				folders[watchedPath] = WatchFolder{SaveMode: WatchFolderModeWatchFolder}
			case 1:
				folders[watchedPath] = WatchFolder{SaveMode: WatchFolderModeDefault}
			default:
				return nil, fmt.Errorf("qBittorrent returned unsupported scan_dirs integer %d for %q", n, watchedPath)
			}
		case string:
			if v == "" {
				return nil, fmt.Errorf("qBittorrent returned an empty custom save path for %q", watchedPath)
			}
			if v == watchedPath {
				folders[watchedPath] = WatchFolder{SaveMode: WatchFolderModeWatchFolder}
			} else {
				folders[watchedPath] = WatchFolder{SaveMode: WatchFolderModeCustom, SavePath: v}
			}
		default:
			return nil, fmt.Errorf("qBittorrent returned an invalid scan_dirs value for %q", watchedPath)
		}
	}
	return folders, nil
}

// WatchFolders reads and validates the complete server collection.
func (c *Client) WatchFolders(ctx context.Context) (map[string]WatchFolder, error) {
	p, err := c.Preferences(ctx)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, errors.New("cannot inspect watched-folder preferences")
	}
	var response map[string]json.RawMessage
	if err := json.Unmarshal(b, &response); err != nil {
		return nil, errors.New("cannot inspect watched-folder preferences")
	}
	return DecodeWatchFolders(response["scan_dirs"])
}

// SetWatchFolders replaces the entire watched-folder collection.
func (c *Client) SetWatchFolders(ctx context.Context, folders map[string]WatchFolder) error {
	wire, err := EncodeWatchFolders(folders)
	if err != nil {
		return err
	}
	b, err := json.Marshal(map[string]any{"scan_dirs": wire})
	if err != nil {
		return errors.New("cannot encode watched folders")
	}
	var patch openapi.PreferencesPatch
	if err := json.Unmarshal(b, &patch); err != nil {
		return errors.New("cannot encode watched folders")
	}
	return c.SetPreferencesPatch(ctx, patch)
}
