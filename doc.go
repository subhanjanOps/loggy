// Package loggy is a small, fast, dependency-free structured logger with a
// chained, zero-allocation builder API.
//
//	l := loggy.New(loggy.WithFormat(loggy.JSONFormat))
//	l.Info().Str("user", "ada").Int("n", 3).Msg("request handled")
//	// {"time":"...","level":"info","msg":"request handled","user":"ada","n":3}
//
// # Design
//
// The public surface is interface-driven (Logger, Entry, Hook, Sampler) with a
// concrete *Event and *Context builder on the hot path for speed. Fields are
// encoded straight into a pooled buffer as they are chained — no per-call slice,
// no map, and no reflection for common types — so an enabled log costs zero heap
// allocations and a disabled one is a cheap no-op.
//
// # Levels
//
// Five severity levels are available: Debug, Info, Warn, Error, and Fatal. The
// Fatal and Panic entry points both emit at fatal level; Fatal then calls
// os.Exit(1) and Panic then panics with the message. The active threshold is
// set with WithLevel or SetLevel and checked cheaply via Enabled.
//
// # Child loggers
//
// With begins a child-logger builder that carries pre-encoded persistent fields:
//
//	reqLog := l.With().Str("request_id", "r-123").Logger()
//	reqLog.Info().Msg("received") // includes request_id
//
// # Formats and color
//
// JSONFormat (default) emits one JSON object per line; TextFormat emits a
// human-readable line and, when writing to a terminal, colorizes the level.
//
// # Concurrency
//
// All methods are safe for concurrent use. By default writes are guarded by a
// mutex so any io.Writer works. If the writer is itself concurrency-safe, pass
// WithConcurrentWriter to drop the lock for higher parallel throughput.
package loggy
