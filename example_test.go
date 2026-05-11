package pgfmt_test

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/mdigger/pgfmt"
)

// This example demonstrates how to use the high-level Write function
// for a quick and standard export using default settings.
func ExampleWrite() {
	// Assume rows is a valid pgx.Rows object from a query
	var rows pgx.Rows

	w := csv.NewWriter(os.Stdout)

	// Set global preferences if needed
	pgfmt.DefaultConfig.Null = "N/A"
	pgfmt.DefaultConfig.DateTimeFormat = time.Stamp

	// The destination writer comes first, followed by the database rows.
	err := pgfmt.Write(w, rows)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Export failed: %v\n", err)
	}

	w.Flush()
}

// This example shows how to create a custom RowStreamer with specific
// formatting rules and string truncation.
func ExampleNewRowStreamer() {
	var rows pgx.Rows

	// Define custom formatting
	cfg := pgfmt.Config{
		Null:            "NULL",
		BoolTrue:        "yes",
		BoolFalse:       "no",
		MaxStringLength: 20,
	}

	// Create a specialized streaming worker
	stream := pgfmt.NewRowStreamer(cfg.Format, nil)

	w := csv.NewWriter(os.Stdout)

	err := stream(w, rows)
	if err != nil {
		panic(err)
	}

	w.Flush()
}

// This example demonstrates how to configure the export for localized environments,
// specifically for opening CSV files in Russian or European Excel versions.
func ExampleConfig_localizedExcel() {
	var rows pgx.Rows // Assume this is your query result

	// 1. Setup config with localized formats
	cfg := pgfmt.Config{
		DecimalSeparator: ",",          // Standard for RU/EU locales
		DateOnlyFormat:   "02.01.2006", // Typical Russian date format
		DateTimeFormat:   "02.01.2006 15:04:05",
		Null:             "", // Better look for empty cells in Excel
	}

	// 2. Write UTF-8 BOM to stdout (or your file)
	// This is essential for Excel to correctly detect UTF-8 encoding.
	_, _ = os.Stdout.Write([]byte{0xEF, 0xBB, 0xBF})

	// 3. Setup CSV writer with a semicolon as the delimiter
	w := csv.NewWriter(os.Stdout)
	w.Comma = ';'

	// 4. Create and run the streamer
	stream := pgfmt.NewRowStreamer(cfg.Format, nil)
	if err := stream(w, rows); err != nil {
		panic(err)
	}

	// Don't forget to flush the CSV writer!
	w.Flush()
}

// This example demonstrates low-level Handler usage for non-CSV outputs,
// such as implementing a custom printer.
func ExampleNewHandler() {
	var rows pgx.Rows

	// Create a handler that processes a slice of strings
	handler := pgfmt.NewHandler(func(values []any, dst []string) {
		for i, v := range values {
			dst[i] = fmt.Sprint(v)
		}
	}, nil)

	// Yield receives the pre-allocated slice for each row
	err := handler(rows, func(row []string) error {
		fmt.Printf("Data: %v\n", row)
		return nil
	})
	if err != nil {
		panic(err)
	}
}

// This example demonstrates how to enable logging for unrecognized types
// and how to inspect them after the export is finished.
func ExampleConfig_unrecognizedTypes() {
	var rows pgx.Rows

	// 1. Setup config with a logger
	cfg := &pgfmt.Config{
		Logger: slog.New(slog.NewTextHandler(os.Stdout, nil)),
	}

	w := csv.NewWriter(os.Stdout)
	stream := pgfmt.NewRowStreamer(cfg.Format, nil)

	// 2. Perform export
	if err := stream(w, rows); err != nil {
		panic(err)
	}
	w.Flush()

	// 3. Check if any unknown types were encountered during the process
	unknowns := cfg.GetUnknownTypes()
	if len(unknowns) > 0 {
		fmt.Printf("Unrecognized types found: %v\n", unknowns)
	}
}
