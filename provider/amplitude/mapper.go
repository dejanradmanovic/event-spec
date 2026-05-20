package amplitude

import (
	"time"
	"unicode/utf8"

	"github.com/dejanradmanovic/event-spec/provider"
)

const maxStringChars = 1024

// amplitudeEvent is the Amplitude HTTP batch event schema.
type amplitudeEvent struct {
	UserID          string         `json:"user_id,omitempty"`
	DeviceID        string         `json:"device_id,omitempty"`
	EventType       string         `json:"event_type"`
	Time            int64          `json:"time"`
	EventProperties map[string]any `json:"event_properties,omitempty"`
	UserProperties  map[string]any `json:"user_properties,omitempty"`
	// Group identify fields â€” only populated for $groupidentify events.
	GroupType       string         `json:"group_type,omitempty"`
	GroupValue      string         `json:"group_value,omitempty"`
	GroupProperties map[string]any `json:"group_properties,omitempty"`
	InsertID        string         `json:"insert_id,omitempty"`
	IP              string         `json:"ip,omitempty"`
	Language        string         `json:"language,omitempty"`
	OSName          string         `json:"os_name,omitempty"`
	Platform        string         `json:"platform,omitempty"`
}

// amplitudeBatchRequest is the payload sent to the Amplitude batch endpoint.
type amplitudeBatchRequest struct {
	APIKey string           `json:"api_key"`
	Events []amplitudeEvent `json:"events"`
}

func mapTrackMessage(msg provider.TrackMessage) amplitudeEvent {
	ts := msg.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	ev := amplitudeEvent{
		UserID:          msg.UserID,
		DeviceID:        msg.AnonymousID,
		EventType:       msg.EventName,
		Time:            ts.UnixMilli(),
		EventProperties: coerceProperties(msg.Properties),
		InsertID:        msg.MessageID,
	}
	applyMessageContext(&ev, msg.MessageContext)
	return ev
}

func mapIdentifyMessage(msg provider.IdentifyMessage) amplitudeEvent {
	ts := msg.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	ev := amplitudeEvent{
		UserID:    msg.UserID,
		DeviceID:  msg.AnonymousID,
		EventType: "$identify",
		Time:      ts.UnixMilli(),
		UserProperties: map[string]any{
			"$set": coerceProperties(msg.Traits),
		},
		InsertID: msg.MessageID,
	}
	applyMessageContext(&ev, msg.MessageContext)
	return ev
}

func mapGroupMessage(msg provider.GroupMessage) amplitudeEvent {
	ts := msg.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	ev := amplitudeEvent{
		UserID:     msg.UserID,
		DeviceID:   msg.AnonymousID,
		EventType:  "$groupidentify",
		Time:       ts.UnixMilli(),
		GroupType:  "group",
		GroupValue: msg.GroupID,
		GroupProperties: map[string]any{
			"$set": coerceProperties(msg.Traits),
		},
		InsertID: msg.MessageID,
	}
	applyMessageContext(&ev, msg.MessageContext)
	return ev
}

// mapAliasMessage maps an alias call to an Amplitude $identify event that
// links PreviousID (device_id) to the new UserID, merging the two identities.
func mapAliasMessage(msg provider.AliasMessage) amplitudeEvent {
	ts := msg.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	ev := amplitudeEvent{
		UserID:    msg.UserID,
		DeviceID:  msg.PreviousID,
		EventType: "$identify",
		Time:      ts.UnixMilli(),
		InsertID:  msg.MessageID,
	}
	applyMessageContext(&ev, msg.MessageContext)
	return ev
}

// applyMessageContext maps the standard MessageContext fields to their Amplitude
// equivalents and merges Extra attributes into EventProperties.
func applyMessageContext(ev *amplitudeEvent, mc provider.MessageContext) {
	if mc.IPAddress != "" {
		ev.IP = mc.IPAddress
	}
	if mc.Locale != "" {
		ev.Language = mc.Locale
	}
	if osName, ok := mc.OS["name"].(string); ok && osName != "" {
		ev.OSName = osName
	}
	if platform, ok := mc.App["platform"].(string); ok && platform != "" {
		ev.Platform = platform
	}
	// Merge Extra attributes into EventProperties so context_properties from
	// AnalyticsContext.Attributes (e.g. session_id, platform) reach the provider.
	if len(mc.Extra) > 0 {
		if ev.EventProperties == nil {
			ev.EventProperties = make(map[string]any, len(mc.Extra))
		}
		for k, v := range mc.Extra {
			if _, exists := ev.EventProperties[k]; !exists {
				ev.EventProperties[k] = coerceValue(v)
			}
		}
	}
}

// coerceProperties applies Amplitude property constraints to every value in props.
// Returns nil if props is nil.
func coerceProperties(props map[string]any) map[string]any {
	if props == nil {
		return nil
	}
	out := make(map[string]any, len(props))
	for k, v := range props {
		out[k] = coerceValue(v)
	}
	return out
}

// coerceValue applies Amplitude type constraints recursively.
// Strings are truncated to maxStringChars characters; arrays and nested maps
// are coerced element-wise; all other types are returned unchanged.
func coerceValue(v any) any {
	switch val := v.(type) {
	case string:
		return truncateString(val)
	case []any:
		out := make([]any, len(val))
		for i, item := range val {
			out[i] = coerceValue(item)
		}
		return out
	case map[string]any:
		return coerceProperties(val)
	default:
		return v
	}
}

// truncateString truncates s to at most maxStringChars Unicode code points.
func truncateString(s string) string {
	n := 0
	for i := range s {
		if n == maxStringChars {
			return s[:i]
		}
		n++
	}
	// Verify the string is valid UTF-8 â€” range loop skips invalid bytes.
	if !utf8.ValidString(s) {
		return string([]rune(s)[:n])
	}
	return s
}
