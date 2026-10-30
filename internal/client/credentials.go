package client

import "context"

// CredentialsPatch is a generated write-only qBittorrent preferences patch.
// It is intentionally separate from Preferences, the GET/read model.
type CredentialsPatch = PreferencesPatch

// SetCredentialsPatch sends only the explicitly selected write-only values.
func (c *Client) SetCredentialsPatch(ctx context.Context, patch CredentialsPatch) error {
	return c.SetPreferencesPatch(ctx, patch)
}
