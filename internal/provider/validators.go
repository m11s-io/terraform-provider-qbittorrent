package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"math"
	"strings"
)

type preferenceIntValidator struct{ field preferenceField }

func (v preferenceIntValidator) Description(context.Context) string {
	return "Value is outside the supported integer range or enum."
}
func (v preferenceIntValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (v preferenceIntValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	n := req.ConfigValue.ValueInt64()
	inRange := (v.field.integerMin == nil || n >= *v.field.integerMin) && (v.field.integerMax == nil || n <= *v.field.integerMax)
	inEnum := len(v.field.integerEnums) == 0
	for _, x := range v.field.integerEnums {
		inEnum = inEnum || n == x
	}
	isSentinel := false
	for _, x := range v.field.integerSentinels {
		isSentinel = isSentinel || n == x
	}
	valid := (inRange && inEnum) || isSentinel
	if v.field.integerMultipleOf > 0 && !isSentinel && n%v.field.integerMultipleOf != 0 {
		valid = false
	}
	if !valid {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid numeric preference", v.Description(ctx))
	}
}

type preferenceFloatValidator struct{ field preferenceField }

func (v preferenceFloatValidator) Description(context.Context) string {
	return "Value must be finite and within the supported range."
}
func (v preferenceFloatValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (v preferenceFloatValidator) ValidateFloat64(ctx context.Context, req validator.Float64Request, resp *validator.Float64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	n := req.ConfigValue.ValueFloat64()
	if !validPreferenceFloat(v.field, n) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid numeric preference", v.Description(ctx))
	}
}

func validPreferenceFloat(f preferenceField, n float64) bool {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return false
	}
	for _, sentinel := range f.floatSentinels {
		if n == sentinel {
			return true
		}
	}
	return (f.floatMin == nil || n >= *f.floatMin) && (f.floatMax == nil || n <= *f.floatMax)
}

type rangeValidator struct{ min, max int64 }

func (v rangeValidator) Description(context.Context) string {
	return fmt.Sprintf("Value must be between %d and %d.", v.min, v.max)
}
func (v rangeValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (v rangeValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if n := req.ConfigValue.ValueInt64(); n < v.min || n > v.max {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid numeric preference", v.Description(ctx))
	}
}

type numberValidator struct{ port bool }

func (v numberValidator) Description(context.Context) string {
	if v.port {
		return "Port must be between 1 and 65535."
	}
	return "Rate must be 0 or a multiple of 1024 between 1024 and 2147482624 bytes/s."
}
func (v numberValidator) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }
func (v numberValidator) ValidateInt64(ctx context.Context, req validator.Int64Request, resp *validator.Int64Response) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	n := req.ConfigValue.ValueInt64()
	valid := n >= 0 && n <= 2147482624 && n%1024 == 0
	if v.port {
		valid = n >= 1 && n <= 65535
	}
	if !valid {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid numeric preference", v.Description(ctx))
	}
}

type categoryNameValidator struct{}

func (categoryNameValidator) Description(context.Context) string {
	return "Use a nonempty category name without backslashes, newlines, leading/trailing slash or repeated slash."
}
func (v categoryNameValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}
func (v categoryNameValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if !validCategoryName(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid category name", v.Description(ctx))
	}
}
func validCategoryName(s string) bool {
	return s != "" && !strings.ContainsAny(s, "\\\r\n\x00") && !strings.HasPrefix(s, "/") && !strings.HasSuffix(s, "/") && !strings.Contains(s, "//")
}
