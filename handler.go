package pgfmt

import (
	"fmt"
	"maps"
	"reflect"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// Handler is a function that processes [pgx.Rows] and calls yield for each transformed row.
// It encapsulates the entire streaming logic.
type Handler[T any] func(rows pgx.Rows, yield func([]T) error) error

// Transformer maps raw database values into a destination slice.
type Transformer[T any] func(values []any, dst []T)

// DefaultTypes provides pre-compiled [reflect.Type] descriptors for common PostgreSQL OIDs.
var DefaultTypes = map[uint32]reflect.Type{
	pgtype.TimeOID:    reflect.TypeFor[pgtype.Time](),
	pgtype.DateOID:    reflect.TypeFor[pgtype.Date](),
	pgtype.NumericOID: reflect.TypeFor[pgtype.Numeric](),
	pgtype.BitOID:     reflect.TypeFor[pgtype.Bits](),
	pgtype.VarbitOID:  reflect.TypeFor[pgtype.Bits](),
	pgtype.UUIDOID:    reflect.TypeFor[pgtype.UUID](),
}

// NewHandler returns a function tailored for high-performance row processing.
// All type reflection and configuration are performed once during the handler's creation.
func NewHandler[T any](transform Transformer[T], customTypes map[uint32]any) Handler[T] {
	// Prepare the type map once and keep it in the closure.
	typeMap := maps.Clone(DefaultTypes)
	for oid, val := range customTypes {
		typeMap[oid] = reflect.TypeOf(val)
	}

	return func(rows pgx.Rows, yield func([]T) error) error {
		fields := rows.FieldDescriptions()
		colCount := len(fields)

		// Pre-allocate buffers for the lifetime of the rows execution.
		scanArgs := make([]any, colCount)
		values := make([]any, colCount)
		resultRow := make([]T, colCount)

		for idx, field := range fields {
			if t, ok := typeMap[field.DataTypeOID]; ok {
				// Allocate a typed buffer once per stream.
				valPtr := reflect.New(t).Interface()
				scanArgs[idx] = valPtr
				values[idx] = valPtr
			} else {
				// Use direct pointers to the values slice slots.
				scanArgs[idx] = &values[idx]
			}
		}

		for rows.Next() {
			err := rows.Scan(scanArgs...)
			if err != nil {
				return fmt.Errorf("scan error: %w", err)
			}

			transform(values, resultRow)

			err = yield(resultRow)
			if err != nil {
				return fmt.Errorf("yield error: %w", err)
			}
		}

		return rows.Err()
	}
}
