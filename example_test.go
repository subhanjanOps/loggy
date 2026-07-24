package loggy_test

import (
	"os"
	"time"

	"github.com/subhanjanops/loggy"
)

// fixedClock keeps example output deterministic.
func fixedClock() loggy.Option {
	return loggy.WithTimeFunc(func() time.Time {
		return time.Date(2026, 7, 24, 10, 30, 0, 0, time.UTC)
	})
}

func ExampleNew() {
	l := loggy.New(loggy.WithOutput(os.Stdout), loggy.WithFormat(loggy.JSONFormat), fixedClock())
	l.Info().Str("user", "ada").Int("n", 3).Msg("request handled")
	// Output: {"time":"2026-07-24T10:30:00Z","level":"info","msg":"request handled","user":"ada","n":3}
}

func ExampleLogger_With() {
	l := loggy.New(loggy.WithOutput(os.Stdout), loggy.WithFormat(loggy.JSONFormat), fixedClock())
	reqLog := l.With().Str("request_id", "r-123").Logger()
	reqLog.Info().Int("status", 200).Msg("received")
	// Output: {"time":"2026-07-24T10:30:00Z","level":"info","msg":"received","request_id":"r-123","status":200}
}

func ExampleEvent_Msgf() {
	l := loggy.New(loggy.WithOutput(os.Stdout), loggy.WithFormat(loggy.JSONFormat), fixedClock())
	l.Warn().Int("pct", 87).Msgf("%d%% of quota used", 87)
	// Output: {"time":"2026-07-24T10:30:00Z","level":"warn","msg":"87% of quota used","pct":87}
}

func ExampleWithLevel() {
	l := loggy.New(loggy.WithOutput(os.Stdout), loggy.WithFormat(loggy.JSONFormat),
		loggy.WithLevel(loggy.WarnLevel), fixedClock())
	l.Info().Msg("filtered out") // below Warn, dropped
	l.Warn().Msg("shown")
	// Output: {"time":"2026-07-24T10:30:00Z","level":"warn","msg":"shown"}
}
