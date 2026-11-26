package sqladapter

import (
	"database/sql"
	"math"
	"time"

	"github.com/igorpooh1978/GoEF/provider"
)

func arguments(slots []provider.Slot, bindings provider.Bindings) []any {
	args := make([]any, len(bindings))
	for i, binding := range bindings {
		value := sqlValue(binding)
		if slots[i].Name != "" {
			value = sql.Named(slots[i].Name, value)
		}
		args[i] = value
	}
	return args
}

// ValidateBindings has already excluded types outside this switch.
func sqlValue(binding provider.Value) any {
	switch value := binding.(type) {
	case provider.Integer:
		return int64(value)
	case provider.Float:
		return float64(value)
	case provider.Boolean:
		return bool(value)
	case provider.Text:
		return string(value)
	case provider.Binary:
		if value == nil {
			return []byte{}
		}
		return []byte(value)
	case provider.Timestamp:
		return time.Time(value)
	default:
		return nil
	}
}

func cellValue(raw any, column provider.Column, integerBool bool) (provider.Value, bool) {
	if raw == nil {
		return provider.Null{}, column.Nullable
	}
	switch column.Kind {
	case provider.KindInteger:
		switch value := raw.(type) {
		case int64:
			return provider.Integer(value), true
		case uint64:
			if value <= math.MaxInt64 {
				return provider.Integer(value), true
			}
		}
	case provider.KindFloat:
		switch value := raw.(type) {
		case float64:
			return provider.Float(value), true
		case float32:
			return provider.Float(value), true
		}
	case provider.KindBoolean:
		switch value := raw.(type) {
		case bool:
			return provider.Boolean(value), true
		case int64:
			if integerBool && (value == 0 || value == 1) {
				return provider.Boolean(value == 1), true
			}
		case uint64:
			if integerBool && (value == 0 || value == 1) {
				return provider.Boolean(value == 1), true
			}
		}
	case provider.KindText:
		switch value := raw.(type) {
		case string:
			return provider.Text(value), true
		case []byte:
			return provider.Text(value), true
		}
	case provider.KindBinary:
		if value, ok := raw.([]byte); ok {
			return provider.Binary(value), true
		}
	case provider.KindTimestamp:
		if value, ok := raw.(time.Time); ok {
			return provider.Timestamp(value), true
		}
	}
	return nil, false
}
