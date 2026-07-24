package loggy

import "context"

// ctxKey is the unexported, collision-free key under which a Logger is stored
// in a context.Context. A struct{} key means no map is involved — context uses
// a single linked value node.
type ctxKey struct{}

// WithContext returns a copy of ctx carrying l, retrievable via FromContext.
func WithContext(ctx context.Context, l Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromContext returns the Logger stored in ctx, or the package default logger
// if none is present.
func FromContext(ctx context.Context) Logger {
	if ctx != nil {
		if l, ok := ctx.Value(ctxKey{}).(Logger); ok && l != nil {
			return l
		}
	}
	return Default()
}
