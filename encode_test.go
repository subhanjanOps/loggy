package loggy

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------
// JSON string escaping
// ---------------------------------------------------------------------

func TestAppendJSONString(t *testing.T) {
	cases := map[string]string{
		"plain": `"plain"`,
		`a"b`:   `"a\"b"`,
		`a\b`:   `"a\\b"`,
		"a\nb":  `"a\nb"`,
		"a\rb":  `"a\rb"`,
		"a\tb":  `"a\tb"`,
		"héllo": `"héllo"`, // multi-byte UTF-8 passes through
	}
	for in, want := range cases {
		got := string(appendJSONString(nil, in))
		if got != want {
			t.Errorf("appendJSONString(%q) = %s, want %s", in, got, want)
		}
		if !json.Valid([]byte(got)) {
			t.Errorf("appendJSONString(%q) produced invalid JSON: %s", in, got)
		}
	}
}

func TestAppendJSONStringControlChar(t *testing.T) {
	// A control byte (0x01) must become a \uXXXX escape, kept as valid JSON that
	// round-trips back to the original string. Built at runtime to avoid a
	// literal control byte in the source.
	in := string([]byte{'a', 0x01, 'b'})
	got := string(appendJSONString(nil, in))
	if !json.Valid([]byte(got)) {
		t.Fatalf("control char produced invalid JSON: %q", got)
	}
	var back string
	if err := json.Unmarshal([]byte(got), &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back != in {
		t.Errorf("round-trip mismatch: got %q, want %q", back, in)
	}
}

// ---------------------------------------------------------------------
// Float encoding incl. NaN / Inf
// ---------------------------------------------------------------------

func TestAppendJSONFloat(t *testing.T) {
	cases := map[float64]string{
		1.5:          "1.5",
		math.NaN():   `"NaN"`,
		math.Inf(1):  `"+Inf"`,
		math.Inf(-1): `"-Inf"`,
	}
	for in, want := range cases {
		if got := string(appendJSONFloat(nil, in)); got != want {
			t.Errorf("appendJSONFloat(%v) = %s, want %s", in, got, want)
		}
	}
}

// ---------------------------------------------------------------------
// appendTextValue / appendJSONValue — the Any fallback paths.
// ---------------------------------------------------------------------

func TestAppendTextValue(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, "<nil>"},
		{"s", "s"},
		{true, "true"},
		{7, "7"},
		{int64(9), "9"},
		{1.5, "1.5"},
		{2 * time.Second, "2s"},
		{errString("boom"), "boom"},
		{error(nil), "<nil>"},
		{stringerT{"x"}, "x"},
		{[]int{1, 2}, "[1 2]"}, // fmt fallback
	}
	for _, c := range cases {
		if got := string(appendTextValue(nil, c.in)); got != c.want {
			t.Errorf("appendTextValue(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAppendJSONValue(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{nil, "null"},
		{"s", `"s"`},
		{true, "true"},
		{7, "7"},
		{int64(9), "9"},
		{1.5, "1.5"},
		{2 * time.Second, "2000000000"},
		{errString("boom"), `"boom"`},
		{error(nil), "null"},
		{stringerT{"x"}, `"x"`},
		{map[string]int{"a": 1}, `{"a":1}`}, // json.Marshal fallback
	}
	for _, c := range cases {
		if got := string(appendJSONValue(nil, c.in)); got != c.want {
			t.Errorf("appendJSONValue(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAppendJSONValueMarshalError(t *testing.T) {
	// A channel is not JSON-encodable → the error branch quotes the %v form.
	ch := make(chan int)
	got := string(appendJSONValue(nil, ch))
	if !json.Valid([]byte(got)) {
		t.Errorf("marshal-error fallback is not valid JSON: %s", got)
	}
}

// errString is a minimal error for the value tests.
type errString string

func (e errString) Error() string { return string(e) }

// ---------------------------------------------------------------------
// appendLine — JSON & text, with/without name, caller, stack.
// ---------------------------------------------------------------------

func TestAppendLineJSON(t *testing.T) {
	e := entry{
		time:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		level:   InfoLevel,
		message: "hi",
		caller:  "f.go:1",
		stack:   "goroutine 1",
	}
	persistent := fragStr(nil, JSONFormat, "app", "svc")
	streamed := fragInt(nil, JSONFormat, "n", 3)
	line := string(appendLine(nil, JSONFormat, "svc", false, e, persistent, streamed))

	if !json.Valid([]byte(strings.TrimRight(line, "\n"))) {
		t.Fatalf("invalid JSON: %s", line)
	}
	for _, want := range []string{`"logger":"svc"`, `"msg":"hi"`, `"app":"svc"`, `"n":3`, `"caller":"f.go:1"`, `"stack":"goroutine 1"`} {
		if !strings.Contains(line, want) {
			t.Errorf("line %s missing %s", line, want)
		}
	}
}

func TestAppendLineTextNoNameNoColor(t *testing.T) {
	e := entry{
		time:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		level:   WarnLevel,
		message: "hi",
	}
	line := string(appendLine(nil, TextFormat, "", false, e, nil, nil))
	if !strings.HasPrefix(line, "2026-01-02T03:04:05Z WARN hi") {
		t.Errorf("unexpected text line: %q", line)
	}
	if strings.Contains(line, "[") {
		t.Errorf("no name should mean no brackets: %q", line)
	}
}

func TestAppendLineTextColorAndStackNewline(t *testing.T) {
	e := entry{
		time:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		level:   ErrorLevel,
		message: "hi",
		caller:  "f.go:7",
		stack:   "trace-without-newline",
	}
	line := string(appendLine(nil, TextFormat, "svc", true, e, nil, nil))
	if !strings.Contains(line, colorRed) || !strings.Contains(line, colorReset) {
		t.Errorf("expected color codes: %q", line)
	}
	if !strings.Contains(line, "(f.go:7)") {
		t.Errorf("expected caller in text output: %q", line)
	}
	if !strings.HasSuffix(line, "\n") {
		t.Errorf("stack without trailing newline should get one: %q", line)
	}
}

func TestAppendLineTextStackAlreadyNewline(t *testing.T) {
	e := entry{
		time:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		level:   ErrorLevel,
		message: "hi",
		stack:   "trace-with-newline\n",
	}
	line := string(appendLine(nil, TextFormat, "", false, e, nil, nil))
	if strings.HasSuffix(line, "\n\n") {
		t.Errorf("stack already ending in newline should not be doubled: %q", line)
	}
}

// ---------------------------------------------------------------------
// Level helpers
// ---------------------------------------------------------------------

func TestLevelStringAndUpper(t *testing.T) {
	cases := []struct {
		l           Level
		lower, uppr string
	}{
		{DebugLevel, "debug", "DEBUG"},
		{InfoLevel, "info", "INFO"},
		{WarnLevel, "warn", "WARN"},
		{ErrorLevel, "error", "ERROR"},
		{FatalLevel, "fatal", "FATAL"},
		{Level(99), "unknown", "UNKNOWN"},
	}
	for _, c := range cases {
		if c.l.String() != c.lower {
			t.Errorf("Level(%d).String() = %q, want %q", c.l, c.l.String(), c.lower)
		}
		if c.l.upper() != c.uppr {
			t.Errorf("Level(%d).upper() = %q, want %q", c.l, c.l.upper(), c.uppr)
		}
	}
}

func TestLevelColor(t *testing.T) {
	cases := map[Level]string{
		DebugLevel: colorWhite,
		InfoLevel:  colorGreen,
		WarnLevel:  colorYellow,
		ErrorLevel: colorRed,
		FatalLevel: colorBoldRed,
		Level(99):  colorReset,
	}
	for l, want := range cases {
		if got := levelColor(l); got != want {
			t.Errorf("levelColor(%d) = %q, want %q", l, got, want)
		}
	}
}

// ---------------------------------------------------------------------
// fragErr both formats, nil and non-nil.
// ---------------------------------------------------------------------

func TestFragErr(t *testing.T) {
	if got := string(fragErr(nil, JSONFormat, nil)); got != `,"error":null` {
		t.Errorf("json nil err = %q", got)
	}
	if got := string(fragErr(nil, TextFormat, nil)); got != " error=<nil>" {
		t.Errorf("text nil err = %q", got)
	}
	if got := string(fragErr(nil, JSONFormat, errString("x"))); got != `,"error":"x"` {
		t.Errorf("json err = %q", got)
	}
	if got := string(fragErr(nil, TextFormat, errString("x"))); got != " error=x" {
		t.Errorf("text err = %q", got)
	}
}
