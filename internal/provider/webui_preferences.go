package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var webUIPreferenceFields = []preferenceField{
	{"date_format", "string", "Date format: default, MM/dd/yyyy, h:mm:ss AM/PM; MM/dd/yyyy, HH:mm:ss; dd/MM/yyyy, HH:mm:ss; yyyy-MM-dd HH:mm:ss; yyyy/MM/dd HH:mm:ss; dd.MM.yyyy, HH:mm:ss; MMM dd, yyyy, h:mm:ss AM/PM; or dd MMM yyyy, HH:mm:ss."},
	{"color_scheme", "string", "Color scheme: auto, light or dark. auto removes the stored override."},
	{"display_density", "string", "Display density: default or compact. default removes the stored override."},
	{"use_alt_row_colors", "bool", "Use alternating row colors."},
	{"dblclick_download", "string", "Downloading torrent double-click: 0 no action, 1 start/stop."},
	{"dblclick_complete", "string", "Completed torrent double-click: 0 no action, 1 start/stop."},
	{"dblclick_filter", "string", "Filter double-click: 0 no action, 1 reset selection."},
	{"hide_zero_status_filters", "bool", "Automatically hide zero status filters."},
	{"add_new_torrent_dialog_enabled", "bool", "Display torrent content and options when adding torrents."},
	{"full_url_tracker_column", "bool", "Display full announce URL in the Tracker column."},
	{"use_virtual_list", "bool", "Enable optimized table rendering."},
}

func (r *preferencesResource) fields() []preferenceField {
	if r.webui {
		return webUIPreferenceFields
	}
	return preferenceFields
}

func (r *preferencesResource) readPreferences(ctx context.Context) (any, error) {
	if r.webui {
		return r.client.WebUIPreferences(ctx)
	}
	return r.client.Preferences(ctx)
}

func webUIStringValidators(name string, webui bool) []validator.String {
	if !webui {
		return nil
	}
	var allowed []string
	switch name {
	case "color_scheme":
		allowed = []string{"auto", "light", "dark"}
	case "display_density":
		allowed = []string{"default", "compact"}
	case "dblclick_download", "dblclick_complete", "dblclick_filter":
		allowed = []string{"0", "1"}
	case "date_format":
		allowed = []string{"default", "MM/dd/yyyy, h:mm:ss AM/PM", "MM/dd/yyyy, HH:mm:ss", "dd/MM/yyyy, HH:mm:ss", "yyyy-MM-dd HH:mm:ss", "yyyy/MM/dd HH:mm:ss", "dd.MM.yyyy, HH:mm:ss", "MMM dd, yyyy, h:mm:ss AM/PM", "dd MMM yyyy, HH:mm:ss"}
	}
	return []validator.String{stringChoiceValidator{allowed: allowed}}
}

type stringChoiceValidator struct{ allowed []string }

func (v stringChoiceValidator) Description(context.Context) string {
	return fmt.Sprintf("Use one of: %s.", strings.Join(v.allowed, "; "))
}
func (v stringChoiceValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (v stringChoiceValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	for _, s := range v.allowed {
		if s == req.ConfigValue.ValueString() {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(req.Path, "Unsupported WebUI value", v.Description(ctx))
}
