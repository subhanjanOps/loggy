package loggy

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

// termAction is what Msg/Msgf does after writing: nothing, exit, or panic.
type termAction uint8

const (
	termNone termAction = iota
	termExit
	termPanic
)

// Event is a chained, zero-allocation log entry. Fields are encoded directly
// into a pooled buffer as they are added; the terminal Msg/Msgf writes the line
// and recycles the Event. Begin one with a Logger level method (l.Info() etc).
//
// An Event must not be used after Msg/Msgf. When the level is disabled the
// level method returns a shared no-op Event whose methods do nothing.
type Event struct {
	l       *logger
	buf     []byte
	lvl     Level
	term    termAction
	enabled bool
}

// disabledEvent is the shared no-op returned for a disabled level. Its methods
// never touch buf or l, so sharing one pointer across goroutines is race-free.
var disabledEvent = &Event{}

// eventPool recycles enabled events so a log allocates nothing.
var eventPool = sync.Pool{
	New: func() any {
		return &Event{buf: make([]byte, 0, 256)}
	},
}

// newEvent returns an enabled, pooled Event, or the shared no-op when the level
// is filtered out. Fatal/Panic (term != none) are always enabled so they always
// terminate the process, even with logging turned down.
func (l *logger) newEvent(level Level, term termAction) *Event {
	if term == termNone && level < l.Level() {
		return disabledEvent
	}
	e := eventPool.Get().(*Event)
	e.l = l
	e.lvl = level
	e.term = term
	e.enabled = true
	e.buf = e.buf[:0]
	return e
}

// ---------------------------------------------------------------------
// Chainable field methods.
// ---------------------------------------------------------------------

// Str adds a string field.
func (e *Event) Str(key, val string) *Event {
	if e.enabled {
		e.buf = fragStr(e.buf, e.l.format, key, val)
	}
	return e
}

// Int adds an int field.
func (e *Event) Int(key string, val int) *Event {
	if e.enabled {
		e.buf = fragInt(e.buf, e.l.format, key, int64(val))
	}
	return e
}

// Int64 adds an int64 field.
func (e *Event) Int64(key string, val int64) *Event {
	if e.enabled {
		e.buf = fragInt(e.buf, e.l.format, key, val)
	}
	return e
}

// Float64 adds a float64 field. NaN and ±Inf are rendered as strings in JSON.
func (e *Event) Float64(key string, val float64) *Event {
	if e.enabled {
		e.buf = fragFloat(e.buf, e.l.format, key, val)
	}
	return e
}

// Bool adds a bool field.
func (e *Event) Bool(key string, val bool) *Event {
	if e.enabled {
		e.buf = fragBool(e.buf, e.l.format, key, val)
	}
	return e
}

// Dur adds a duration field: nanoseconds in JSON, a string like 250ms in text.
func (e *Event) Dur(key string, d time.Duration) *Event {
	if e.enabled {
		e.buf = fragDur(e.buf, e.l.format, key, d)
	}
	return e
}

// Err adds the error under the key "error" (null/<nil> when err is nil).
func (e *Event) Err(err error) *Event {
	if e.enabled {
		e.buf = fragErr(e.buf, e.l.format, err)
	}
	return e
}

// Stringer adds a field rendered via the value's String method.
func (e *Event) Stringer(key string, val fmt.Stringer) *Event {
	if e.enabled {
		e.buf = fragStringer(e.buf, e.l.format, key, val)
	}
	return e
}

// Any adds an arbitrary value, falling back to reflection (json.Marshal / fmt)
// for types without a dedicated method.
func (e *Event) Any(key string, val any) *Event {
	if e.enabled {
		e.buf = fragAny(e.buf, e.l.format, key, val)
	}
	return e
}

// Ctx runs the logger's ContextExtractor (if any) against ctx, letting it add
// fields such as a trace id to this event.
func (e *Event) Ctx(ctx context.Context) *Event {
	if e.enabled && e.l.ctxExtractor != nil {
		e.l.ctxExtractor(ctx, e)
	}
	return e
}

// ---------------------------------------------------------------------
// Terminal methods.
// ---------------------------------------------------------------------

// Msg writes the entry with the given message and finishes the event.
func (e *Event) Msg(msg string) { e.send(msg) }

// Msgf formats the message with fmt.Sprintf, then writes the entry.
func (e *Event) Msgf(format string, args ...any) { e.send(fmt.Sprintf(format, args...)) }

// send assembles and emits the line, recycles the event, then performs any
// terminal action (Fatal exit / Panic). callerInfo(3): user -> Msg/Msgf -> send.
func (e *Event) send(msg string) {
	if !e.enabled {
		return // shared no-op event
	}
	l := e.l
	en := entry{time: l.timeFunc(), level: e.lvl, message: msg}
	if l.caller {
		en.caller = callerInfo(3)
	}
	if e.lvl >= l.stackTrace {
		en.stack = stackTrace()
	}
	l.emit(en, e.buf)

	term := e.term
	e.l = nil
	eventPool.Put(e)

	switch term {
	case termExit:
		exit(1)
	case termPanic:
		panic(msg)
	}
}

// exit lives behind this indirection so tests can intercept Fatal without
// killing the test binary.
var exit = os.Exit

// ---------------------------------------------------------------------
// Context builder — With() accumulates persistent fields for a child logger.
// ---------------------------------------------------------------------

// Context builds a child logger carrying pre-encoded persistent fields. Finish
// with Logger(). Field methods mirror Event's.
type Context struct {
	l   *logger
	buf []byte
}

// With begins a child-logger builder, inheriting the receiver's persistent
// fields.
func (l *logger) With() *Context {
	c := &Context{l: l}
	if len(l.fields) > 0 {
		c.buf = append(c.buf, l.fields...)
	}
	return c
}

// Str adds a persistent string field to the child logger.
func (c *Context) Str(key, val string) *Context {
	c.buf = fragStr(c.buf, c.l.format, key, val)
	return c
}

// Int adds a persistent int field to the child logger.
func (c *Context) Int(key string, val int) *Context {
	c.buf = fragInt(c.buf, c.l.format, key, int64(val))
	return c
}

// Int64 adds a persistent int64 field to the child logger.
func (c *Context) Int64(key string, val int64) *Context {
	c.buf = fragInt(c.buf, c.l.format, key, val)
	return c
}

// Float64 adds a persistent float64 field to the child logger.
func (c *Context) Float64(key string, val float64) *Context {
	c.buf = fragFloat(c.buf, c.l.format, key, val)
	return c
}

// Bool adds a persistent bool field to the child logger.
func (c *Context) Bool(key string, val bool) *Context {
	c.buf = fragBool(c.buf, c.l.format, key, val)
	return c
}

// Dur adds a persistent duration field to the child logger.
func (c *Context) Dur(key string, d time.Duration) *Context {
	c.buf = fragDur(c.buf, c.l.format, key, d)
	return c
}

// Err adds a persistent error field (key "error") to the child logger.
func (c *Context) Err(err error) *Context {
	c.buf = fragErr(c.buf, c.l.format, err)
	return c
}

// Stringer adds a persistent Stringer field to the child logger.
func (c *Context) Stringer(key string, val fmt.Stringer) *Context {
	c.buf = fragStringer(c.buf, c.l.format, key, val)
	return c
}

// Any adds a persistent arbitrary-value field to the child logger.
func (c *Context) Any(key string, val any) *Context {
	c.buf = fragAny(c.buf, c.l.format, key, val)
	return c
}

// Logger returns a child Logger carrying the accumulated fields. It shares the
// parent's writer, hooks, sampler, and write lock, and does not mutate the
// parent.
func (c *Context) Logger() Logger {
	p := c.l
	child := &logger{
		out:          p.out,
		format:       p.format,
		fields:       append([]byte(nil), c.buf...),
		caller:       p.caller,
		stackTrace:   p.stackTrace,
		timeFunc:     p.timeFunc,
		hooks:        p.hooks,
		sampler:      p.sampler,
		name:         p.name,
		ctxExtractor: p.ctxExtractor,
		color:        p.color,
		lockFree:     p.lockFree,
		mu:           p.mu,
	}
	child.level.Store(p.level.Load())
	return child
}
