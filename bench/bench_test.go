// Package bench compares github.com/subhanjanops/loggy against zerolog and zap.
//
// Methodology / fairness:
//   - All three write JSON to io.Discard with a timestamp and no caller
//     (except the Caller benchmark), no sampling, no hooks.
//   - Each scenario is a top-level Benchmark with a b.Run sub-benchmark per
//     library, so results line up as Scenario/practice, Scenario/zerolog,
//     Scenario/zap for direct comparison.
//   - Duration/float encodings differ slightly between libraries (ns int vs
//     ms number vs float seconds); the amount of work is comparable.
//
// Run:  go test -run '^$' -bench . -benchmem ./bench/
package bench

import (
	"errors"
	"io"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/subhanjanops/loggy"
)

var (
	errSample = errors.New("connection refused")
	durSample = 12 * time.Millisecond
)

// ---------------------------------------------------------------------
// Constructors — all to io.Discard, JSON, timestamped, level Info.
// ---------------------------------------------------------------------

func newPractice(opts ...loggy.Option) loggy.Logger {
	base := []loggy.Option{loggy.WithOutput(io.Discard), loggy.WithFormat(loggy.JSONFormat)}
	return loggy.New(append(base, opts...)...)
}

func newZerolog() zerolog.Logger {
	return zerolog.New(io.Discard).With().Timestamp().Logger()
}

func newZap(opts ...zap.Option) *zap.Logger {
	enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.AddSync(io.Discard), zapcore.InfoLevel)
	return zap.New(core, opts...)
}

// ---------------------------------------------------------------------
// 1. Message only — no fields.
// ---------------------------------------------------------------------

func BenchmarkNoFields(b *testing.B) {
	b.Run("practice", func(b *testing.B) {
		l := newPractice()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Msg("hello world")
		}
	})
	b.Run("zerolog", func(b *testing.B) {
		l := newZerolog()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Msg("hello world")
		}
	})
	b.Run("zap", func(b *testing.B) {
		l := newZap()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info("hello world")
		}
	})
}

// ---------------------------------------------------------------------
// 2. Small payload — 3 fields (2 strings + 1 int).
// ---------------------------------------------------------------------

func BenchmarkSmall(b *testing.B) {
	b.Run("practice", func(b *testing.B) {
		l := newPractice()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Str("method", "GET").Str("path", "/api/orders").Int("status", 200).Msg("request handled")
		}
	})
	b.Run("zerolog", func(b *testing.B) {
		l := newZerolog()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Str("method", "GET").Str("path", "/api/orders").Int("status", 200).Msg("request handled")
		}
	})
	b.Run("zap", func(b *testing.B) {
		l := newZap()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info("request handled", zap.String("method", "GET"), zap.String("path", "/api/orders"), zap.Int("status", 200))
		}
	})
}

// ---------------------------------------------------------------------
// 3. Ten mixed-type fields.
// ---------------------------------------------------------------------

func BenchmarkTenFields(b *testing.B) {
	b.Run("practice", func(b *testing.B) {
		l := newPractice()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().
				Str("f1", "a").Str("f2", "b").
				Int("f3", 1).Int("f4", 2).
				Bool("f5", true).Bool("f6", false).
				Float64("f7", 1.5).Float64("f8", 2.5).
				Str("f9", "c").Int("f10", 3).
				Msg("event")
		}
	})
	b.Run("zerolog", func(b *testing.B) {
		l := newZerolog()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().
				Str("f1", "a").Str("f2", "b").
				Int("f3", 1).Int("f4", 2).
				Bool("f5", true).Bool("f6", false).
				Float64("f7", 1.5).Float64("f8", 2.5).
				Str("f9", "c").Int("f10", 3).
				Msg("event")
		}
	})
	b.Run("zap", func(b *testing.B) {
		l := newZap()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info("event",
				zap.String("f1", "a"), zap.String("f2", "b"),
				zap.Int("f3", 1), zap.Int("f4", 2),
				zap.Bool("f5", true), zap.Bool("f6", false),
				zap.Float64("f7", 1.5), zap.Float64("f8", 2.5),
				zap.String("f9", "c"), zap.Int("f10", 3),
			)
		}
	})
}

// ---------------------------------------------------------------------
// 4. Accumulated context — logger carries 3 preset fields, log with none.
// ---------------------------------------------------------------------

func BenchmarkAccumulatedContext(b *testing.B) {
	b.Run("practice", func(b *testing.B) {
		l := newPractice().With().Str("app", "svc").Str("env", "prod").Int("ver", 2).Logger()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Msg("tick")
		}
	})
	b.Run("zerolog", func(b *testing.B) {
		l := newZerolog().With().Str("app", "svc").Str("env", "prod").Int("ver", 2).Logger()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Msg("tick")
		}
	})
	b.Run("zap", func(b *testing.B) {
		l := newZap().With(zap.String("app", "svc"), zap.String("env", "prod"), zap.Int("ver", 2))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info("tick")
		}
	})
}

// ---------------------------------------------------------------------
// 5. Accumulated context + 3 call-time fields.
// ---------------------------------------------------------------------

func BenchmarkContextPlusFields(b *testing.B) {
	b.Run("practice", func(b *testing.B) {
		l := newPractice().With().Str("app", "svc").Str("env", "prod").Logger()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Str("method", "GET").Int("status", 200).Dur("took", durSample).Msg("handled")
		}
	})
	b.Run("zerolog", func(b *testing.B) {
		l := newZerolog().With().Str("app", "svc").Str("env", "prod").Logger()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Str("method", "GET").Int("status", 200).Dur("took", durSample).Msg("handled")
		}
	})
	b.Run("zap", func(b *testing.B) {
		l := newZap().With(zap.String("app", "svc"), zap.String("env", "prod"))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info("handled", zap.String("method", "GET"), zap.Int("status", 200), zap.Duration("took", durSample))
		}
	})
}

// ---------------------------------------------------------------------
// 6. Disabled level — no fields (cheapest reject path).
// ---------------------------------------------------------------------

func BenchmarkDisabledNoFields(b *testing.B) {
	b.Run("practice", func(b *testing.B) {
		l := newPractice(loggy.WithLevel(loggy.ErrorLevel))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Msg("dropped")
		}
	})
	b.Run("zerolog", func(b *testing.B) {
		l := zerolog.New(io.Discard).Level(zerolog.ErrorLevel).With().Timestamp().Logger()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Msg("dropped")
		}
	})
	b.Run("zap", func(b *testing.B) {
		enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
		l := zap.New(zapcore.NewCore(enc, zapcore.AddSync(io.Discard), zapcore.ErrorLevel))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info("dropped")
		}
	})
}

// ---------------------------------------------------------------------
// 7. Disabled level — with fields (call-site cost of a dropped call).
// ---------------------------------------------------------------------

func BenchmarkDisabledWithFields(b *testing.B) {
	b.Run("practice", func(b *testing.B) {
		l := newPractice(loggy.WithLevel(loggy.ErrorLevel))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Str("k", "v").Int("n", 1).Bool("ok", true).Msg("dropped")
		}
	})
	b.Run("zerolog", func(b *testing.B) {
		l := zerolog.New(io.Discard).Level(zerolog.ErrorLevel).With().Timestamp().Logger()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Str("k", "v").Int("n", 1).Bool("ok", true).Msg("dropped")
		}
	})
	b.Run("zap", func(b *testing.B) {
		enc := zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig())
		l := zap.New(zapcore.NewCore(enc, zapcore.AddSync(io.Discard), zapcore.ErrorLevel))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info("dropped", zap.String("k", "v"), zap.Int("n", 1), zap.Bool("ok", true))
		}
	})
}

// ---------------------------------------------------------------------
// 8. Caller enabled — cost of runtime.Caller per line.
// ---------------------------------------------------------------------

func BenchmarkWithCaller(b *testing.B) {
	b.Run("practice", func(b *testing.B) {
		l := newPractice(loggy.WithCaller(true))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Str("k", "v").Msg("with caller")
		}
	})
	b.Run("zerolog", func(b *testing.B) {
		l := zerolog.New(io.Discard).With().Timestamp().Caller().Logger()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Str("k", "v").Msg("with caller")
		}
	})
	b.Run("zap", func(b *testing.B) {
		l := newZap(zap.AddCaller())
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info("with caller", zap.String("k", "v"))
		}
	})
}

// ---------------------------------------------------------------------
// 9. Concurrent — shared logger hammered from many goroutines (pool + lock).
// ---------------------------------------------------------------------

func BenchmarkConcurrent(b *testing.B) {
	b.Run("practice", func(b *testing.B) {
		l := newPractice()
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				l.Info().Str("k", "v").Int("n", 1).Msg("m")
			}
		})
	})
	b.Run("practice_concurrentwriter", func(b *testing.B) {
		l := newPractice(loggy.WithConcurrentWriter())
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				l.Info().Str("k", "v").Int("n", 1).Msg("m")
			}
		})
	})
	b.Run("zerolog", func(b *testing.B) {
		l := newZerolog()
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				l.Info().Str("k", "v").Int("n", 1).Msg("m")
			}
		})
	})
	b.Run("zap", func(b *testing.B) {
		l := newZap()
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				l.Info("m", zap.String("k", "v"), zap.Int("n", 1))
			}
		})
	})
}

// ---------------------------------------------------------------------
// 10. Per-field-type cost (practice only) — isolate each encoder.
// ---------------------------------------------------------------------

func BenchmarkFieldTypes(b *testing.B) {
	l := newPractice()
	b.Run("Str", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Str("k", "value").Msg("m")
		}
	})
	b.Run("Int", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Int("k", 1234567).Msg("m")
		}
	})
	b.Run("Float64", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Float64("k", 3.14159).Msg("m")
		}
	})
	b.Run("Bool", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Bool("k", true).Msg("m")
		}
	})
	b.Run("Dur", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Dur("k", durSample).Msg("m")
		}
	})
	b.Run("Err", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Err(errSample).Msg("m")
		}
	})
	b.Run("Any_reflection", func(b *testing.B) {
		payload := struct {
			A int
			B string
		}{1, "x"}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Any("k", payload).Msg("m")
		}
	})
}

// ---------------------------------------------------------------------
// 11. Format cost (practice only) — JSON vs Text for the same payload.
// ---------------------------------------------------------------------

func BenchmarkFormat(b *testing.B) {
	fields := func(l loggy.Logger) {
		l.Info().Str("method", "GET").Int("status", 200).Dur("took", durSample).Msg("handled")
	}
	b.Run("json", func(b *testing.B) {
		l := loggy.New(loggy.WithOutput(io.Discard), loggy.WithFormat(loggy.JSONFormat))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			fields(l)
		}
	})
	b.Run("text", func(b *testing.B) {
		l := loggy.New(loggy.WithOutput(io.Discard), loggy.WithFormat(loggy.TextFormat))
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			fields(l)
		}
	})
}

// ---------------------------------------------------------------------
// 12. Formatted message (Msgf / Sugar) — printf-style cost.
// ---------------------------------------------------------------------

func BenchmarkFormattedMessage(b *testing.B) {
	b.Run("practice", func(b *testing.B) {
		l := newPractice()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Int("code", 42).Msgf("done in %dms", 7)
		}
	})
	b.Run("zerolog", func(b *testing.B) {
		l := newZerolog()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Info().Int("code", 42).Msgf("done in %dms", 7)
		}
	})
	b.Run("zap_sugar", func(b *testing.B) {
		l := newZap().Sugar()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Infof("done in %dms", 7)
		}
	})
}
