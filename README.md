# loggy

[![Go Reference](https://pkg.go.dev/badge/github.com/subhanjanops/loggy.svg)](https://pkg.go.dev/github.com/subhanjanops/loggy)
[![Go Report Card](https://goreportcard.com/badge/github.com/subhanjanops/loggy)](https://goreportcard.com/report/github.com/subhanjanops/loggy)
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

## Features

- **Zero allocations** on the hot path — fields are encoded straight into a
  pooled buffer as you chain them; no per-call slice, no map, no reflection for
  common types.
- **No dependencies** — standard library only.
- **Structured JSON** or **human-readable text** (with automatic level colors on
  a terminal).
- **Chained builder API**: `l.Info().Str(...).Int(...).Msg(...)`.
- **Child loggers** with pre-encoded persistent fields via `With()`.
- **Context extraction** (`Ctx`), **hooks**, **samplers**, **caller** and
  **stack traces**, a **package-level default**, and an opt-in **lock-free**
  concurrent write path.
- Safe for concurrent use.

## Install

```sh
go get github.com/subhanjanops/loggy
```

Requires Go 1.25+.

## Usage

### Levels

```go
l.Debug().Msg("verbose detail")
l.Info().Int("status", 200).Msg("request handled")
l.Warn().Dur("took", 1200*time.Millisecond).Msg("slow query")
l.Error().Err(err).Msg("upstream failed")
l.Fatal().Msg("cannot continue") // logs then os.Exit(1)
l.Panic().Msg("invariant broken") // logs then panic(msg)
```

Set the threshold with `WithLevel` or `SetLevel`; check it cheaply with
`Enabled`. Below-threshold calls are a no-op and allocate nothing.

### Fields

`Str`, `Int`, `Int64`, `Float64`, `Bool`, `Dur`, `Err`, `Stringer`, and `Any`
(reflection fallback for slices/maps/structs).

### Child loggers

```go
reqLog := l.With().Str("request_id", "r-123").Logger()
reqLog.Info().Msg("received") // includes request_id; parent is unchanged
```

### Formats and color

```go
loggy.New(loggy.WithFormat(loggy.TextFormat)) // TIME LEVEL [name] msg k=v ...
loggy.New(loggy.WithFormat(loggy.JSONFormat)) // {"time":...,"level":...}
```

Text output colorizes the level automatically when writing to a terminal
(white=debug, green=info, yellow=warn, red=error). Force it with
`WithColor(true|false)`.

### Context

```go
l := loggy.New(loggy.WithContextExtractor(func(ctx context.Context, e *loggy.Event) {
	if id, ok := ctx.Value(traceKey).(string); ok {
		e.Str("trace_id", id)
	}
}))
l.Info().Ctx(ctx).Str("path", "/orders").Msg("handling request")

// Or carry a logger through a request:
ctx = loggy.WithContext(ctx, reqLog)
loggy.FromContext(ctx).Info().Msg("downstream work")
```

### Hooks and sampling

```go
loggy.WithHook(myHook)        // Fire(Entry) error, called per entry
loggy.WithSampler(mySampler)  // Allow(Entry) bool, drops entries
```

### Options

| Option | Purpose |
|---|---|
| `WithOutput(w)` | destination writer (default `os.Stdout`) |
| `WithLevel(lvl)` | minimum level (default `InfoLevel`) |
| `WithFormat(f)` | `TextFormat` or `JSONFormat` (default JSON) |
| `WithName(name)` | logger name on every line |
| `WithCaller(true)` | attach `file:line` |
| `WithStackTrace(lvl)` | stack trace at/above `lvl` |
| `WithColor(bool)` | force level colors on/off |
| `WithHook(h)` | per-entry hook |
| `WithSampler(s)` | volume control |
| `WithTimeFunc(fn)` | custom clock (deterministic tests) |
| `WithContextExtractor(fn)` | fields from `context.Context` |
| `WithConcurrentWriter()` | lock-free writes (see below) |

## Concurrency

All methods are safe for concurrent use. By default writes are guarded by a
mutex so any `io.Writer` works. If your writer is itself concurrency-safe
(`os.File`, `os.Stdout`/`Stderr`, `io.Discard`, or one you've synchronized),
pass `WithConcurrentWriter()` to drop the lock for higher parallel throughput.

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
