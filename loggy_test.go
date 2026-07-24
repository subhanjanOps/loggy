package loggy

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------
// Shared test helpers
// ---------------------------------------------------------------------

// fixedTime pins the clock so output is deterministic.
func fixedTime() Option {
	t := time.Date(2026, 7, 24, 10, 30, 0, 0, time.UTC)
	return WithTimeFunc(func() time.Time { return t })
}

// decode parses one JSON log line into a map for assertions.
func decode(t *testing.T, line string) map[string]any {
	t.Helper()
	if !json.Valid([]byte(line)) {
		t.Fatalf("output is not valid JSON: %q", line)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return m
}

// newBuf returns a JSON logger writing to buf with a fixed clock.
func newBuf(buf *bytes.Buffer, opts ...Option) Logger {
	return New(append([]Option{WithOutput(buf), WithFormat(JSONFormat), fixedTime()}, opts...)...)
}

// ---------------------------------------------------------------------
// Construction & defaults
// ---------------------------------------------------------------------

func TestNewDefaults(t *testing.T) {
	var buf bytes.Buffer
	// Only override the writer; everything else is default.
	l := New(WithOutput(&buf))

	l.Debug().Msg("dropped") // default level is Info → dropped
	if buf.Len() != 0 {
		t.Errorf("default level should drop Debug, got %q", buf.String())
	}
	l.Info().Msg("kept")
	if !json.Valid(buf.Bytes()) { // default format is JSON
		t.Errorf("default format should be JSON, got %q", buf.String())
	}
	if l.Level() != InfoLevel {
		t.Errorf("default level = %v, want info", l.Level())
	}
}

func TestOptions(t *testing.T) {
	var buf bytes.Buffer
	l := New(
		WithOutput(&buf),
		WithLevel(DebugLevel),
		WithFormat(JSONFormat),
		WithName("svc"),
		fixedTime(),
	)
	l.Debug().Msg("m")
	m := decode(t, buf.String())
	if m["logger"] != "svc" || m["level"] != "debug" {
		t.Errorf("options not applied: %v", m)
	}
}

// ---------------------------------------------------------------------
// Levels & filtering
// ---------------------------------------------------------------------

func TestAllLevelsRender(t *testing.T) {
	cases := []struct {
		start func(Logger) *Event
		want  string
	}{
		{func(l Logger) *Event { return l.Debug() }, "debug"},
		{func(l Logger) *Event { return l.Info() }, "info"},
		{func(l Logger) *Event { return l.Warn() }, "warn"},
		{func(l Logger) *Event { return l.Error() }, "error"},
	}
	for _, c := range cases {
		var buf bytes.Buffer
		l := newBuf(&buf, WithLevel(DebugLevel))
		c.start(l).Msg("m")
		if m := decode(t, buf.String()); m["level"] != c.want {
			t.Errorf("level = %v, want %v", m["level"], c.want)
		}
	}
}

func TestFilteringAndEnabled(t *testing.T) {
	var buf bytes.Buffer
	l := newBuf(&buf, WithLevel(WarnLevel))

	l.Info().Msg("dropped")
	if buf.Len() != 0 {
		t.Errorf("Info dropped at Warn, got %q", buf.String())
	}
	if l.Enabled(InfoLevel) {
		t.Error("Enabled(Info) should be false at Warn")
	}
	if !l.Enabled(ErrorLevel) {
		t.Error("Enabled(Error) should be true at Warn")
	}
	l.Error().Msg("kept")
	if !strings.Contains(buf.String(), "kept") {
		t.Error("Error should pass at Warn")
	}
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	l := newBuf(&buf, WithLevel(ErrorLevel))
	l.Info().Msg("no")
	if buf.Len() != 0 {
		t.Fatal("expected Info suppressed")
	}
	l.SetLevel(DebugLevel)
	l.Info().Msg("yes")
	if !strings.Contains(buf.String(), "yes") {
		t.Error("expected Info after SetLevel(Debug)")
	}
	if l.Level() != DebugLevel {
		t.Errorf("Level() = %v, want debug", l.Level())
	}
}

// ---------------------------------------------------------------------
// Sampler & hooks
// ---------------------------------------------------------------------

type countingSampler struct {
	remaining int
	seen      int
}

func (s *countingSampler) Allow(Entry) bool {
	s.seen++
	if s.remaining > 0 {
		s.remaining--
		return true
	}
	return false
}

func TestSampler(t *testing.T) {
	var buf bytes.Buffer
	s := &countingSampler{remaining: 1}
	l := newBuf(&buf, WithSampler(s))
	l.Info().Msg("first")
	l.Info().Msg("second")

	if strings.Count(buf.String(), "\n") != 1 {
		t.Errorf("sampler should allow 1 line, got %q", buf.String())
	}
	if s.seen != 2 {
		t.Errorf("sampler consulted %d times, want 2", s.seen)
	}
}

type recordingHook struct{ entries []Entry }

func (h *recordingHook) Fire(e Entry) error {
	h.entries = append(h.entries, e)
	return nil
}

func TestHookSeesEntryMetadata(t *testing.T) {
	var buf bytes.Buffer
	h := &recordingHook{}
	l := New(WithOutput(&buf), WithFormat(JSONFormat), WithHook(h),
		WithCaller(true), WithStackTrace(ErrorLevel), fixedTime())

	l.Error().Str("k", "v").Msg("boom")
	if len(h.entries) != 1 {
		t.Fatalf("hook saw %d entries, want 1", len(h.entries))
	}
	e := h.entries[0]
	if e.Message() != "boom" || e.Level() != ErrorLevel {
		t.Errorf("entry = %q/%v", e.Message(), e.Level())
	}
	if e.Caller() == "" {
		t.Error("expected caller on entry")
	}
	if e.Stack() == "" {
		t.Error("expected stack on entry at Error")
	}
	if e.Time().IsZero() {
		t.Error("expected non-zero time")
	}
}

// ---------------------------------------------------------------------
// Caller & stack
// ---------------------------------------------------------------------

func TestCallerAndStack(t *testing.T) {
	var buf bytes.Buffer
	l := New(WithOutput(&buf), WithFormat(JSONFormat),
		WithCaller(true), WithStackTrace(InfoLevel), fixedTime())
	l.Info().Msg("m")

	m := decode(t, buf.String())
	caller, _ := m["caller"].(string)
	if !strings.Contains(caller, "loggy_test.go:") {
		t.Errorf("caller = %q, want loggy_test.go:N", caller)
	}
	if stack, _ := m["stack"].(string); !strings.Contains(stack, "goroutine") {
		t.Errorf("stack missing: %q", stack)
	}
}

// ---------------------------------------------------------------------
// Lifecycle: Sync / Close
// ---------------------------------------------------------------------

type syncWriter struct {
	bytes.Buffer
	synced bool
}

func (w *syncWriter) Sync() error { w.synced = true; return nil }

type closeWriter struct {
	bytes.Buffer
	closed bool
}

func (w *closeWriter) Close() error { w.closed = true; return nil }

func TestSyncAndClose(t *testing.T) {
	sw := &syncWriter{}
	l := New(WithOutput(sw))
	if err := l.Sync(); err != nil || !sw.synced {
		t.Errorf("Sync: err=%v synced=%v", err, sw.synced)
	}

	cw := &closeWriter{}
	l2 := New(WithOutput(cw))
	if err := l2.Close(); err != nil || !cw.closed {
		t.Errorf("Close: err=%v closed=%v", err, cw.closed)
	}

	// A plain writer supports neither: both return nil without panicking.
	var buf bytes.Buffer
	l3 := New(WithOutput(&buf))
	if l3.Sync() != nil || l3.Close() != nil {
		t.Error("plain writer Sync/Close should be nil")
	}
}

// ---------------------------------------------------------------------
// Big line exercises the pool-drop branch (cap > maxPooledBuffer).
// ---------------------------------------------------------------------

func TestHugeLineDoesNotCorrupt(t *testing.T) {
	var buf bytes.Buffer
	l := newBuf(&buf)
	big := strings.Repeat("x", maxPooledBuffer+1024)
	l.Info().Str("big", big).Msg("m")

	m := decode(t, buf.String())
	if m["big"] != big {
		t.Error("huge field round-trip failed")
	}
}

// ---------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------

type safeCollector struct {
	mu    sync.Mutex
	lines [][]byte
}

func (c *safeCollector) Write(p []byte) (int, error) {
	c.mu.Lock()
	c.lines = append(c.lines, append([]byte(nil), p...))
	c.mu.Unlock()
	return len(p), nil
}

func TestConcurrentWriterNoCorruption(t *testing.T) {
	const goroutines, perG = 16, 200
	c := &safeCollector{}
	l := New(WithOutput(c), WithFormat(JSONFormat), WithConcurrentWriter(), fixedTime())

	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < perG; i++ {
				l.Info().Int("g", id).Int("i", i).Msg("concurrent")
			}
		}(g)
	}
	wg.Wait()

	if len(c.lines) != goroutines*perG {
		t.Fatalf("got %d lines, want %d", len(c.lines), goroutines*perG)
	}
	for _, line := range c.lines {
		if !json.Valid(line) {
			t.Fatalf("corrupted line: %q", line)
		}
	}
}

func TestDefaultMutexPathConcurrent(t *testing.T) {
	// Same as above but without WithConcurrentWriter — exercises the locked path.
	c := &safeCollector{}
	l := New(WithOutput(c), WithFormat(JSONFormat), fixedTime())
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				l.Info().Int("i", i).Msg("x")
			}
		}()
	}
	wg.Wait()
	if len(c.lines) != 800 {
		t.Fatalf("got %d lines, want 800", len(c.lines))
	}
}

// ---------------------------------------------------------------------
// Fatal & Panic (exit injected)
// ---------------------------------------------------------------------

func TestFatalExits(t *testing.T) {
	var buf bytes.Buffer
	var code = -1
	prev := exit
	exit = func(c int) { code = c }
	defer func() { exit = prev }()

	l := newBuf(&buf)
	l.Fatal().Str("reason", "boom").Msg("dying")

	if code != 1 {
		t.Errorf("Fatal exit code = %d, want 1", code)
	}
	if m := decode(t, buf.String()); m["msg"] != "dying" || m["reason"] != "boom" {
		t.Errorf("Fatal did not log: %v", m)
	}
}

func TestFatalExitsEvenWhenLevelHigh(t *testing.T) {
	// Fatal must terminate regardless of level; FatalLevel is the max so it is
	// always enabled, but this guards the "term != none" bypass in newEvent.
	var buf bytes.Buffer
	var called bool
	prev := exit
	exit = func(int) { called = true }
	defer func() { exit = prev }()

	l := newBuf(&buf, WithLevel(FatalLevel))
	l.Fatal().Msg("bye")
	if !called {
		t.Error("Fatal did not call exit")
	}
}

func TestPanicPanics(t *testing.T) {
	var buf bytes.Buffer
	l := newBuf(&buf)
	defer func() {
		r := recover()
		if r != "boom" {
			t.Errorf("recover = %v, want boom", r)
		}
		if !strings.Contains(buf.String(), "boom") {
			t.Error("Panic did not log before panicking")
		}
	}()
	l.Panic().Msg("boom")
}

// ---------------------------------------------------------------------
// Unexported helpers
// ---------------------------------------------------------------------

func TestShortFile(t *testing.T) {
	cases := map[string]string{
		"/a/b/c.go":     "c.go",
		`C:\x\y\z.go`:   "z.go",
		"nodir.go":      "nodir.go",
		"mixed/a\\b.go": "b.go",
	}
	for in, want := range cases {
		if got := shortFile(in); got != want {
			t.Errorf("shortFile(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCallerInfoOutOfRange(t *testing.T) {
	if got := callerInfo(9999); got != "" {
		t.Errorf("callerInfo(9999) = %q, want empty", got)
	}
}

func TestIsTerminal(t *testing.T) {
	if isTerminal(&bytes.Buffer{}) {
		t.Error("bytes.Buffer is not a terminal")
	}
	if isTerminal(io.Discard) {
		t.Error("io.Discard is not a terminal")
	}
	// A regular *os.File is not a character device.
	f, err := os.CreateTemp(t.TempDir(), "loggy-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if isTerminal(f) {
		t.Error("a regular file is not a terminal")
	}
}

func TestUseColorModes(t *testing.T) {
	always := New(WithOutput(&bytes.Buffer{}), WithColor(true)).(*logger)
	if !always.useColor() {
		t.Error("WithColor(true) should force color")
	}
	never := New(WithOutput(&bytes.Buffer{}), WithColor(false)).(*logger)
	if never.useColor() {
		t.Error("WithColor(false) should disable color")
	}
	auto := New(WithOutput(&bytes.Buffer{})).(*logger) // auto → non-terminal → false
	if auto.useColor() {
		t.Error("auto color to a buffer should be false")
	}
}

func TestErrorFieldValue(t *testing.T) {
	var buf bytes.Buffer
	l := newBuf(&buf)
	l.Error().Err(errors.New("disk full")).Msg("failed")
	if m := decode(t, buf.String()); m["error"] != "disk full" {
		t.Errorf("error = %v", m["error"])
	}
}
