package pgfmt

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net/netip"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgtype/zeronull"
)

// Fallback defines a function called when the converter doesn't recognize a type.
// If it returns nil, the result defaults to [Config.Null].
type Fallback func(v any) *string

// Config defines the formatting rules for data-to-string conversion.
type Config struct {
	Null             string       // String representation for nil/null values
	BoolTrue         string       // String representation for true
	BoolFalse        string       // String representation for false
	DateOnlyFormat   string       // Format for dates (e.g., "2006-01-02")
	DateTimeFormat   string       // Format for timestamps
	TimeOnlyFormat   string       // Format for time values
	DecimalSeparator string       // Character used as a decimal point in floating-point numbers
	MaxStringLength  int          // Max length before truncation with ellipsis
	OnFallback       Fallback     // Custom handler for unknown types
	Logger           *slog.Logger // Optional structured logger
	// unknownTypes stores encountered types that have no explicit handling.
	unknownTypes atomic.Pointer[sync.Map]
	_            noCopy
}

// Format converts any supported value to a string based on its type and configuration.
func (cfg *Config) Format(val any) string { //nolint:gocognit,gocyclo,cyclop,funlen,maintidx
	if val == nil {
		return cfg.Null
	}

	// Dereference pointers if necessary.
	reflectValue := reflect.ValueOf(val)
	if reflectValue.Kind() == reflect.Pointer {
		if reflectValue.IsNil() {
			return cfg.Null
		}

		val = reflectValue.Elem().Interface()
	}

	switch val := val.(type) {
	case string:
		return cfg.String(val)
	case []byte:
		var str string
		if utf8.Valid(val) {
			str = string(val)
		} else {
			str = "\\x" + hex.EncodeToString(val)
		}

		return cfg.String(str)
	case [16]byte:
		return UUID(val)

	// explicit cases for primitive types to avoid reflect.Value overhead.
	case bool:
		return cfg.Bool(val)
	case int8:
		return cfg.Int(int64(val))
	case int16:
		return cfg.Int(int64(val))
	case int32:
		return cfg.Int(int64(val))
	case int64:
		return cfg.Int(val)
	case uint8:
		return cfg.Uint(uint64(val))
	case uint16:
		return cfg.Uint(uint64(val))
	case uint32:
		return cfg.Uint(uint64(val))
	case uint64:
		return cfg.Uint(val)
	case float32:
		return cfg.Float(float64(val))
	case float64:
		return cfg.Float(val)

	case time.Time:
		return cfg.DateTime(val)
	case pgtype.Time:
		return cfg.Time(val)
	case pgtype.Date:
		return cfg.Date(val)
	case pgtype.Numeric:
		return cfg.Numeric(val)
	case pgtype.Interval:
		return cfg.Interval(val)
	case pgtype.Bits:
		return cfg.Bits(val)

	// pgtype specialized types must be handled separately for field access.
	case pgtype.Bool:
		if !val.Valid {
			return cfg.Null
		}

		return cfg.Bool(val.Bool)
	case pgtype.Float4:
		if !val.Valid {
			return cfg.Null
		}

		return cfg.Float(float64(val.Float32))
	case pgtype.Float8:
		if !val.Valid {
			return cfg.Null
		}

		return cfg.Float(val.Float64)
	case pgtype.Int2:
		if !val.Valid {
			return cfg.Null
		}

		return cfg.Int(int64(val.Int16))
	case pgtype.Int4:
		if !val.Valid {
			return cfg.Null
		}

		return cfg.Int(int64(val.Int32))
	case pgtype.Int8:
		if !val.Valid {
			return cfg.Null
		}

		return cfg.Int(val.Int64)
	case pgtype.Text:
		if !val.Valid {
			return cfg.Null
		}

		return cfg.String(val.String)
	case pgtype.UUID:
		if !val.Valid {
			return cfg.Null
		}

		return UUID(val.Bytes)
	case pgtype.Timestamp:
		if !val.Valid || val.InfinityModifier != pgtype.Finite {
			return cfg.Null
		}

		return cfg.DateTime(val.Time)
	case pgtype.Timestamptz:
		if !val.Valid || val.InfinityModifier != pgtype.Finite {
			return cfg.Null
		}

		return cfg.DateTime(val.Time)

	case netip.Addr:
		return val.String()
	case netip.Prefix:
		return val.String()

	// zeronull types.
	case zeronull.Int2:
		if val == 0 {
			return cfg.Null
		}

		return cfg.Int(int64(val))
	case zeronull.Int4:
		if val == 0 {
			return cfg.Null
		}

		return cfg.Int(int64(val))
	case zeronull.Int8:
		if val == 0 {
			return cfg.Null
		}

		return cfg.Int(int64(val))
	case zeronull.Float8:
		if val == 0 {
			return cfg.Null
		}

		return cfg.Float(float64(val))
	case zeronull.Text:
		if val == "" {
			return cfg.Null
		}

		return cfg.String(string(val))
	case zeronull.Timestamp:
		return cfg.DateTime(time.Time(val))
	case zeronull.Timestamptz:
		return cfg.DateTime(time.Time(val))
	case zeronull.UUID:
		if val == [16]byte{} {
			return cfg.Null
		}

		return UUID([16]byte(val))

	default:
		if cfg.OnFallback != nil {
			res := cfg.OnFallback(val)
			if res != nil {
				return *res
			}

			return cfg.Null
		}

		return cfg.fallback(val)
	}
}

// String checks max allowed string length [Config.MaxStringLength] and truncates if exceeded.
func (cfg *Config) String(val string) string {
	if cfg.MaxStringLength <= 0 || len(val) <= cfg.MaxStringLength {
		return val
	}

	var count int
	for i := range val {
		if count >= cfg.MaxStringLength {
			return val[:i] + "\u2026"
		}

		count++
	}

	return val
}

// Bool returns a string representation of a boolean.
func (cfg *Config) Bool(val bool) string {
	if cfg.BoolTrue != cfg.BoolFalse {
		if val {
			return cfg.BoolTrue
		}

		return cfg.BoolFalse
	}

	if val {
		return "TRUE"
	}

	return "FALSE"
}

// Int returns a decimal string from int64.
func (cfg *Config) Int(val int64) string {
	return strconv.FormatInt(val, 10)
}

// Uint returns a decimal string from uint64.
func (cfg *Config) Uint(val uint64) string {
	return strconv.FormatUint(val, 10)
}

// Float formats a float (32 or 64 bit). Returns [Config.Null] for NaN/Inf.
// It handles both types via float64 but keeps precision consistent.
func (cfg *Config) Float(val float64) string {
	if math.IsNaN(val) || math.IsInf(val, 0) {
		return cfg.Null
	}

	str := strconv.FormatFloat(val, 'g', -1, 64)

	if cfg.DecimalSeparator != "" && cfg.DecimalSeparator != "." {
		return strings.Replace(str, ".", cfg.DecimalSeparator, 1)
	}
	return str
}

// DateTime formats [time.Time] using [Config.DateTimeFormat] or [time.DateTime].
func (cfg *Config) DateTime(val time.Time) string {
	if val.IsZero() {
		return cfg.Null
	}

	layout := cfg.DateTimeFormat
	if layout == "" {
		layout = time.DateTime
	}

	return val.Local().Format(layout) //nolint:gosmopolitan
}

// Time formats [pgtype.Time] using [Config.TimeOnlyFormat] or [time.TimeOnly].
func (cfg *Config) Time(val pgtype.Time) string {
	if !val.Valid {
		return cfg.Null
	}

	timeVal := time.Unix(0, val.Microseconds*1000).UTC()

	layout := cfg.TimeOnlyFormat
	if layout == "" {
		layout = time.TimeOnly
	}

	return timeVal.Format(layout)
}

// Date formats [pgtype.Date] using [Config.DateOnlyFormat] or [time.DateOnly].
func (cfg *Config) Date(val pgtype.Date) string {
	if !val.Valid || val.InfinityModifier != pgtype.Finite {
		return cfg.Null
	}

	layout := cfg.DateOnlyFormat
	if layout == "" {
		layout = time.DateOnly
	}

	return val.Time.Format(layout)
}

// Numeric extracts a string from [pgtype.Numeric].
func (cfg *Config) Numeric(val pgtype.Numeric) string {
	if !val.Valid || val.InfinityModifier != pgtype.Finite || val.NaN {
		return cfg.Null
	}

	d, _ := val.Value()
	if str, ok := d.(string); ok {
		if cfg.DecimalSeparator != "" && cfg.DecimalSeparator != "." {
			return strings.Replace(str, ".", cfg.DecimalSeparator, 1)
		}

		return str
	}

	return cfg.Null
}

// Interval formats [pgtype.Interval] as "HH:MM:SS" (plus months if present).
func (cfg *Config) Interval(val pgtype.Interval) string {
	if !val.Valid {
		return cfg.Null
	}

	totalMicro := val.Microseconds + (int64(val.Days) * 24 * 3600 * 1000000)
	absMicro := totalMicro

	sign := ""
	if totalMicro < 0 {
		sign = "-"
		absMicro = -totalMicro
	}

	totalSeconds := absMicro / 1000000
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	buf := make([]byte, 0, 32)
	if val.Months != 0 {
		buf = strconv.AppendInt(buf, int64(val.Months), 10)
		buf = append(buf, " mon "...)
	}

	if sign == "-" && (hours > 0 || minutes > 0 || seconds > 0) {
		buf = append(buf, '-')
	}

	appendPad := func(b []byte, v int64) []byte {
		if v < 10 {
			b = append(b, '0')
		}

		return strconv.AppendInt(b, v, 10)
	}
	buf = appendPad(buf, hours)
	buf = append(buf, ':')
	buf = appendPad(buf, minutes)
	buf = append(buf, ':')
	buf = appendPad(buf, seconds)

	return string(buf)
}

// Bits converts [pgtype.Bits] to a string of '0' and '1'.
func (cfg *Config) Bits(val pgtype.Bits) string {
	if !val.Valid {
		return cfg.Null
	}

	buf := make([]byte, 0, val.Len)
	for i := range val.Len {
		byteIdx := i / 8

		bitIdx := 7 - (i % 8)
		if (val.Bytes[byteIdx] >> uint(bitIdx) & 1) == 1 {
			buf = append(buf, '1')
		} else {
			buf = append(buf, '0')
		}
	}

	return string(buf)
}

// GetUnknownTypes returns a list of type names encountered during conversion.
func (cfg *Config) GetUnknownTypes() []string {
	unknownTypes := cfg.unknownTypes.Load()
	if unknownTypes == nil {
		return nil
	}

	var list []string

	unknownTypes.Range(func(key, _ any) bool {
		if s, ok := key.(string); ok {
			list = append(list, s)
		}

		return true
	})

	return list
}

// fallback handles complex or unknown types. If a [Config.Logger] is set, it logs
// a warning for the first occurrence of each unique unrecognized type.
func (cfg *Config) fallback(val any) string {
	t := reflect.TypeOf(val)
	typeName := t.String()

	// Lazy initialization only happens here, when we actually have something to record.
	unknowns := cfg.unknownTypes.Load()
	if unknowns == nil {
		newMap := new(sync.Map)
		if cfg.unknownTypes.CompareAndSwap(nil, newMap) {
			unknowns = newMap
		} else {
			unknowns = cfg.unknownTypes.Load()
		}
	}

	// Register the type and check if it's the first time we see it.
	if _, loaded := unknowns.LoadOrStore(typeName, struct{}{}); !loaded {
		if cfg.Logger != nil {
			//nolint:sloglint
			cfg.Logger.Warn("pgfmt: unrecognized type encountered, using default formatting",
				slog.String("type", typeName),
				slog.String("value_example", fmt.Sprintf("%.20v", val)),
			)
		}
	}

	// Default fallback logic (JSON for collections, fmt.Sprint for others).
	rv := reflect.ValueOf(val)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array || rv.Kind() == reflect.Map {
		b, err := json.Marshal(val)
		if err == nil {
			return string(b)
		}
	}

	return fmt.Sprint(val)
}

// DefaultConfig provides standard conversion settings.
var DefaultConfig Config

// Format converts a value using the global [DefaultConfig].
func Format(val any) string {
	return DefaultConfig.Format(val)
}

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
