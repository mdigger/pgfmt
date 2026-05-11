# pgfmt

High-performance, stream-oriented toolkit for exporting PostgreSQL data (via [pgx v5](https://github.com/jackc/pgx)) into various text-based formats like CSV, HTML, or logs.

## Features

- **High Performance**: Minimal allocations through buffer reuse and "fast path" optimizations for common types.
- **Universal Output**: Stream data to any target using the `RowWriter` interface (compatible with `encoding/csv`).
- **Robust Type Support**: Handles complex PostgreSQL types including `pgtype` (UUID, Numeric, Intervals, Network addresses) and `zeronull`.
- **Internationalized**: Configurable decimal separators (e.g., for European/Russian Excel), localized date/time formats, and custom boolean labels.
- **Memory Efficient**: Processes billions of rows in a streaming fashion without loading the entire result set into memory.

## Architecture

| Layer | Component | Responsibility |
| :--- | :--- | :--- |
| **High Level** | `Write` | Simple one-liner for standard exports to any `RowWriter`. |
| **Middle Level** | `Config` | Formatting logic (nulls, dates, decimals, truncation). |
| **Core Level** | `Handler` | Row iteration, buffer management, and low-level scanning. |

## Installation

```bash
go get github.com/mdigger/pgfmt
```

## Quick Start

### 1. Simple Export to CSV

The easiest way to export rows using default settings:

```go
w := csv.NewWriter(os.Stdout)
err := pgfmt.Write(w, rows)
w.Flush()
```

### 2. Localized Export (e.g., for Russian/European Excel)

Handle specific regional requirements like semicolon delimiters, comma decimals, and UTF-8 BOM:

```go
cfg := pgfmt.Config{
    DecimalSeparator: ",",
    DateOnlyFormat:   "02.01.2006",
}

// Write BOM for Excel auto-detection
os.Stdout.Write([]byte{0xEF, 0xBB, 0xBF})

w := csv.NewWriter(os.Stdout)
w.Comma = ';'

streamer := pgfmt.NewRowStreamer(cfg.Format, nil)
err := streamer(w, rows)
w.Flush()

```

### 3. Custom Processing (Generic Handler)

Use the core `Handler` to transform rows into any custom format (e.g., printing to console):

```go
handler := pgfmt.NewHandler(func(values []any, dst []string) {
    for i, v := range values {
        dst[i] = fmt.Sprint(v)
    }
}, nil)

err := handler(rows, func(row []string) error {
    fmt.Printf("Data: %v\n", row)
    return nil
})

```

## Observability

The library can track unrecognized types during export. This is useful for debugging and fine-tuning your configuration:

```go
cfg := &pgfmt.Config{
    Logger: slog.Default(),
}
// After export:
unknownTypes := cfg.GetUnknownTypes()
```

## Performance Tips

* **String Truncation**: `Config` uses an $O(1)$ byte-length check before falling back to rune counting for truncation.
* **Lock-Free Tracking**: Type monitoring uses `atomic.Pointer` and `sync.Map` for zero contention during parallel exports.
* **Buffer Reuse**: `Handler` allocates row buffers once per stream, significantly reducing GC pressure.
