package loggy

import "sync/atomic"

// std holds the package-level default Logger. It is stored in an atomic.Pointer
// so SetDefault/Default are safe to call concurrently, and initialized on first
// use to avoid an init-order dependency.
var std atomic.Pointer[Logger]

// Default returns the package-level default Logger, creating a plain one on
// first access.
func Default() Logger {
	if p := std.Load(); p != nil {
		return *p
	}
	l := New()
	if std.CompareAndSwap(nil, &l) {
		return l
	}
	return *std.Load()
}

// SetDefault replaces the package-level default Logger.
func SetDefault(l Logger) {
	std.Store(&l)
}

// InfoPkg logs a message at InfoLevel through the default logger.
func InfoPkg(msg string) { Default().Info().Msg(msg) }

// ErrorPkg logs a message at ErrorLevel through the default logger.
func ErrorPkg(msg string) { Default().Error().Msg(msg) }
