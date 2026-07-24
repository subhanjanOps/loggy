# Contributing to loggy

Thanks for your interest in improving loggy! This is a small, dependency-free
library, and the bar for merging is: it stays **fast**, **allocation-free on the
hot path**, **dependency-free**, and **well-tested**. This guide explains how to
get set up and what a good change looks like.

## Getting started

Requires **Go 1.25+**.

```sh
git clone https://github.com/subhanjanops/loggy
cd loggy
go test ./...
```

The repository is **two modules**:

- The library at the root (`github.com/subhanjanops/loggy`) — this must have
  **no dependencies** other than the standard library.
- The benchmark comparison suite in `bench/`, a separate module that imports
  zerolog and zap via a `replace` directive. Keep those dependencies out of the
  root module.

## Before you open a pull request

Run the same checks CI runs:

```sh
gofmt -l .            # must print nothing
go vet ./...
golangci-lint run ./...
go test -race -covermode=atomic -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | tail -1

# lint and build the benchmark module too
cd bench && golangci-lint run ./... && go build ./...
```

- **Formatting:** all code must be `gofmt`-clean.
- **Linting:** [golangci-lint](https://golangci-lint.run) must pass with zero
  issues. The config lives in [`.golangci.yml`](.golangci.yml); install the
  linter with
  `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest`.
  Every exported identifier needs a doc comment (enforced by `revive`).
- **Coverage:** new code should be covered. The suite sits at ~99%; please don't
  regress it. Prefer table-driven tests, and use internal (`package loggy`)
  tests when you need to exercise unexported helpers.
- **No new dependencies** in the root module. If a change seems to need one,
  open an issue first to discuss.

## Performance changes

loggy's headline property is **zero allocations on the enabled and disabled
paths**. If your change touches the hot path (`Event`, `Context`, `emit`, the
`encode.go` helpers), include before/after benchmark numbers:

```sh
cd bench
go test -run '^$' -bench . -benchmem -count=6 > new.txt
# compare against main with benchstat (go install golang.org/x/perf/cmd/benchstat@latest)
benchstat old.txt new.txt
```

A change that adds an allocation to a previously zero-alloc path will not be
merged without a strong justification.

## Coding conventions

- Follow standard Go style (`gofmt`, and the spirit of
  [Effective Go](https://go.dev/doc/effective_go)).
- Every exported identifier needs a doc comment beginning with its name.
- Keep files cohesive and focused (see the existing one-concern-per-file
  layout).
- Prefer clear code over clever code; the encoders are the one place where
  low-level byte pushing is expected, and it is commented as such.

## Commit messages

Write imperative, present-tense summaries (e.g. "add Uint field method"). Keep
the subject under ~72 characters and explain the *why* in the body when it isn't
obvious.

## Reporting bugs and requesting features

Open an [issue](https://github.com/subhanjanops/loggy/issues) with:

- what you expected vs. what happened,
- a minimal reproducer (a few lines using loggy), and
- your Go version and OS.

For feature ideas, describe the use case first — small, composable additions
that preserve the zero-allocation guarantee are the most likely to land.

## Code of Conduct

Be respectful and constructive. Assume good faith, and keep discussion focused
on the technical merits.

## License

By contributing, you agree that your contributions are licensed under the
project's [MIT License](LICENSE).
