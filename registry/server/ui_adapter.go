package server

import (
	"context"
	"time"

	"github.com/dejanradmanovic/event-spec/registry"
	"github.com/dejanradmanovic/event-spec/registry/server/ui"
	"github.com/dejanradmanovic/event-spec/spec"
)

// uiStoreAdapter wraps Store to satisfy ui.Store.
// The field-level types are structurally identical; the adapter converts between packages.
type uiStoreAdapter struct {
	st Store
}

func (a *uiStoreAdapter) ListEvents(ctx context.Context, filter registry.ListFilter) ([]spec.EventDef, error) {
	return a.st.ListEvents(ctx, filter)
}

func (a *uiStoreAdapter) GetEvent(ctx context.Context, namespace, name, version string) (*spec.EventDef, error) {
	return a.st.GetEvent(ctx, namespace, name, version)
}

func (a *uiStoreAdapter) LookupAPIKey(ctx context.Context, keyHash string) (string, string, error) {
	return a.st.LookupAPIKey(ctx, keyHash)
}

func (a *uiStoreAdapter) ListAuditLog(ctx context.Context, f ui.AuditFilter) ([]ui.AuditEntry, error) {
	entries, err := a.st.ListAuditLog(ctx, AuditFilter{
		Since:      f.Since,
		Until:      f.Until,
		EntityType: f.EntityType,
		UserID:     f.UserID,
		Limit:      f.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]ui.AuditEntry, len(entries))
	for i, e := range entries {
		out[i] = ui.AuditEntry{
			ID:         e.ID,
			Action:     e.Action,
			EntityType: e.EntityType,
			EntityID:   e.EntityID,
			UserID:     e.UserID,
			Timestamp:  e.Timestamp,
			Details:    e.Details,
		}
	}
	return out, nil
}

func (a *uiStoreAdapter) ListAPIKeys(ctx context.Context) ([]ui.APIKeyRecord, error) {
	records, err := a.st.ListAPIKeys(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ui.APIKeyRecord, len(records))
	for i, r := range records {
		out[i] = ui.APIKeyRecord{
			ID:        r.ID,
			Role:      r.Role,
			Name:      r.Name,
			CreatedBy: r.CreatedBy,
			CreatedAt: r.CreatedAt,
			ExpiresAt: r.ExpiresAt,
		}
	}
	return out, nil
}

func (a *uiStoreAdapter) CreateAPIKey(ctx context.Context, keyHash, role, name, createdBy string, expiresAt *time.Time) (int64, error) {
	return a.st.CreateAPIKey(ctx, keyHash, role, name, createdBy, expiresAt)
}

func (a *uiStoreAdapter) RevokeAPIKey(ctx context.Context, id int64) error {
	return a.st.RevokeAPIKey(ctx, id)
}

func (a *uiStoreAdapter) ListWebhooksAdmin(ctx context.Context) ([]ui.WebhookRecord, error) {
	records, err := a.st.ListWebhooksAdmin(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ui.WebhookRecord, len(records))
	for i, r := range records {
		out[i] = ui.WebhookRecord{
			ID:        r.ID,
			URL:       r.URL,
			CreatedBy: r.CreatedBy,
			CreatedAt: r.CreatedAt,
		}
	}
	return out, nil
}

func (a *uiStoreAdapter) RegisterWebhook(ctx context.Context, webhookURL, userID string) error {
	return a.st.RegisterWebhook(ctx, webhookURL, userID)
}

func (a *uiStoreAdapter) DeleteWebhook(ctx context.Context, id int64) error {
	return a.st.DeleteWebhook(ctx, id)
}

func (a *uiStoreAdapter) ListSettings(ctx context.Context) ([]ui.ServerSetting, error) {
	settings, err := a.st.ListSettings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ui.ServerSetting, len(settings))
	for i, s := range settings {
		out[i] = ui.ServerSetting{Key: s.Key, Value: s.Value}
	}
	return out, nil
}

func (a *uiStoreAdapter) SetSetting(ctx context.Context, key, value string) error {
	// After persisting, apply the in-memory toggle on the server — handled by the caller
	// (handleSetConfig in the UI package delegates back via the adapter).
	return a.st.SetSetting(ctx, key, value)
}
