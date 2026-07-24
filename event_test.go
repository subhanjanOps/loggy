package loggy

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"
)

// stringerT implements fmt.Stringer for the Stringer field tests.
type stringerT struct{ v string }

func (s stringerT) String() string { return s.v }

// ---------------------------------------------------------------------
// Every Event field method, JSON.
// ---------------------------------------------------------------------

func TestEventFieldTypesJSON(t *testing.T) {
	var buf bytes.Buffer
	l := newBuf(&buf)
	l.Info().
		Str("s", "v").
		Int("i", 7).
		Int64("i64", 9).
		Float64("f", 1.5).
		Bool("b", true).
		Dur("d", 2*time.Second).
		Err(nil).
		Stringer("sg", stringerT{"x"}).
		Any("a", []int{1, 2}).
		Msg("m")

	m := decode(t, buf.String())
	checks := map[string]any{
		"s": "v", "i": float64(7), "i64": float64(9), "f": 1.5,
		"b": true, "d": float64(2 * time.Second), "sg": "x",
	}
	for k, want := range checks {
		if m[k] != want {
			t.Errorf("field %q = %v, want %v", k, m[k], want)
		}
	}
	if m["error"] != nil { // Err(nil) → null
		t.Errorf("Err(nil) = %v, want null", m["error"])
	}
	if a, ok := m["a"].([]any); !ok || len(a) != 2 {
		t.Errorf("Any slice = %v", m["a"])
	}
}

// ---------------------------------------------------------------------
// Every Event field method, Text.
// ---------------------------------------------------------------------

func TestEventFieldTypesText(t *testing.T) {
	var buf bytes.Buffer
	l := New(WithOutput(&buf), WithFormat(TextFormat), fixedTime())
	l.Info().
		Str("s", "v").
		Int("i", 7).
		Float64("f", 1.5).
		Bool("b", false).
		Dur("d", 250*time.Millisecond).
		Err(nil).
		Stringer("sg", stringerT{"x"}).
		Any("a", 42).
		Msg("hello")

	out := buf.String()
	for _, want := range []string{"INFO", "hello", "s=v", "i=7", "f=1.5", "b=false", "d=250ms", "error=<nil>", "sg=x", "a=42"} {
		if !strings.Contains(out, want) {
			t.Errorf("text %q missing %q", out, want)
		}
	}
}

func TestEventErrNonNil(t *testing.T) {
	var buf bytes.Buffer
	l := New(WithOutput(&buf), WithFormat(TextFormat), fixedTime())
	l.Error().Err(errNamed).Msg("m")
	if !strings.Contains(buf.String(), "error=named") {
		t.Errorf("got %q", buf.String())
	}
}

var errNamed = &namedError{}

type namedError struct{}

func (*namedError) Error() string { return "named" }

// ---------------------------------------------------------------------
// Disabled event: every method is a no-op, nothing is written.
// ---------------------------------------------------------------------

func TestDisabledEventNoOp(t *testing.T) {
	var buf bytes.Buffer
	l := newBuf(&buf, WithLevel(ErrorLevel))
	l.Info().
		Str("s", "v").Int("i", 1).Int64("i64", 2).Float64("f", 1.5).
		Bool("b", true).Dur("d", time.Second).Err(errNamed).
		Stringer("sg", stringerT{"x"}).Any("a", 1).
		Ctx(context.Background()).
		Msg("dropped")
	if buf.Len() != 0 {
		t.Errorf("disabled event wrote %q", buf.String())
	}
}

func TestDisabledEventMsgf(t *testing.T) {
	var buf bytes.Buffer
	l := newBuf(&buf, WithLevel(ErrorLevel))
	l.Info().Int("n", 1).Msgf("x=%d", 5)
	if buf.Len() != 0 {
		t.Errorf("disabled Msgf wrote %q", buf.String())
	}
}

// ---------------------------------------------------------------------
// Msgf
// ---------------------------------------------------------------------

func TestMsgf(t *testing.T) {
	var buf bytes.Buffer
	l := newBuf(&buf)
	l.Info().Int("code", 42).Msgf("done in %dms", 7)
	m := decode(t, buf.String())
	if m["msg"] != "done in 7ms" || m["code"] != float64(42) {
		t.Errorf("Msgf wrong: %v", m)
	}
}

// ---------------------------------------------------------------------
// Ctx extractor
// ---------------------------------------------------------------------

type ctxKeyT string

func TestEventCtx(t *testing.T) {
	var buf bytes.Buffer
	const key ctxKeyT = "trace"
	ext := func(ctx context.Context, e *Event) {
		if v, ok := ctx.Value(key).(string); ok {
			e.Str("trace_id", v)
		}
	}
	l := New(WithOutput(&buf), WithFormat(JSONFormat), WithContextExtractor(ext), fixedTime())
	ctx := context.WithValue(context.Background(), key, "xyz")
	l.Info().Ctx(ctx).Str("path", "/").Msg("req")

	m := decode(t, buf.String())
	if m["trace_id"] != "xyz" || m["path"] != "/" {
		t.Errorf("ctx: %v", m)
	}
}

func TestEventCtxNilExtractor(t *testing.T) {
	var buf bytes.Buffer
	l := newBuf(&buf) // no extractor
	l.Info().Ctx(context.Background()).Str("k", "v").Msg("plain")
	m := decode(t, buf.String())
	if m["msg"] != "plain" || m["k"] != "v" {
		t.Errorf("nil extractor changed behavior: %v", m)
	}
}

// ---------------------------------------------------------------------
// Context builder (With) — every type, immutability, nesting.
// ---------------------------------------------------------------------

func TestWithFieldTypes(t *testing.T) {
	var buf bytes.Buffer
	l := newBuf(&buf).With().
		Str("s", "v").
		Int("i", 1).
		Int64("i64", 2).
		Float64("f", 1.5).
		Bool("b", true).
		Dur("d", time.Second).
		Err(errNamed).
		Stringer("sg", stringerT{"x"}).
		Any("a", 3).
		Logger()
	l.Info().Msg("m")

	m := decode(t, buf.String())
	for k, want := range map[string]any{"s": "v", "i": float64(1), "b": true, "sg": "x", "error": "named"} {
		if m[k] != want {
			t.Errorf("persistent %q = %v, want %v", k, m[k], want)
		}
	}
}

func TestWithImmutable(t *testing.T) {
	var buf bytes.Buffer
	parent := newBuf(&buf)
	child := parent.With().Str("req", "abc").Logger()

	child.Info().Msg("c")
	if m := decode(t, buf.String()); m["req"] != "abc" {
		t.Errorf("child missing field: %v", m)
	}
	buf.Reset()
	parent.Info().Msg("p")
	if m := decode(t, buf.String()); m["req"] != nil {
		t.Errorf("parent mutated: %v", m)
	}
}

func TestWithNested(t *testing.T) {
	var buf bytes.Buffer
	l := newBuf(&buf).
		With().Str("a", "1").Logger().
		With().Str("b", "2").Logger()
	l.Info().Str("c", "3").Msg("m")

	m := decode(t, buf.String())
	if m["a"] != "1" || m["b"] != "2" || m["c"] != "3" {
		t.Errorf("nested With: %v", m)
	}
}

func TestWithInheritsConfig(t *testing.T) {
	var buf bytes.Buffer
	child := New(WithOutput(&buf), WithFormat(JSONFormat), WithName("svc"),
		WithLevel(WarnLevel), fixedTime()).
		With().Str("k", "v").Logger()

	child.Info().Msg("dropped") // inherits WarnLevel
	if buf.Len() != 0 {
		t.Errorf("child should inherit level, got %q", buf.String())
	}
	child.Warn().Msg("kept")
	if m := decode(t, buf.String()); m["logger"] != "svc" || m["k"] != "v" {
		t.Errorf("child config: %v", m)
	}
}
