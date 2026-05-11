// Package pgfmt provides a high-performance, stream-oriented toolkit for exporting PostgreSQL data
// retrieved via the pgx (v5) driver into various text-based formats like CSV, HTML, or logs.
//
// The library is designed with a focus on performance, type safety, and minimal allocations.
// It bridges the gap between raw database rows and formatted string output, handling
// complex PostgreSQL types (including [pgtype], and [zeronull]) with ease.
//
// Key components:
//
// 1. [Handler]: The core engine that manages row iteration and buffer reuse. It abstracts away
// the boilerplate of scanning rows and allows you to focus on how to process the resulting data.
//
// 2. [Config]: A comprehensive formatter that converts Go/[pgtype] values into strings. It supports
// smart truncation, custom null representations, boolean labels, localized time formats, and
// configurable decimal separators for international environments (e.g., Excel-friendly exports).
//
// 3. [RowStreamer]: An abstraction over row-based output formats. It allows streaming database
// results directly into any object implementing the [RowWriter] interface, such as the standard
// [encoding/csv.Writer].
//
// Performance considerations:
//   - Uses pre-allocated slices for scanning to minimize GC pressure during large exports.
//   - Implements an O(1) "fast path" for string truncation using byte-length checks before rune-counting.
//   - Avoids reflection in hot loops for primitive and common pgtype structures.
//   - Thread-safe by design: Uses atomic pointers for lazy initialization of internal registries,
//     allowing a single [Config] instance to be shared across multiple goroutines.
package pgfmt
