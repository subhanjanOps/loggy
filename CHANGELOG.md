# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2026-07-24

### Added

- Initial release of `loggy`: a dependency-free, zero-allocation structured
  logger with a chained builder API.
- JSON and text formats; automatic level colors on a terminal.
- Five severity levels: Debug, Info, Warn, Error, Fatal. `Fatal()` and
  `Panic()` both emit at fatal level, then exit and panic respectively.
- `With()` child loggers with pre-encoded persistent fields.
- `Event.Ctx` context extraction, hooks, samplers, caller and stack traces.
- Package-level default logger (`Default`, `SetDefault`, `InfoPkg`, `ErrorPkg`).
- `WithConcurrentWriter` opt-in lock-free write path.
- Comparative benchmark suite against the standard library's `log/slog`,
  zerolog, and zap in `bench/`.

### Tooling

- GitHub Actions CI enforcing `gofmt`, `go vet`, golangci-lint, and the
  race-enabled test suite (~99% coverage) on every push and pull request.
- golangci-lint configuration (`.golangci.yml`, v2) with the standard linters
  plus bodyclose, gocritic, misspell, revive, unconvert, and unparam; run
  against both the library and the `bench/` module.

[Unreleased]: https://github.com/subhanjanops/loggy/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/subhanjanops/loggy/releases/tag/v1.0.0
