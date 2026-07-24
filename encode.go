package loggy

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"
)

// maxPooledBuffer caps the capacity of buffers returned to the pool so an
// occasional huge line (e.g. a big stack trace) doesn't pin large allocations.
const maxPooledBuffer = 64 << 10

// bufPool recycles the byte buffers used to render each line, so the steady
// state is zero heap allocations for the buffer itself.
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 512)
		return &b
	},
}

// ANSI color codes used to colorize the level label in text output.
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorWhite   = "\033[37m"
	colorBoldRed = "\033[1;31m"
)

// levelColor maps a level to its ANSI color: white=debug, green=info,
// yellow=warn, red=error, bold-red=fatal.
func levelColor(l Level) string {
	switch l {
	case DebugLevel:
		return colorWhite
	case InfoLevel:
		return colorGreen
	case WarnLevel:
		return colorYellow
	case ErrorLevel:
		return colorRed
	case FatalLevel:
		return colorBoldRed
	default:
		return colorReset
	}
}

// Small strconv wrappers, shared by the fragment encoders below.
func appendInt(dst []byte, n int64) []byte     { return strconv.AppendInt(dst, n, 10) }
func appendBool(dst []byte, b bool) []byte     { return strconv.AppendBool(dst, b) }
func appendFloat(dst []byte, f float64) []byte { return strconv.AppendFloat(dst, f, 'g', -1, 64) }

// ---------------------------------------------------------------------
// Field fragments — append one "key=value" (text) / ,"key":value (JSON) chunk.
// Shared by *Event (streamed fields) and *Context (persistent fields), so the
// value-encoding logic lives in exactly one place with no per-kind switch.
// ---------------------------------------------------------------------

func fragKey(buf []byte, f Format, key string) []byte {
	if f == JSONFormat {
		buf = append(buf, ',')
		buf = appendJSONString(buf, key)
		return append(buf, ':')
	}
	buf = append(buf, ' ')
	buf = append(buf, key...)
	return append(buf, '=')
}

func fragStr(buf []byte, f Format, key, val string) []byte {
	buf = fragKey(buf, f, key)
	if f == JSONFormat {
		return appendJSONString(buf, val)
	}
	return append(buf, val...)
}

func fragInt(buf []byte, f Format, key string, val int64) []byte {
	return appendInt(fragKey(buf, f, key), val)
}

func fragFloat(buf []byte, f Format, key string, val float64) []byte {
	buf = fragKey(buf, f, key)
	if f == JSONFormat {
		return appendJSONFloat(buf, val)
	}
	return appendFloat(buf, val)
}

func fragBool(buf []byte, f Format, key string, val bool) []byte {
	return appendBool(fragKey(buf, f, key), val)
}

func fragDur(buf []byte, f Format, key string, d time.Duration) []byte {
	buf = fragKey(buf, f, key)
	if f == JSONFormat {
		return appendInt(buf, int64(d)) // nanoseconds
	}
	return append(buf, d.String()...)
}

// fragErr always uses the conventional "error" key.
func fragErr(buf []byte, f Format, err error) []byte {
	buf = fragKey(buf, f, "error")
	if err == nil {
		if f == JSONFormat {
			return append(buf, "null"...)
		}
		return append(buf, "<nil>"...)
	}
	if f == JSONFormat {
		return appendJSONString(buf, err.Error())
	}
	return append(buf, err.Error()...)
}

func fragStringer(buf []byte, f Format, key string, s fmt.Stringer) []byte {
	buf = fragKey(buf, f, key)
	if f == JSONFormat {
		return appendJSONString(buf, s.String())
	}
	return append(buf, s.String()...)
}

func fragAny(buf []byte, f Format, key string, val any) []byte {
	buf = fragKey(buf, f, key)
	if f == JSONFormat {
		return appendJSONValue(buf, val)
	}
	return appendTextValue(buf, val)
}

// ---------------------------------------------------------------------
// Line assembly
// ---------------------------------------------------------------------

// appendLine renders a full line (with trailing newline). Fields arrive already
// encoded: persistent are the logger's inherited fields, streamed are the ones
// added on this event. No map, no reflection, no intermediate string.
func appendLine(buf []byte, format Format, name string, color bool, e entry, persistent, streamed []byte) []byte {
	if format == JSONFormat {
		buf = append(buf, '{')
		buf = appendJSONKey(buf, "time")
		buf = append(buf, '"')
		buf = e.time.AppendFormat(buf, time.RFC3339Nano)
		buf = append(buf, '"')
		buf = append(buf, ',')
		buf = appendJSONKey(buf, "level")
		buf = appendJSONString(buf, e.level.String())
		if name != "" {
			buf = append(buf, ',')
			buf = appendJSONKey(buf, "logger")
			buf = appendJSONString(buf, name)
		}
		buf = append(buf, ',')
		buf = appendJSONKey(buf, "msg")
		buf = appendJSONString(buf, e.message)
		buf = append(buf, persistent...)
		buf = append(buf, streamed...)
		if e.caller != "" {
			buf = append(buf, ',')
			buf = appendJSONKey(buf, "caller")
			buf = appendJSONString(buf, e.caller)
		}
		if e.stack != "" {
			buf = append(buf, ',')
			buf = appendJSONKey(buf, "stack")
			buf = appendJSONString(buf, e.stack)
		}
		return append(buf, '}', '\n')
	}

	buf = e.time.AppendFormat(buf, time.RFC3339Nano)
	buf = append(buf, ' ')
	if color {
		buf = append(buf, levelColor(e.level)...)
		buf = append(buf, e.level.upper()...)
		buf = append(buf, colorReset...)
	} else {
		buf = append(buf, e.level.upper()...)
	}
	if name != "" {
		buf = append(buf, " ["...)
		buf = append(buf, name...)
		buf = append(buf, ']')
	}
	buf = append(buf, ' ')
	buf = append(buf, e.message...)
	buf = append(buf, persistent...)
	buf = append(buf, streamed...)
	if e.caller != "" {
		buf = append(buf, " ("...)
		buf = append(buf, e.caller...)
		buf = append(buf, ')')
	}
	buf = append(buf, '\n')
	if e.stack != "" {
		buf = append(buf, e.stack...)
		if e.stack[len(e.stack)-1] != '\n' {
			buf = append(buf, '\n')
		}
	}
	return buf
}

// ---------------------------------------------------------------------
// Value encoders used by fragAny for arbitrary values.
// ---------------------------------------------------------------------

// appendTextValue writes a compact, unquoted text value; common types are
// handled without reflection, anything else falls back to fmt.
func appendTextValue(buf []byte, v any) []byte {
	switch val := v.(type) {
	case nil:
		return append(buf, "<nil>"...)
	case string:
		return append(buf, val...)
	case bool:
		return appendBool(buf, val)
	case int:
		return appendInt(buf, int64(val))
	case int64:
		return appendInt(buf, val)
	case float64:
		return appendFloat(buf, val)
	case time.Duration:
		return append(buf, val.String()...)
	case error:
		// A nil error interface boxed in any hits `case nil` above, so val is
		// non-nil here.
		return append(buf, val.Error()...)
	case fmt.Stringer:
		return append(buf, val.String()...)
	default:
		return fmt.Appendf(buf, "%v", val)
	}
}

// appendJSONValue writes an arbitrary value as JSON; common types are encoded
// directly, only slices/maps/structs hit json.Marshal.
func appendJSONValue(buf []byte, v any) []byte {
	switch val := v.(type) {
	case nil:
		return append(buf, "null"...)
	case string:
		return appendJSONString(buf, val)
	case bool:
		return appendBool(buf, val)
	case int:
		return appendInt(buf, int64(val))
	case int64:
		return appendInt(buf, val)
	case float64:
		return appendJSONFloat(buf, val)
	case time.Duration:
		return appendInt(buf, int64(val))
	case error:
		// A nil error interface boxed in any hits `case nil` above, so val is
		// non-nil here.
		return appendJSONString(buf, val.Error())
	case fmt.Stringer:
		return appendJSONString(buf, val.String())
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return appendJSONString(buf, fmt.Sprintf("%v", v))
		}
		return append(buf, data...)
	}
}

// appendJSONKey appends a quoted key followed by a colon.
func appendJSONKey(buf []byte, key string) []byte {
	buf = appendJSONString(buf, key)
	return append(buf, ':')
}

// appendJSONFloat encodes a float, rendering the non-JSON values NaN/±Inf as
// strings instead of producing invalid JSON.
func appendJSONFloat(buf []byte, f float64) []byte {
	switch {
	case math.IsNaN(f):
		return append(buf, `"NaN"`...)
	case math.IsInf(f, 1):
		return append(buf, `"+Inf"`...)
	case math.IsInf(f, -1):
		return append(buf, `"-Inf"`...)
	}
	return appendFloat(buf, f)
}

const hexDigits = "0123456789abcdef"

// appendJSONString appends s as a quoted, escaped JSON string. It escapes only
// what JSON requires (", \, and control characters < 0x20), passing multi-byte
// UTF-8 through unchanged — correct JSON and far cheaper than json.Marshal.
func appendJSONString(buf []byte, s string) []byte {
	buf = append(buf, '"')
	start := 0
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b >= 0x20 && b != '"' && b != '\\' {
			continue
		}
		buf = append(buf, s[start:i]...)
		switch b {
		case '"':
			buf = append(buf, '\\', '"')
		case '\\':
			buf = append(buf, '\\', '\\')
		case '\n':
			buf = append(buf, '\\', 'n')
		case '\r':
			buf = append(buf, '\\', 'r')
		case '\t':
			buf = append(buf, '\\', 't')
		default:
			buf = append(buf, '\\', 'u', '0', '0', hexDigits[b>>4], hexDigits[b&0xf])
		}
		start = i + 1
	}
	buf = append(buf, s[start:]...)
	return append(buf, '"')
}
