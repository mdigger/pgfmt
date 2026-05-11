package pgfmt_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mdigger/pgfmt"
)

// mockRowWriter captures the output for verification.
type mockRowWriter struct {
	records [][]string
	err     error
}

func (m *mockRowWriter) Write(record []string) error {
	if m.err != nil {
		return m.err
	}
	// Crucial: copy the slice because NewHandler reuses it.
	row := make([]string, len(record))
	copy(row, record)
	m.records = append(m.records, row)
	return nil
}

func TestNewRowStreamer(t *testing.T) {
	// Simple formatter for testing.
	formatter := func(v any) string {
		if v == nil {
			return "NULL"
		}
		return strings.ToUpper(fmt.Sprint(v))
	}

	t.Run("successful stream with custom logic", func(t *testing.T) {
		writer := &mockRowWriter{}
		streamer := pgfmt.NewRowStreamer(formatter, nil)

		mock := &mockRows{
			fields: []pgconn.FieldDescription{
				{Name: "id", DataTypeOID: pgtype.Int4OID},
				{Name: "name", DataTypeOID: pgtype.TextOID},
			},
			data: [][]any{
				{int32(1), "alice"},
				{int32(2), "bob"},
			},
		}

		if err := streamer(writer, mock); err != nil {
			t.Fatalf("streamer failed: %v", err)
		}

		if len(writer.records) != 2 {
			t.Errorf("expected 2 records, got %d", len(writer.records))
		}
		if writer.records[0][1] != "ALICE" {
			t.Errorf("expected ALICE, got %q", writer.records[0][1])
		}
	})

	t.Run("propagation of writer errors", func(t *testing.T) {
		writeErr := errors.New("write failure")
		writer := &mockRowWriter{err: writeErr}
		streamer := pgfmt.NewRowStreamer(formatter, nil)

		mock := &mockRows{
			fields: []pgconn.FieldDescription{{DataTypeOID: pgtype.Int4OID}},
			data:   [][]any{{1}},
		}

		err := streamer(writer, mock)
		if !errors.Is(err, writeErr) {
			t.Errorf("expected error %v, got %v", writeErr, err)
		}
	})
}

func TestWrite(t *testing.T) {
	t.Run("global Write uses default settings", func(t *testing.T) {
		writer := &mockRowWriter{}
		mock := &mockRows{
			fields: []pgconn.FieldDescription{
				{Name: "val", DataTypeOID: pgtype.Float8OID},
			},
			data: [][]any{
				{123.45},
			},
		}

		// Uses defaultStreamer -> DefaultConfig (DecimalSeparator = ".")
		if err := pgfmt.Write(writer, mock); err != nil {
			t.Errorf("global Write failed: %v", err)
		}

		if writer.records[0][0] != "123.45" {
			t.Errorf("expected default formatting '123.45', got %q", writer.records[0][0])
		}
	})
}
