package loggy

import (
	"io"
	"time"
)

// Option configures a Logger at construction time. It is deliberately opaque:
// its underlying func operates on the unexported *logger, so callers can only
// obtain Options from the WithX constructors below and pass them to New — they
// cannot forge one or reach into a Logger's internals.
type Option func(*logger)

// colorMode controls level colorization of text output. The zero value is
// colorAuto, so a default logger colorizes only when writing to a terminal.
type colorMode int

const (
	colorAuto colorMode = iota
	colorAlways
	colorNever
)

// WithOutput sets the destination writer. The default is os.Stdout.
func WithOutput(w io.Writer) Option {
	return func(l *logger) { l.out = w }
}

// WithLevel sets the minimum level to emit. The default is InfoLevel.
func WithLevel(lvl Level) Option {
	return func(l *logger) { l.level.Store(int32(lvl)) }
}

// WithFormat selects TextFormat or JSONFormat. The default is JSONFormat.
func WithFormat(f Format) Option {
	return func(l *logger) { l.format = f }
}

// WithName sets a logger name included on every line (as "logger" in JSON).
func WithName(name string) Option {
	return func(l *logger) { l.name = name }
}

// WithCaller enables attaching the "file:line" of the call site to each entry.
func WithCaller(enabled bool) Option {
	return func(l *logger) { l.caller = enabled }
}

// WithStackTrace attaches a stack trace to entries at or above minLevel. The
// default threshold is FatalLevel.
func WithStackTrace(minLevel Level) Option {
	return func(l *logger) { l.stackTrace = minLevel }
}

// WithHook registers a Hook invoked for every emitted entry. It may be called
// multiple times to add several hooks.
func WithHook(h Hook) Option {
	return func(l *logger) { l.hooks = append(l.hooks, h) }
}

// WithSampler sets a Sampler that can drop entries before they are written.
func WithSampler(s Sampler) Option {
	return func(l *logger) { l.sampler = s }
}

// WithTimeFunc overrides the clock used for timestamps. Handy for deterministic
// tests. The default is time.Now.
func WithTimeFunc(fn func() time.Time) Option {
	return func(l *logger) { l.timeFunc = fn }
}

// WithContextExtractor sets the function used by Event.Ctx to pull fields out of
// a context.Context (e.g. a trace id).
func WithContextExtractor(fn ContextExtractor) Option {
	return func(l *logger) { l.ctxExtractor = fn }
}

// WithConcurrentWriter declares that the output io.Writer is safe for
// concurrent Write calls and writes each buffer atomically. The logger then
// skips its internal write mutex, giving lock-free throughput under contention
// (each log line is a single Write, so the writer serializes them).
//
// Enable this ONLY for such writers — os.File, os.Stdout/Stderr, io.Discard, or
// a writer you have wrapped with your own synchronization. Enabling it with a
// writer whose Write is not concurrency-safe (e.g. a bare bytes.Buffer) will
// interleave or corrupt output.
func WithConcurrentWriter() Option {
	return func(l *logger) { l.lockFree = true }
}

// WithColor forces ANSI level colors on (true) or off (false) for text output.
// The default is automatic: colors are emitted only when the output is a
// terminal, so files and pipes stay clean. JSON output is never colorized.
func WithColor(enabled bool) Option {
	return func(l *logger) {
		if enabled {
			l.color = colorAlways
		} else {
			l.color = colorNever
		}
	}
}
