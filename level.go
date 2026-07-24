package loggy

// Level is the severity of a log entry. Higher values are more severe.
type Level int32

// The available severity levels, in increasing order.
const (
	DebugLevel Level = iota // verbose diagnostic detail
	InfoLevel               // normal operational messages
	WarnLevel               // something unexpected but non-fatal
	ErrorLevel              // a failure that needs attention
	FatalLevel              // terminal failure (Fatal exits, Panic panics)
)

// String returns the lowercase name of the level, used by the encoders.
func (l Level) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	default:
		return "unknown"
	}
}

// upper returns the uppercase level name for text output, allocation-free.
func (l Level) upper() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Format selects how entries are rendered on the wire.
type Format int

// The available output formats.
const (
	TextFormat Format = iota // human-readable line, colorized on a terminal
	JSONFormat               // one JSON object per line
)
