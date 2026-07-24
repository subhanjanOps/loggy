package loggy

import (
	"context"
	"time"
)

// Entry is one assembled log record, handed to Hooks and Samplers before it is
// written. Fields are not exposed here: on the streaming builder path they are
// already encoded to bytes, so an Entry carries only the record's scalar
// metadata.
type Entry interface {
	Time() time.Time
	Level() Level
	Message() string
	Caller() string
	Stack() string
}

// entry is the unexported concrete implementation of Entry.
type entry struct {
	time    time.Time
	level   Level
	message string
	caller  string
	stack   string
}

func (e entry) Time() time.Time { return e.time }
func (e entry) Level() Level    { return e.level }
func (e entry) Message() string { return e.message }
func (e entry) Caller() string  { return e.caller }
func (e entry) Stack() string   { return e.stack }

// Hook lets external code react to every emitted entry (e.g. shipping errors
// to Sentry, incrementing metrics counters).
type Hook interface {
	Fire(Entry) error
}

// Sampler decides whether a given entry should be emitted at all
// (rate limiting / log volume control).
type Sampler interface {
	Allow(Entry) bool
}

// ContextExtractor pulls values out of a context.Context and adds them to the
// event — e.g. attaching trace_id/span_id. It writes directly through the
// Event's chainable methods, so no intermediate field type is needed:
//
//	func(ctx context.Context, e *Event) {
//	    if id, ok := ctx.Value(traceKey).(string); ok {
//	        e.Str("trace_id", id)
//	    }
//	}
type ContextExtractor func(ctx context.Context, e *Event)
