package validation_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/dejanradmanovic/event-spec/analytics"
	"github.com/dejanradmanovic/event-spec/hooks"
	"github.com/dejanradmanovic/event-spec/hooks/validation"
	"github.com/dejanradmanovic/event-spec/spec"
	"github.com/dejanradmanovic/event-spec/testutil"
)

func TestValidationHook_Before_validEvent_passesThrough(t *testing.T) {
	h := validation.New(lookupFor(productViewedDef()), nil)
	hc := trackHC("Product Viewed", map[string]any{
		"product_id":   "SKU-123",
		"product_name": "Widget",
		"category":     "electronics",
		"price":        29.99,
	})

	got, err := h.Before(context.Background(), hc, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil envelope for valid event, got %+v", got)
	}
}

func TestValidationHook_Before_invalidPropertyType_rejectsEvent(t *testing.T) {
	h := validation.New(lookupFor(productViewedDef()), nil)
	hc := trackHC("Product Viewed", map[string]any{
		"product_id":   "SKU-123",
		"product_name": "Widget",
		"category":     "electronics",
		"price":        "not-a-number", // string instead of number
	})

	_, err := h.Before(context.Background(), hc, nil)
	if err == nil {
		t.Fatal("expected validation error for wrong property type, got nil")
	}

	var verr *validation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("error type: got %T, want *validation.ValidationError", err)
	}
	if verr.EventName != "Product Viewed" {
		t.Errorf("EventName: got %q, want %q", verr.EventName, "Product Viewed")
	}
	if len(verr.Violations) == 0 {
		t.Error("expected at least one violation describing the failed property")
	}
	if err.Error() == "" {
		t.Error("error message must not be empty")
	}
}

func TestValidationHook_Before_missingRequiredProperty_rejectsEvent(t *testing.T) {
	h := validation.New(lookupFor(productViewedDef()), nil)
	hc := trackHC("Product Viewed", map[string]any{
		"product_id": "SKU-123",
		// product_name and price are required but absent
	})

	_, err := h.Before(context.Background(), hc, nil)
	if err == nil {
		t.Fatal("expected validation error for missing required property, got nil")
	}

	var verr *validation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("error type: got %T, want *validation.ValidationError", err)
	}
	if len(verr.Violations) == 0 {
		t.Error("expected violations listing the missing required properties")
	}
}

func TestValidationHook_Before_unknownEvent_passesThrough(t *testing.T) {
	// lookup always returns false → no spec → validation is skipped
	noSpec := func(_ string) (*spec.EventDef, bool) { return nil, false }
	h := validation.New(noSpec, nil)
	hc := trackHC("Legacy Raw Event", map[string]any{"arbitrary": 42})

	got, err := h.Before(context.Background(), hc, nil)
	if err != nil {
		t.Fatalf("unexpected error for event with no spec: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil envelope for unknown event, got %+v", got)
	}
}

func TestValidationHook_Before_nilMessage_passesThrough(t *testing.T) {
	h := validation.New(lookupFor(productViewedDef()), nil)
	hc := hooks.HookContext{
		Operation: "track",
		EventName: "Product Viewed",
		Message:   nil,
	}

	got, err := h.Before(context.Background(), hc, nil)
	if err != nil {
		t.Fatalf("unexpected error when message is nil: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil envelope when message is nil, got %+v", got)
	}
}

func TestValidationHook_Before_rawMapMessage_validatesProperties(t *testing.T) {
	h := validation.New(lookupFor(productViewedDef()), nil)
	hc := hooks.HookContext{
		Operation: "track",
		EventName: "Product Viewed",
		Message: map[string]any{
			"product_id":   "SKU-456",
			"product_name": "Gadget",
			"category":     "books",
			"price":        "oops", // wrong type via raw map
		},
	}

	_, err := h.Before(context.Background(), hc, nil)
	if err == nil {
		t.Fatal("expected validation error for invalid raw map property, got nil")
	}
	var verr *validation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("error type: got %T, want *validation.ValidationError", err)
	}
}

func TestValidationHook_Before_enumViolation_rejectsEvent(t *testing.T) {
	h := validation.New(lookupFor(productViewedDef()), nil)
	hc := trackHC("Product Viewed", map[string]any{
		"product_id":   "SKU-123",
		"product_name": "Widget",
		"category":     "furniture", // not in enum
		"price":        29.99,
	})

	_, err := h.Before(context.Background(), hc, nil)
	if err == nil {
		t.Fatal("expected validation error for enum violation, got nil")
	}
	var verr *validation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("error type: got %T, want *validation.ValidationError", err)
	}
}

func TestValidationHook_Before_patternViolation_rejectsEvent(t *testing.T) {
	h := validation.New(lookupFor(productViewedDef()), nil)
	hc := trackHC("Product Viewed", map[string]any{
		"product_id":   "SKU-123",
		"product_name": "Widget",
		"category":     "electronics",
		"price":        29.99,
		"currency":     "usd", // lowercase violates ^[A-Z]{3}$
	})

	_, err := h.Before(context.Background(), hc, nil)
	if err == nil {
		t.Fatal("expected validation error for pattern violation, got nil")
	}
	var verr *validation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("error type: got %T, want *validation.ValidationError", err)
	}
}

func TestValidationHook_Before_minimumViolation_rejectsEvent(t *testing.T) {
	h := validation.New(lookupFor(productViewedDef()), nil)
	hc := trackHC("Product Viewed", map[string]any{
		"product_id":   "SKU-123",
		"product_name": "Widget",
		"category":     "electronics",
		"price":        -1.0, // below minimum: 0
	})

	_, err := h.Before(context.Background(), hc, nil)
	if err == nil {
		t.Fatal("expected validation error for minimum violation, got nil")
	}
	var verr *validation.ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("error type: got %T, want *validation.ValidationError", err)
	}
}

func TestValidationHook_Before_deletedEvent_returnsErrDeletedEvent(t *testing.T) {
	def := productViewedDef()
	def.Status = spec.StatusDeleted
	h := validation.New(lookupFor(def), nil)
	hc := trackHC("Product Viewed", map[string]any{"product_id": "SKU-1"})

	_, err := h.Before(context.Background(), hc, nil)
	if err == nil {
		t.Fatal("expected error for deleted event, got nil")
	}
	if !errors.Is(err, validation.ErrDeletedEvent) {
		t.Errorf("errors.Is(err, ErrDeletedEvent) = false; err = %v", err)
	}
}

func TestValidationHook_Before_deprecatedEvent_warnsAndProceed(t *testing.T) {
	def := productViewedDef()
	def.Status = spec.StatusDeprecated

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	h := validation.New(lookupFor(def), logger)
	hc := trackHC("Product Viewed", map[string]any{
		"product_id":   "SKU-1",
		"product_name": "Widget",
		"category":     "electronics",
		"price":        9.99,
	})

	got, err := h.Before(context.Background(), hc, nil)
	if err != nil {
		t.Fatalf("unexpected error for deprecated event: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil envelope for valid deprecated event, got %+v", got)
	}
	if !strings.Contains(buf.String(), "deprecated") {
		t.Errorf("expected deprecation warning in log, got: %s", buf.String())
	}
}

func TestValidationHook_Before_deprecatedEvent_nilLogger_noWarningPanic(t *testing.T) {
	def := productViewedDef()
	def.Status = spec.StatusDeprecated
	h := validation.New(lookupFor(def), nil)
	hc := trackHC("Product Viewed", map[string]any{
		"product_id":   "SKU-1",
		"product_name": "Widget",
		"category":     "electronics",
		"price":        9.99,
	})

	got, err := h.Before(context.Background(), hc, nil)
	if err != nil {
		t.Fatalf("unexpected error when logger is nil: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil envelope, got %+v", got)
	}
}

func TestValidationHook_Before_draftEvent_passesThrough(t *testing.T) {
	def := productViewedDef()
	def.Status = spec.StatusDraft
	h := validation.New(lookupFor(def), nil)
	hc := trackHC("Product Viewed", map[string]any{
		"product_id":   "SKU-1",
		"product_name": "Widget",
		"category":     "electronics",
		"price":        9.99,
	})

	got, err := h.Before(context.Background(), hc, nil)
	if err != nil {
		t.Fatalf("draft event must not be blocked: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil envelope for draft event, got %+v", got)
	}
}

func TestValidationHook_Integration_deletedEvent_blocksTrack(t *testing.T) {
	def := productViewedDef()
	def.Status = spec.StatusDeleted

	cap := testutil.NewCaptureProvider("test")
	client := analytics.NewClient(
		analytics.WithProviders(cap),
		analytics.WithHooks(validation.New(lookupFor(def), nil)),
	)

	err := client.Track(context.Background(), analytics.Event{
		Name: "Product Viewed",
		Properties: map[string]any{
			"product_id":   "SKU-1",
			"product_name": "Widget",
			"category":     "electronics",
			"price":        9.99,
		},
	})
	if err == nil {
		t.Fatal("expected error blocking Track for deleted event, got nil")
	}
	if !errors.Is(err, validation.ErrDeletedEvent) {
		t.Errorf("errors.Is(err, ErrDeletedEvent) = false; err = %v", err)
	}
	if len(cap.Tracks) != 0 {
		t.Errorf("expected no events delivered, got %d", len(cap.Tracks))
	}
}

// ---- helpers ----

// trackHC builds a HookContext carrying properties inside an *EventEnvelope.
func trackHC(eventName string, props map[string]any) hooks.HookContext {
	return hooks.HookContext{
		Operation: "track",
		EventName: eventName,
		Message: &hooks.EventEnvelope{
			EventName:  eventName,
			Properties: props,
		},
	}
}

// lookupFor returns a LookupFunc that resolves by EventName.
func lookupFor(def *spec.EventDef) validation.LookupFunc {
	return func(eventName string) (*spec.EventDef, bool) {
		if eventName == def.EventName {
			return def, true
		}
		return nil, false
	}
}

func productViewedDef() *spec.EventDef {
	minPrice := float64(0)
	return &spec.EventDef{
		Name:      "product_viewed",
		EventName: "Product Viewed",
		Namespace: "ecommerce",
		Version:   "1-2-0",
		Status:    spec.StatusActive,
		Type:      spec.TypeTrack,
		Properties: map[string]spec.PropertyDef{
			"product_id":   {Type: spec.PropertyTypeString, Required: true},
			"product_name": {Type: spec.PropertyTypeString, Required: true},
			"category": {
				Type:     spec.PropertyTypeString,
				Required: true,
				Enum:     []string{"clothing", "electronics", "books", "home", "sports", "other"},
			},
			"price":       {Type: spec.PropertyTypeNumber, Required: true, Minimum: &minPrice},
			"currency":    {Type: spec.PropertyTypeString, Required: false, Pattern: "^[A-Z]{3}$"},
			"coupon_code": {Type: spec.PropertyTypeString, Required: false},
		},
	}
}
