package sdata

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"io"
	"reflect"

	"github.com/pachyderm/pachyderm/v2/src/internal/errors"
)

// JSONWriter writes tuples as newline separated json objects.
type JSONWriter struct {
	bufw   *bufio.Writer
	enc    *json.Encoder
	fields []string
	record map[string]interface{}
}

// TODO: figure out some way to specify a projection so that we can write nested structures.
func NewJSONWriter(w io.Writer, fieldNames []string) *JSONWriter {
	bufw := bufio.NewWriter(w)
	enc := json.NewEncoder(bufw)
	return &JSONWriter{
		bufw:   bufw,
		enc:    enc,
		fields: fieldNames,
		record: make(map[string]interface{}, len(fieldNames)),
	}
}

func (m *JSONWriter) WriteTuple(row Tuple) error {
	if len(row) != len(m.fields) {
		return ErrTupleFields{Writer: m, Fields: m.fields, Tuple: row}
	}
	record := m.record
	for i := range row {
		var y interface{}
		switch x := row[i].(type) {
		case *sql.NullInt16:
			if x.Valid {
				y = x.Int16
			} else {
				y = nil
			}
		case *sql.NullInt32:
			if x.Valid {
				y = x.Int32
			} else {
				y = nil
			}
		case *sql.NullInt64:
			if x.Valid {
				y = x.Int64
			} else {
				y = nil
			}
		case *sql.NullFloat64:
			if x.Valid {
				y = x.Float64
			} else {
				y = nil
			}
		case *sql.NullString:
			if x.Valid {
				y = x.String
			} else {
				y = nil
			}
		case *sql.NullTime:
			if x.Valid {
				y = x.Time
			} else {
				y = nil
			}
		default:
			y = row[i]
		}
		record[m.fields[i]] = y
	}
	return errors.EnsureStack(m.enc.Encode(record))
}

func (m *JSONWriter) Flush() error {
	return errors.EnsureStack(m.bufw.Flush())
}

type jsonParser struct {
	dec        *json.Decoder
	fieldNames []string

	m map[string]interface{}
}

func NewJSONParser(r io.Reader, fieldNames []string) TupleReader {
	return &jsonParser{
		dec:        json.NewDecoder(r),
		fieldNames: fieldNames,
	}
}

func (p *jsonParser) Next(row Tuple) error {
	if len(row) != len(p.fieldNames) {
		return ErrTupleFields{Fields: p.fieldNames, Tuple: row}
	}
	m := p.getMap()
	if err := p.dec.Decode(&m); err != nil {
		return err
	}
	for i := range row {
		colName := p.fieldNames[i]
		v, exists := m[colName]
		if !exists {
			row[i] = nil
			continue
		}
		ty1 := reflect.TypeOf(v)
		ty2 := reflect.TypeOf(row[i])
		if ty2.AssignableTo(ty1) {
			row[i] = v
		} else if !ty1.ConvertibleTo(ty2) {
			return errors.Errorf("cannot convert %v to %v", ty1, ty2)
		} else {
			row[i] = reflect.ValueOf(v).Interface()
		}
	}
	return nil
}

func (p *jsonParser) getMap() map[string]interface{} {
	if p.m == nil {
		p.m = make(map[string]interface{})
	}
	return p.m
}
