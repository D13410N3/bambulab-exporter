package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"
)

// printerState keeps the merged view of the printer's "print" object. Bambu
// reports are incremental, so each message is deep-merged into the previous
// state.
type printerState struct {
	mu      sync.RWMutex
	print   map[string]any
	updated time.Time
}

func newPrinterState() *printerState {
	return &printerState{print: make(map[string]any)}
}

func (s *printerState) merge(update map[string]any) {
	if len(update) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	mergeMaps(s.print, update)
	s.updated = time.Now()
}

func (s *printerState) snapshot() (map[string]any, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return deepCopyMap(s.print), s.updated
}

func mergeMaps(dst, src map[string]any) {
	for key, value := range src {
		if srcMap, ok := value.(map[string]any); ok {
			if dstMap, ok := dst[key].(map[string]any); ok {
				mergeMaps(dstMap, srcMap)
				continue
			}
		}
		dst[key] = value
	}
}

func deepCopyMap(src map[string]any) map[string]any {
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = deepCopyValue(value)
	}
	return out
}

func deepCopyValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return deepCopyMap(value)
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = deepCopyValue(item)
		}
		return out
	default:
		return value
	}
}

func valueAt(root map[string]any, keys ...string) (any, bool) {
	var current any = root
	for _, key := range keys {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func objectAt(root map[string]any, keys ...string) (map[string]any, bool) {
	value, ok := valueAt(root, keys...)
	if !ok {
		return nil, false
	}
	object, ok := value.(map[string]any)
	return object, ok
}

func arrayAt(root map[string]any, keys ...string) ([]any, bool) {
	value, ok := valueAt(root, keys...)
	if !ok {
		return nil, false
	}
	array, ok := value.([]any)
	return array, ok
}

func floatAt(root map[string]any, keys ...string) (float64, bool) {
	value, ok := valueAt(root, keys...)
	if !ok {
		return 0, false
	}
	return floatValue(value)
}

func stringAt(root map[string]any, keys ...string) (string, bool) {
	value, ok := valueAt(root, keys...)
	if !ok {
		return "", false
	}
	return stringValue(value), true
}

func boolAt(root map[string]any, keys ...string) (bool, bool) {
	value, ok := valueAt(root, keys...)
	if !ok {
		return false, false
	}
	return boolValue(value)
}

func floatValue(value any) (float64, bool) {
	switch value := value.(type) {
	case json.Number:
		number, err := value.Float64()
		return number, err == nil
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case uint64:
		return float64(value), true
	case string:
		number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return number, err == nil
	case bool:
		if value {
			return 1, true
		}
		return 0, true
	default:
		return 0, false
	}
}

func stringValue(value any) string {
	switch value := value.(type) {
	case string:
		return value
	case json.Number:
		return value.String()
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(value), 'f', -1, 32)
	case int:
		return strconv.Itoa(value)
	case int64:
		return strconv.FormatInt(value, 10)
	case uint64:
		return strconv.FormatUint(value, 10)
	case bool:
		return strconv.FormatBool(value)
	default:
		return ""
	}
}

func boolValue(value any) (bool, bool) {
	switch value := value.(type) {
	case bool:
		return value, true
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "true", "on", "yes", "1":
			return true, true
		case "false", "off", "no", "0":
			return false, true
		}
	case json.Number:
		number, err := value.Float64()
		if err == nil {
			return number != 0, true
		}
	case float64:
		return value != 0, true
	}
	return false, false
}

func wifiSignalDBm(value any) (float64, bool) {
	if number, ok := floatValue(value); ok {
		return number, true
	}
	text, ok := value.(string)
	if !ok {
		return 0, false
	}
	text = strings.TrimSpace(text)
	if strings.HasSuffix(strings.ToLower(text), "dbm") {
		text = strings.TrimSpace(text[:len(text)-3])
	}
	number, err := strconv.ParseFloat(text, 64)
	return number, err == nil
}
