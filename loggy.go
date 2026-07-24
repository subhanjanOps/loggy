package loggy

import (
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Logger is a structured logger. All methods are safe for concurrent use. The
// level methods begin a chained entry finished by Msg/Msgf.
type Logger interface {
	Debug() *Event
	Info() *Event
	Warn() *Event
	Error() *Event
	Fatal() *Event // Msg/Msgf then os.Exit(1)
	Panic() *Event // Msg/Msgf then panic(msg)

	// With begins a child-logger builder; finish it with Logger():
	//   child := l.With().Str("component", "db").Logger()
	With() *Context

	SetLevel(lvl Level)
	Level() Level
	Enabled(lvl Level) bool // cheap pre-check for hot paths
	Sync() error            // flush the underlying writer if it supports it
	Close() error           // close the underlying writer if it is an io.Closer
}

// logger is the unexported concrete implementation of Logger.
type logger struct {
	out          io.Writer
	level        atomic.Int32
	format       Format
	fields       []byte // pre-encoded persistent field fragments
	caller       bool
	stackTrace   Level
	timeFunc     func() time.Time
	hooks        []Hook
	sampler      Sampler
	name         string
	ctxExtractor ContextExtractor
	color        colorMode
	lockFree     bool        // skip mu when the writer is concurrency-safe
	mu           *sync.Mutex // guards writes to out; shared with children
}

// New builds a Logger with sane defaults (stdout, InfoLevel, JSON, time.Now)
// and applies the given options in order.
func New(opts ...Option) Logger {
	l := &logger{
		out:        os.Stdout,
		format:     JSONFormat,
		stackTrace: FatalLevel,
		timeFunc:   time.Now,
		mu:         &sync.Mutex{},
	}
	l.level.Store(int32(InfoLevel))
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// ---------------------------------------------------------------------
// Level entry points — each begins a chained Event.
// ---------------------------------------------------------------------

func (l *logger) Debug() *Event { return l.newEvent(DebugLevel, termNone) }
func (l *logger) Info() *Event  { return l.newEvent(InfoLevel, termNone) }
func (l *logger) Warn() *Event  { return l.newEvent(WarnLevel, termNone) }
func (l *logger) Error() *Event { return l.newEvent(ErrorLevel, termNone) }
func (l *logger) Fatal() *Event { return l.newEvent(FatalLevel, termExit) }
func (l *logger) Panic() *Event { return l.newEvent(FatalLevel, termPanic) }

// emit runs the sampler and hooks, then assembles the final line (persistent +
// streamed fields) into a pooled buffer and writes it.
func (l *logger) emit(e entry, streamed []byte) {
	if l.sampler != nil && !l.sampler.Allow(e) {
		return
	}
	for _, h := range l.hooks {
		_ = h.Fire(e)
	}

	bufp := bufPool.Get().(*[]byte)
	buf := appendLine((*bufp)[:0], l.format, l.name, l.useColor(), e, l.fields, streamed)

	// Each line is a single Write. When the caller has declared the writer
	// concurrency-safe (WithConcurrentWriter), skip the mutex entirely and let
	// the writer serialize — this is what gives lock-free parallel throughput.
	if l.lockFree {
		_, _ = l.out.Write(buf)
	} else {
		l.mu.Lock()
		_, _ = l.out.Write(buf)
		l.mu.Unlock()
	}

	if cap(buf) <= maxPooledBuffer {
		*bufp = buf
		bufPool.Put(bufp)
	}
}

// ---------------------------------------------------------------------
// Level & lifecycle control
// ---------------------------------------------------------------------

func (l *logger) SetLevel(lvl Level)     { l.level.Store(int32(lvl)) }
func (l *logger) Level() Level           { return Level(l.level.Load()) }
func (l *logger) Enabled(lvl Level) bool { return lvl >= l.Level() }

func (l *logger) Sync() error {
	if s, ok := l.out.(interface{ Sync() error }); ok {
		return s.Sync()
	}
	return nil
}

func (l *logger) Close() error {
	if c, ok := l.out.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// useColor reports whether text output should be colorized: always/never when
// forced via WithColor, otherwise auto — true only when out is a terminal.
func (l *logger) useColor() bool {
	switch l.color {
	case colorAlways:
		return true
	case colorNever:
		return false
	default:
		return isTerminal(l.out)
	}
}

// isTerminal reports whether w is a character device (a TTY/console), the
// classic isatty check — works cross-platform without external dependencies.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// ---------------------------------------------------------------------
// Caller / stack helpers
// ---------------------------------------------------------------------

// callerInfo returns "file:line" for the frame `skip` levels above this helper.
func callerInfo(skip int) string {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return ""
	}
	return shortFile(file) + ":" + strconv.Itoa(line)
}

// stackTrace captures the current goroutine's stack.
func stackTrace() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// shortFile trims a path to its final element, handling both separators.
func shortFile(file string) string {
	for i := len(file) - 1; i >= 0; i-- {
		if file[i] == '/' || file[i] == '\\' {
			return file[i+1:]
		}
	}
	return file
}
