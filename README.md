# loggy

[![Go Reference](https://pkg.go.dev/badge/github.com/subhanjanops/loggy.svg)](https://pkg.go.dev/github.com/subhanjanops/loggy)
[![CI](https://github.com/subhanjanops/loggy/actions/workflows/ci.yml/badge.svg)](https://github.com/subhanjanops/loggy/actions/workflows/ci.yml)
[![golangci-lint](https://img.shields.io/badge/lint-golangci--lint-blue.svg)](.golangci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A small, fast, **dependency-free** structured logger for Go with a chained,
**zero-allocation** builder API.

```go
l := loggy.New(loggy.WithFormat(loggy.JSONFormat))
l.Info().Str("user", "ada").Int("n", 3).Msg("request handled")
// {"time":"2026-07-24T10:30:00Z","level":"info","msg":"request handled","user":"ada","n":3}
```

---

## Contents

- [Philosophy](#philosophy)
- [Features](#features)
- [Install](#install)
- [Getting started](#getting-started)
- [Levels](#levels)
- [Fields](#fields)
- [Formats and color](#formats-and-color)
- [Child loggers](#child-loggers)
- [Context](#context)
- [Hooks](#hooks)
- [Sampling](#sampling)
- [Caller and stack traces](#caller-and-stack-traces)
- [The package-level default](#the-package-level-default)
- [Concurrency](#concurrency)
- [Lifecycle: Sync and Close](#lifecycle-sync-and-close)
- [Options reference](#options-reference)
- [Output format reference](#output-format-reference)
- [Performance](#performance)
- [Contributing](#contributing)
- [License](#license)

## Philosophy

Most logging cost is paid on lines you never read. loggy is built around that
observation, so its design goals are, in order:

- **Zero allocations on the hot path.** Fields are encoded straight into a
  pooled buffer the moment you chain them — no per-call slice of `any`, no
  intermediate `map`, and no reflection for common types. An enabled log
  produces zero heap allocations; a disabled one is a branch and a return.
- **Nothing to pull in.** The entire library is the Go standard library. No
  transitive dependencies to audit, version, or wait on. Terminal detection,
  JSON string escaping, and color are all implemented in-tree.
- **A small surface that is hard to misuse.** The public API is
  interface-driven (`Logger`, `Entry`, `Hook`, `Sampler`) while the hot path
  uses a concrete `*Event`/`*Context` builder for speed. `Option` values are
  opaque — you can only obtain them from the `WithX` constructors, so a
  `Logger`'s internals can't be forged or reached into.
- **Pay only for what you enable.** Caller lookup, stack traces, sampling,
  hooks, and context extraction are all opt-in and cost nothing when off.

The result is a logger that reads like `zerolog`, matches it on allocations, and
carries none of the dependency weight.

## Features

- **Zero allocations** on both the enabled and disabled paths.
- **No dependencies** — standard library only.
- **Structured JSON** or **human-readable text** (with automatic level colors on
  a terminal).
- **Chained builder API**: `l.Info().Str(...).Int(...).Msg(...)`.
- **Typed fields**: `Str`, `Int`, `Int64`, `Float64`, `Bool`, `Dur`, `Err`,
  `Stringer`, and an `Any` reflection fallback.
- **Child loggers** with pre-encoded persistent fields via `With()`.
- **Context extraction** (`Ctx`) for trace ids and the like, plus
  `WithContext`/`FromContext` to carry a logger through a request.
- **Hooks** to react to every entry and **samplers** to control volume.
- **Caller** (`file:line`) and **stack traces**, both opt-in.
- **Package-level default** logger for zero-ceremony logging.
- Opt-in **lock-free** concurrent write path.
- Safe for concurrent use.

## Install

```sh
go get github.com/subhanjanops/loggy
```

Requires Go 1.25+.

## Getting started

Construct a logger with `New` and any number of options, then chain a level, its
fields, and a terminal `Msg`/`Msgf`:

```go
package main

import "github.com/subhanjanops/loggy"

func main() {
	l := loggy.New(
		loggy.WithFormat(loggy.JSONFormat),
		loggy.WithLevel(loggy.InfoLevel),
	)

	l.Info().
		Str("service", "checkout").
		Int("status", 200).
		Msg("request handled")
}
```

Prefer no setup at all? Use the [package-level default](#the-package-level-default):

```go
loggy.InfoPkg("service starting")
loggy.Default().Warn().Int("retries", 3).Msg("retrying")
```

An `Event` must be finished with exactly one `Msg`/`Msgf` and not reused
afterward — the terminal call recycles it back into the pool.

## Levels

Five severity levels, in increasing order: **Debug**, **Info**, **Warn**,
**Error**, **Fatal**.

```go
l.Debug().Msg("verbose detail")
l.Info().Int("status", 200).Msg("request handled")
l.Warn().Dur("took", 1200*time.Millisecond).Msg("slow query")
l.Error().Err(err).Msg("upstream failed")
l.Fatal().Msg("cannot continue") // logs then os.Exit(1)
l.Panic().Msg("invariant broken") // logs then panic(msg)
```

`Fatal()` and `Panic()` both emit at fatal level; `Fatal` then calls
`os.Exit(1)` and `Panic` then `panic`s with the message. Because they terminate
the program, they fire regardless of the configured threshold.

Set the threshold at construction with `WithLevel`, or at runtime with
`SetLevel` (safe to call concurrently). Read it back with `Level()`. For an
expensive-to-build log, gate it with the cheap `Enabled` pre-check:

```go
if l.Enabled(loggy.DebugLevel) {
	l.Debug().Any("snapshot", expensiveDump()).Msg("state")
}
```

Below-threshold calls are a no-op and allocate nothing, so the guard is only
worth it when *computing the fields* is costly.

## Fields

Each typed method appends one field and returns the `Event` for chaining:

| Method | Go type | JSON rendering | Text rendering |
|---|---|---|---|
| `Str(k, v)` | `string` | quoted, escaped | `k=v` |
| `Int(k, v)` | `int` | number | `k=v` |
| `Int64(k, v)` | `int64` | number | `k=v` |
| `Float64(k, v)` | `float64` | number (`NaN`/`±Inf` as strings) | `k=v` |
| `Bool(k, v)` | `bool` | `true`/`false` | `k=v` |
| `Dur(k, d)` | `time.Duration` | integer nanoseconds | `k=250ms` |
| `Err(err)` | `error` | key `error` (`null` if nil) | key `error` (`<nil>` if nil) |
| `Stringer(k, v)` | `fmt.Stringer` | `v.String()`, quoted | `k=v.String()` |
| `Any(k, v)` | `any` | direct for common types, else `json.Marshal` | direct, else `fmt` |

```go
l.Info().
	Str("method", "GET").
	Int("status", 200).
	Float64("elapsed_ms", 12.5).
	Bool("cached", true).
	Dur("ttl", 30*time.Second).
	Msg("served")
```

`Any` handles arbitrary values: common scalars are encoded directly (no
reflection), and only slices, maps, and structs fall back to `json.Marshal` (or
`fmt` in text mode). Reach for the typed methods on hot paths and keep `Any` for
the occasional structured payload.

## Formats and color

```go
loggy.New(loggy.WithFormat(loggy.TextFormat)) // TIME LEVEL [name] msg k=v ...
loggy.New(loggy.WithFormat(loggy.JSONFormat)) // {"time":...,"level":...} (default)
```

Text output colorizes the **level label** automatically when the destination is
a terminal (white=debug, green=info, yellow=warn, red=error, bold-red=fatal), so
files and pipes stay clean. Force it either way with `WithColor(true|false)`.
JSON output is never colorized.

```text
2026-07-24T10:30:00Z INFO [checkout] request handled status=200 cached=true
```

## Child loggers

`With()` builds a child logger that carries **pre-encoded persistent fields** —
they are encoded once, then prepended to every line the child writes:

```go
reqLog := l.With().Str("request_id", "r-123").Logger()
reqLog.Info().Int("status", 200).Msg("received")
// {"time":...,"level":"info","msg":"received","request_id":"r-123","status":200}
```

A child shares the parent's writer, hooks, sampler, and write lock, and inherits
its level, format, name, and options. Building one does **not** mutate the
parent, and children can be nested:

```go
base := loggy.New(loggy.WithName("api"))
svc := base.With().Str("service", "checkout").Logger()
op := svc.With().Str("op", "charge").Logger() // carries service + op
```

## Context

Two independent context integrations:

**Extract fields from a `context.Context`.** Register a `ContextExtractor`, then
call `Ctx` on an event to run it — handy for pulling a trace id out of the
request context:

```go
l := loggy.New(loggy.WithContextExtractor(func(ctx context.Context, e *loggy.Event) {
	if id, ok := ctx.Value(traceKey).(string); ok {
		e.Str("trace_id", id)
	}
}))
l.Info().Ctx(ctx).Str("path", "/orders").Msg("handling request")
```

**Carry a logger through a request.** Stash a (typically child) logger in the
context and retrieve it downstream:

```go
ctx = loggy.WithContext(ctx, reqLog)
loggy.FromContext(ctx).Info().Msg("downstream work")
```

`FromContext` returns the stored logger, or the package default if the context
carries none (or is nil), so it never returns nil.

## Hooks

A `Hook` is invoked for every emitted entry — after sampling, before the line is
written — for side effects like shipping errors to Sentry or bumping a metric:

```go
type Hook interface {
	Fire(Entry) error
}
```

`Entry` exposes the record's scalar metadata (`Time`, `Level`, `Message`,
`Caller`, `Stack`); its structured fields are already encoded to bytes by the
time a hook sees it. Register one or more with `WithHook`:

```go
l := loggy.New(loggy.WithHook(metricsHook{counter}))
```

Hook errors are ignored by the logger, so a hook must handle its own failures.

## Sampling

A `Sampler` decides whether an entry is written at all — use it for rate
limiting or volume control on chatty paths:

```go
type Sampler interface {
	Allow(Entry) bool
}

l := loggy.New(loggy.WithSampler(everyNth{n: 100}))
```

The sampler runs first; entries it rejects are dropped before hooks and before
any bytes are written.

## Caller and stack traces

Both are opt-in and cost nothing when disabled:

```go
l := loggy.New(
	loggy.WithCaller(true),           // attach file:line to every entry
	loggy.WithStackTrace(loggy.ErrorLevel), // stack for entries at/above Error
)
```

`WithCaller` records the call site as `file:line` (rendered as `caller` in JSON,
`(file.go:42)` in text). `WithStackTrace` captures the current goroutine's stack
for any entry at or above the given level; the default threshold is `FatalLevel`,
so fatal/panic entries carry a stack out of the box.

## The package-level default

For quick logging without threading a `Logger` around, the package keeps a
default:

```go
loggy.InfoPkg("service starting")            // Info through the default
loggy.ErrorPkg("disk full")                  // Error through the default
loggy.Default().Warn().Int("n", 3).Msg("hi") // full builder on the default
```

Replace it — typically once at startup — with `SetDefault`:

```go
loggy.SetDefault(loggy.New(loggy.WithFormat(loggy.JSONFormat), loggy.WithName("app")))
```

`Default` and `SetDefault` are safe to call concurrently; the default is created
lazily on first use.

## Concurrency

All methods are safe for concurrent use. By default writes are guarded by a
mutex so any `io.Writer` works. Each log line is assembled in a pooled buffer and
written in a single `Write`.

If your writer is itself concurrency-safe — `os.File`, `os.Stdout`/`Stderr`,
`io.Discard`, or one you've synchronized yourself — pass
`WithConcurrentWriter()` to drop the internal lock and let the writer serialize
the single-`Write`-per-line calls. This is what gives the lock-free parallel
throughput in the benchmarks below.

```go
l := loggy.New(loggy.WithConcurrentWriter()) // default output is os.Stdout, which is safe
```

> **Caution:** enable this **only** for a concurrency-safe writer. Passing it a
> writer whose `Write` is not safe under concurrency (e.g. a bare
> `bytes.Buffer`) will interleave or corrupt output.

## Lifecycle: Sync and Close

`Sync` flushes the underlying writer if it implements `Sync() error` (e.g.
`os.File`); otherwise it's a no-op. `Close` closes the writer if it's an
`io.Closer`. Both are safe to call on any logger:

```go
defer l.Sync()  // flush buffered output on shutdown
defer l.Close() // if the writer owns a file/socket you want closed
```

## Options reference

Every option is passed to `New` and applied in order.

| Option | Purpose | Default |
|---|---|---|
| `WithOutput(w)` | destination `io.Writer` | `os.Stdout` |
| `WithLevel(lvl)` | minimum level to emit | `InfoLevel` |
| `WithFormat(f)` | `TextFormat` or `JSONFormat` | `JSONFormat` |
| `WithName(name)` | logger name on every line (`logger` in JSON) | none |
| `WithCaller(bool)` | attach `file:line` of the call site | off |
| `WithStackTrace(lvl)` | stack trace at or above `lvl` | `FatalLevel` |
| `WithColor(bool)` | force level colors on/off | auto (terminal only) |
| `WithHook(h)` | per-entry hook (repeatable) | none |
| `WithSampler(s)` | volume control | none |
| `WithTimeFunc(fn)` | custom clock (deterministic tests) | `time.Now` |
| `WithContextExtractor(fn)` | derive fields from a `context.Context` | none |
| `WithConcurrentWriter()` | lock-free writes for safe writers | off (mutex) |

Runtime controls on a constructed `Logger`: `SetLevel`, `Level`, `Enabled`,
`Sync`, `Close`, `With`.

## Output format reference

**JSON** — one object per line, keys in a fixed order: `time` (RFC3339 with
nanoseconds), `level`, `logger` (only if named), `msg`, then your fields in the
order added (persistent first, then per-event), then `caller` and `stack` if
enabled.

```json
{"time":"2026-07-24T10:30:00Z","level":"info","logger":"api","msg":"served","status":200}
```

**Text** — `TIME LEVEL [name] msg key=value ...`, then `(file.go:line)` if caller
is on, with any stack trace on the following lines:

```text
2026-07-24T10:30:00Z INFO [api] served status=200 (server.go:88)
```

Durations render as integer nanoseconds in JSON and as a string (`250ms`) in
text. A nil `Err` becomes `null` in JSON and `<nil>` in text. Floats that JSON
can't represent (`NaN`, `±Inf`) are emitted as strings.

## Performance

Benchmarked against [zerolog](https://github.com/rs/zerolog) and
[zap](https://github.com/uber-go/zap), JSON to `io.Discard` (AMD Ryzen 7 7435HS,
Go 1.25). Full suite in [`bench/`](bench):

| Scenario | loggy | zerolog | zap |
|---|---|---|---|
| No fields | 141 ns · 0 B · 0 allocs | 118 ns · 0/0 | 243 ns · 0/0 |
| 3 fields | 202 ns · 0 B · 0 allocs | 164 ns · 0/0 | 453 ns · 192 B/1 |
| 10 fields | 410 ns · 0 B · 0 allocs | 343 ns · 0/0 | 818 ns · 705 B/1 |
| Accumulated context + fields | **209 ns** · 0/0 | 229 ns · 0/0 | 493 ns · 192 B/1 |
| Disabled + fields | **4.5 ns** · 0/0 | 7.6 ns · 0/0 | 62 ns · 192 B/1 |
| With caller | **669 ns** | 1108 ns | 947 ns |
| Concurrent (`WithConcurrentWriter`) | 21 ns · 0/0 | 21 ns · 0/0 | 114 ns · 128 B/1 |

loggy allocates zero on every enabled and disabled path (matching zerolog),
beats zap across the board, and beats zerolog on accumulated-context and
disabled workloads.

Run them yourself:

```sh
cd bench && go test -run '^$' -bench . -benchmem
```

## Contributing

Contributions are welcome — see [CONTRIBUTING.md](CONTRIBUTING.md). In short:
keep the library dependency-free and allocation-free on the hot path, run
`gofmt`/`go vet`/`golangci-lint run`/`go test -race`, and include benchmark
numbers for hot-path changes.

CI enforces formatting, `go vet`, [golangci-lint](https://golangci-lint.run)
(config in [`.golangci.yml`](.golangci.yml)), and the race-enabled test suite on
every push and pull request.

## License

[MIT](LICENSE) © 2026 Subhanjan Adhikary
