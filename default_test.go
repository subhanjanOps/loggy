package loggy

import (
	"bytes"
	"context"
	"testing"
)

func TestDefaultIsUsable(t *testing.T) {
	if Default() == nil {
		t.Fatal("Default() returned nil")
	}
	// Idempotent: repeated calls return the same instance until SetDefault.
	if Default() != Default() {
		t.Error("Default() not stable")
	}
}

func TestSetDefaultAndPkgFuncs(t *testing.T) {
	var buf bytes.Buffer
	custom := New(WithOutput(&buf), WithFormat(JSONFormat), fixedTime())
	prev := Default()
	SetDefault(custom)
	defer SetDefault(prev)

	InfoPkg("info-pkg")
	if m := decode(t, buf.String()); m["msg"] != "info-pkg" || m["level"] != "info" {
		t.Errorf("InfoPkg: %v", m)
	}
	buf.Reset()
	ErrorPkg("error-pkg")
	if m := decode(t, buf.String()); m["msg"] != "error-pkg" || m["level"] != "error" {
		t.Errorf("ErrorPkg: %v", m)
	}
}

func TestWithContextRoundTrip(t *testing.T) {
	l := New()
	ctx := WithContext(context.Background(), l)
	if got := FromContext(ctx); got != l {
		t.Error("FromContext did not return the stored logger")
	}
}

func TestFromContextAbsentReturnsDefault(t *testing.T) {
	if FromContext(context.Background()) != Default() {
		t.Error("absent logger should return Default()")
	}
}

func TestFromContextNilCtx(t *testing.T) {
	// A nil context must not panic; it returns the default logger. Use a typed
	// nil variable so linters don't flag a literal nil argument.
	var ctx context.Context
	if FromContext(ctx) != Default() {
		t.Error("nil ctx should return Default()")
	}
}
