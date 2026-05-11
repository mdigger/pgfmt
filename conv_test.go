package pgfmt_test

import (
	"math"
	"math/big"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgtype/zeronull"
	"github.com/mdigger/pgfmt"
)

func TestConfig_Format(t *testing.T) {
	// Фиксированное время в локальной зоне
	now := time.Date(2024, 5, 10, 15, 30, 0, 0, time.Local)

	cfg := pgfmt.Config{
		Null:             "NULL",
		BoolTrue:         "YES",
		BoolFalse:        "NO",
		DateOnlyFormat:   "02.01.2006",
		DateTimeFormat:   "2006-01-02 15:04",
		DecimalSeparator: ",",
		MaxStringLength:  10,
	}

	tests := []struct {
		name string
		val  any
		want string
	}{
		// --- Basic types & Pointers ---
		{"ptr_string", pointerTo("hello"), "hello"},
		{"ptr_int", pointerTo(int32(42)), "42"},
		{"ptr_nil", (*int)(nil), "NULL"},
		{"net_addr", netip.MustParseAddr("127.0.0.1"), "127.0.0.1"},

		// --- Numbers (exact values for float32) ---
		{"float32", float32(1.25), "1,25"},
		{"float64", 123.45, "123,45"},
		{"uint64", uint64(18446744073709551615), "18446744073709551615"},

		// --- Strings & Truncation (MaxStringLength: 10) ---
		{"string_normal", "hello", "hello"},
		{"string_trunc", "long string here", "long strin…"},
		{"string_utf8", "🍏🍎🍐", "🍏🍎🍐"},

		// --- pgtype: Valid values ---
		{"pg_int4_valid", pgtype.Int4{Int32: 4, Valid: true}, "4"},
		{"pg_float4_valid", pgtype.Float4{Float32: 1.5, Valid: true}, "1,5"},
		{"pg_uuid_valid", pgtype.UUID{Bytes: [16]byte{1}, Valid: true}, "01000000-0000-0000-0000-000000000000"},
		{"pg_timestamp_valid", pgtype.Timestamp{Time: now, Valid: true}, "2024-05-10 15:30"},
		{"pg_date_valid", pgtype.Date{Time: now, Valid: true}, "10.05.2024"},

		// --- pgtype: Errors, Infinity & Invalid (All should be NULL) ---
		{"pg_numeric_nan", pgtype.Numeric{NaN: true, Valid: true}, "NULL"},
		{"pg_tstz_inf", pgtype.Timestamptz{InfinityModifier: pgtype.Infinity, Valid: true}, "NULL"},
		{"pg_int2_inv", pgtype.Int2{Valid: false}, "NULL"},
		{"pg_text_inv", pgtype.Text{Valid: false}, "NULL"},
		{"pg_interval_inv", pgtype.Interval{Valid: false}, "NULL"},

		// --- zeronull (Zero/Empty -> NULL) ---
		{"zn_int8_zero", zeronull.Int8(0), "NULL"},
		{"zn_int8_val", zeronull.Int8(8), "8"},
		{"zn_float8_val", zeronull.Float8(12.34), "12,34"},
		{"zn_uuid_empty", zeronull.UUID([16]byte{}), "NULL"},
		{"zn_uuid_val", zeronull.UUID([16]byte{1}), "01000000-0000-0000-0000-000000000000"},
		{"zn_tstz_zero", zeronull.Timestamptz{}, "NULL"},

		// --- System & Raw types ---
		{"raw_uuid", [16]byte{1}, "01000000-0000-0000-0000-000000000000"},
		{"time_std", now, now.Format("2006-01-02 15:04")},

		// --- Basic types (Numbers: negative values and boundaries) ---
		{"int8_neg", int8(-128), "-128"},
		{"int16_neg", int16(-32768), "-32768"},
		{"int64_neg", int64(-9223372036854775808), "-9223372036854775808"},
		{"uint8_max", uint8(255), "255"},
		{"uint16_max", uint16(65535), "65535"},
		{"uint32_max", uint32(4294967295), "4294967295"},
		{"float64_nan_std", math.NaN(), "NULL"},
		{"float64_inf_std", math.Inf(1), "NULL"},

		// --- Nil & Bytes ---
		{"nil_raw", nil, "NULL"},
		{"bytes_valid", []byte("hello"), "hello"},
		{"bytes_invalid", []byte{0xFF, 0xFE, 0xFD}, "\\xfffefd"}, // Check for invalid UTF-8

		// --- pgtype (Numbers & Booleans) ---
		{"pg_bool_true", pgtype.Bool{Bool: true, Valid: true}, "YES"},
		{"pg_bool_false", pgtype.Bool{Bool: false, Valid: true}, "NO"},
		{"pg_int2_val", pgtype.Int2{Int16: -2, Valid: true}, "-2"},
		{"pg_int4_inv", pgtype.Int4{Valid: false}, "NULL"},
		{"pg_int8_val", pgtype.Int8{Int64: -8, Valid: true}, "-8"},
		{"pg_float4_inv", pgtype.Float4{Valid: false}, "NULL"},
		{"pg_float8_val", pgtype.Float8{Float64: -1.23, Valid: true}, "-1,23"},
		{"pg_numeric_val", pgtype.Numeric{Int: big.NewInt(100), Exp: -2, Valid: true}, "1,00"},

		// --- pgtype (Date & Time) ---
		{"pg_time", pgtype.Time{Microseconds: 0, Valid: true}, "00:00:00"},
		{"pg_timestamp_inv", pgtype.Timestamp{Valid: false}, "NULL"},
		{"pg_timestamp_inf", pgtype.Timestamp{InfinityModifier: pgtype.Infinity, Valid: true}, "NULL"},
		{"pg_tstz_val", pgtype.Timestamptz{Time: now, Valid: true}, now.Format("2006-01-02 15:04")},
		{"pg_date_inv", pgtype.Date{Valid: false}, "NULL"},
		{"pg_date_inf", pgtype.Date{InfinityModifier: pgtype.Infinity, Valid: true}, "NULL"},
		{"pg_interval", pgtype.Interval{Microseconds: 1000000, Valid: true}, "00:00:01"},

		// --- pgtype (Misc) ---
		{"pg_text_val", pgtype.Text{String: "hello", Valid: true}, "hello"},
		{"pg_uuid_inv", pgtype.UUID{Valid: false}, "NULL"},
		{"pg_bits", pgtype.Bits{Bytes: []byte{0xAA}, Len: 8, Valid: true}, "10101010"},

		// --- Network types ---
		{"net_prefix", netip.MustParsePrefix("192.168.1.0/24"), "192.168.1.0/24"},

		// --- zeronull ---
		{"zn_int2_zero", zeronull.Int2(0), "NULL"},
		{"zn_int2_val", zeronull.Int2(2), "2"},
		{"zn_int4_val", zeronull.Int4(4), "4"},
		{"zn_float8_zero", zeronull.Float8(0), "NULL"},
		{"zn_text_empty", zeronull.Text(""), "NULL"},
		{"zn_text_val", zeronull.Text("hi"), "hi"},
		{"zn_ts_zero", zeronull.Timestamp{}, "NULL"},

		// --- pgtype: More Invalid states ---
		{"pg_bool_inv", pgtype.Bool{Valid: false}, "NULL"},
		{"pg_float8_inv", pgtype.Float8{Valid: false}, "NULL"},
		{"pg_int8_inv", pgtype.Int8{Valid: false}, "NULL"},
		{"pg_time_inv", pgtype.Time{Valid: false}, "NULL"},
		{"pg_bits_inv", pgtype.Bits{Valid: false}, "NULL"},

		// --- zeronull: Zero values ---
		{"zn_int4_zero", zeronull.Int4(0), "NULL"},

		// --- Interval: Negative and Complex ---
		// -1 hour, -30 minutes
		{"pg_interval_neg", pgtype.Interval{Microseconds: -5400000000, Valid: true}, "-01:30:00"},
		// 1 month (formatted as interval string by driver)
		{"pg_interval_month", pgtype.Interval{Months: 1, Valid: true}, "1 mon 00:00:00"},

		// --- Numeric: Edge cases ---
		// Very large numeric that might fail internal parsing/formatting
		{"pg_numeric_huge", pgtype.Numeric{Int: big.NewInt(1), Exp: 100, Valid: true}, "1" + strings.Repeat("0", 100)},

		// --- Special numeric and intervals ---
		{"pg_interval_complex", pgtype.Interval{Months: 1, Days: 5, Microseconds: 1000000, Valid: true}, "1 mon 120:00:01"},

		// --- Complex and unusual types (No OnFallback) ---
		{"complex64", complex64(1 + 2i), "(1+2i)"},
		{"complex128", complex128(3 + 4i), "(3+4i)"},
		{"map_type", map[string]any{"key": 1}, "{\"key\":1}"},
		{"slice_type", []any{1, "two"}, "[1,\"two\"]"},
		{"array_type", [2]any{3, 4}, "[3,4]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.Format(tt.val); got != tt.want {
				t.Errorf("Format() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfig_DefaultFormats(t *testing.T) {
	cfg := pgfmt.Config{} // All formats are empty
	now := time.Date(2024, 5, 10, 15, 30, 0, 0, time.Local)

	tests := []struct {
		name string
		val  any
		want string
	}{
		{"bool_default", false, "FALSE"},
		{"float_no_sep", 1.23, "1.23"},
		{"datetime_default", now, now.Format(time.DateTime)},
		{"date_default", pgtype.Date{Time: now, Valid: true}, now.Format("2006-01-02")},
		{"numeric_no_sep", pgtype.Numeric{Int: big.NewInt(123), Exp: -2, Valid: true}, "1.23"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cfg.Format(tt.val); got != tt.want {
				t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestConfig_FallbackAndUnknowns(t *testing.T) {
	t.Run("OnFallback returns nil", func(t *testing.T) {
		cfg := pgfmt.Config{
			OnFallback: func(v any) *string {
				return nil
			},
		}
		val := map[string]int{"test": 1}
		if res := cfg.Format(val); res != "" {
			t.Errorf("expected standard format when OnFallback returns nil, got %q", res)
		}
	})

	t.Run("Inventory of unknown types", func(t *testing.T) {
		var cfg pgfmt.Config

		unknowns := cfg.GetUnknownTypes()
		if len(unknowns) > 0 {
			t.Errorf("must be empty. Found: %v", unknowns)
		}

		cfg.Format(make(chan int))
		cfg.Format(func(int) string { return "" })
		cfg.Format(complex(1, 1))

		unknowns = cfg.GetUnknownTypes()

		expected := []string{"chan int", "func(int) string", "complex128"}

		if len(unknowns) < len(expected) {
			t.Errorf("expected at least %d unknown types, got %d", len(expected), len(unknowns))
		}

		foundCount := 0
		for _, u := range unknowns {
			for _, exp := range expected {
				if u == exp {
					foundCount++
					break
				}
			}
		}

		if foundCount != len(expected) {
			t.Errorf("not all unknown types were registered. Found: %v", unknowns)
		}
	})
}

func TestConfig_Internal(t *testing.T) {
	t.Run("OnFallback", func(t *testing.T) {
		val := "fallback"
		c := pgfmt.Config{OnFallback: func(_ any) *string { return &val }}
		if res := c.Format(make(chan int)); res != val {
			t.Errorf("expected %s, got %s", val, res)
		}
	})

	t.Run("Default_Bool", func(t *testing.T) {
		c := pgfmt.Config{}
		if res := c.Format(true); res != "TRUE" {
			t.Errorf("expected TRUE, got %s", res)
		}
	})
}

func pointerTo[T any](v T) *T { return &v }
