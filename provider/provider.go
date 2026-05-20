package provider

import (
	"context"
	"errors"
	"time"

	"github.com/dejanradmanovic/event-spec/hooks"
)

// ErrUnsupportedOperation is returned by provider methods that have no equivalent
// in the underlying vendor API, preventing silent data loss.
var ErrUnsupportedOperation = errors.New("unsupported provider operation")

// ProviderCapabilities advertises which analytics operations a provider supports.
type ProviderCapabilities struct {
	Track    bool
	Identify bool
	Group    bool // not all vendors have first-class group support
	Page     bool // web-centric; mobile/backend providers often omit this
	Alias    bool // identity-merge semantics differ heavily between vendors
}

// ProviderMetadata contains descriptive information about a provider implementation.
type ProviderMetadata struct {
	Name         string
	Version      string
	Capabilities ProviderCapabilities
}

// MessageContext contains environment and device metadata attached to every message.
type MessageContext struct {
	Library   map[string]any
	App       map[string]any
	Device    map[string]any
	OS        map[string]any
	Network   map[string]any
	Screen    map[string]any
	Locale    string
	Timezone  string
	UserAgent string
	IPAddress string
	Extra     map[string]any
}

// TrackMessage is the fully-resolved payload for a track call.
type TrackMessage struct {
	MessageID      string
	Timestamp      time.Time
	EventName      string
	Properties     map[string]any
	UserID         string
	AnonymousID    string
	MessageContext MessageContext
}

// IdentifyMessage is the fully-resolved payload for an identify call.
type IdentifyMessage struct {
	MessageID      string
	Timestamp      time.Time
	UserID         string
	AnonymousID    string
	Traits         map[string]any
	MessageContext MessageContext
}

// GroupMessage is the fully-resolved payload for a group call.
type GroupMessage struct {
	MessageID      string
	Timestamp      time.Time
	UserID         string
	AnonymousID    string
	GroupID        string
	Traits         map[string]any
	MessageContext MessageContext
}

// PageMessage is the fully-resolved payload for a page call.
type PageMessage struct {
	MessageID      string
	Timestamp      time.Time
	UserID         string
	AnonymousID    string
	Name           string
	Properties     map[string]any
	MessageContext MessageContext
}

// AliasMessage is the fully-resolved payload for an alias call.
type AliasMessage struct {
	MessageID      string
	Timestamp      time.Time
	UserID         string
	PreviousID     string
	MessageContext MessageContext
}

// Provider is the interface every analytics destination adapter must satisfy.
// The single interface is intentional â€” providers that don't support a method
// return ErrUnsupportedOperation rather than silently no-oping.
type Provider interface {
	Metadata() ProviderMetadata
	Hooks() []hooks.Hook

	Track(ctx context.Context, event TrackMessage) error
	Identify(ctx context.Context, msg IdentifyMessage) error
	Group(ctx context.Context, msg GroupMessage) error
	Page(ctx context.Context, msg PageMessage) error
	Alias(ctx context.Context, msg AliasMessage) error

	Flush(ctx context.Context) error
	Shutdown(ctx context.Context) error
}
