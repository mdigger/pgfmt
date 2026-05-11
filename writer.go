package pgfmt

import (
	"github.com/jackc/pgx/v5"
)

// RowWriter defines the interface for writing a single row of strings.
// This interface is satisfied by *csv.Writer from the standard [encoding/csv] package.
type RowWriter interface {
	Write(record []string) error
}

// RowStreamer is a function type that streams [pgx.Rows] directly into a [RowWriter].
type RowStreamer func(w RowWriter, rows pgx.Rows) error

// NewRowStreamer creates a specialized function for streaming database results.
// It uses a formatter function (e.g., [Config.Format]) to stringify each column.
func NewRowStreamer(formatter func(any) string, customTypes map[uint32]any) RowStreamer {
	// Initialize the internal row handler with the provided conversion logic.
	handle := NewHandler(func(values []any, dst []string) {
		for i, v := range values {
			dst[i] = formatter(v)
		}
	}, customTypes)

	// Return a closure that connects the RowWriter to the database stream.
	return func(w RowWriter, rows pgx.Rows) error {
		return handle(rows, func(row []string) error {
			return w.Write(row)
		})
	}
}

// defaultStreamer is a pre-initialized worker using the [DefaultConfig].
var defaultStreamer = NewRowStreamer(DefaultConfig.Format, nil)

// Write streams database rows into a [RowWriter] using default settings.
func Write(w RowWriter, rows pgx.Rows) error {
	return defaultStreamer(w, rows)
}
