package pgfmt_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn" // Правильный пакет для FieldDescription
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/mdigger/pgfmt"
)

func TestNewHandler(t *testing.T) {
	// 1. Используем твой реальный Config для теста трансформатора
	cfg := &pgfmt.Config{
		Null: "NULL",
	}

	// Transformer теперь ведет себя как в реальности
	transformer := func(values []any, dst []string) {
		for i, v := range values {
			dst[i] = cfg.Format(v)
		}
	}

	customOID := uint32(12345)
	customTypes := map[uint32]any{
		customOID: int64(0),
	}

	handler := pgfmt.NewHandler(transformer, customTypes)

	// 2. Подготавливаем Mock
	mock := &mockRows{
		fields: []pgconn.FieldDescription{
			{Name: "uuid", DataTypeOID: pgtype.UUIDOID},
			{Name: "custom", DataTypeOID: customOID},
			{Name: "raw", DataTypeOID: 999},
		},
		data: [][]any{
			{
				pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
				int64(100),
				"raw_text",
			},
		},
	}

	var results [][]string
	err := handler(mock, func(row []string) error {
		tmp := make([]string, len(row))
		copy(tmp, row)
		results = append(results, tmp)
		return nil
	})
	if err != nil {
		t.Fatalf("handler failed: %v", err)
	}

	// 3. Проверки теперь соответствуют работе pgfmt
	res := results[0]

	// UUID теперь проверяем на стандартный строковый формат
	if res[0] != "01000000-0000-0000-0000-000000000000" {
		t.Errorf("unexpected UUID format: %s", res[0])
	}

	if res[1] != "100" {
		t.Errorf("unexpected custom value: %s", res[1])
	}

	if res[2] != "raw_text" {
		t.Errorf("unexpected raw value: %s", res[2])
	}
}

// --- Обновленный Mock ---

type mockRows struct {
	pgx.Rows
	fields []pgconn.FieldDescription // Исправленный тип
	data   [][]any
	idx    int
}

func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription { return m.fields }
func (m *mockRows) Next() bool {
	m.idx++
	return m.idx <= len(m.data)
}
func (m *mockRows) Err() error { return nil }
func (m *mockRows) Close()     {}

func (m *mockRows) Scan(dest ...any) error {
	row := m.data[m.idx-1]
	for i, val := range row {
		d := dest[i]

		// Если NewHandler подставил *any (для неизвестных типов)
		if p, ok := d.(*any); ok {
			*p = val
			continue
		}

		// Если NewHandler подставил типизированный указатель (reflect.New)
		dv := reflect.ValueOf(d).Elem()
		sv := reflect.ValueOf(val)

		if dv.Type() == sv.Type() {
			dv.Set(sv)
		} else {
			return fmt.Errorf("type mismatch at col %d: want %v, got %v", i, dv.Type(), sv.Type())
		}
	}
	return nil
}
