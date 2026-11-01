package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var webUIPreferenceFields = []preferenceField{
	field("date_format", "Behavior", "string", "Date format: default, MM/dd/yyyy, h:mm:ss AM/PM; MM/dd/yyyy, HH:mm:ss; dd/MM/yyyy, HH:mm:ss; yyyy-MM-dd HH:mm:ss; yyyy/MM/dd HH:mm:ss; dd.MM.yyyy, HH:mm:ss; MMM dd, yyyy, h:mm:ss AM/PM; or dd MMM yyyy, HH:mm:ss."),
	field("color_scheme", "Behavior", "string", "Color scheme: auto, light or dark. auto removes the stored override."),
	field("display_density", "Behavior", "string", "Display density: default or compact. default removes the stored override."),
	field("use_alt_row_colors", "Behavior", "bool", "Use alternating row colors."),
	field("dblclick_download", "Behavior", "string", "Downloading torrent double-click: 0 no action, 1 start/stop."),
	field("dblclick_complete", "Behavior", "string", "Completed torrent double-click: 0 no action, 1 start/stop."),
	field("dblclick_filter", "Behavior", "string", "Filter double-click: 0 no action, 1 reset selection."),
	field("hide_zero_status_filters", "Behavior", "bool", "Automatically hide zero status filters."),
	field("add_new_torrent_dialog_enabled", "Behavior", "bool", "Display torrent content and options when adding torrents."),
	field("full_url_tracker_column", "Behavior", "bool", "Display full announce URL in the Tracker column."),
	field("use_virtual_list", "Behavior", "bool", "Enable optimized table rendering."),
}

func (r *preferencesResource) fields() []preferenceField {
	if r.webui {
		return webUIPreferenceFields
	}
	return allPreferenceFields()
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
