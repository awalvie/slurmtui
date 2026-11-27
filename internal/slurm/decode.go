package slurm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

func decodeRecord(raw json.RawMessage, into any) error {
	err := json.Unmarshal(raw, into)
	var typeErr *json.UnmarshalTypeError
	if !errors.As(err, &typeErr) || typeErr.Field == "" {
		return err
	}
	value, found := valueAt(raw, strings.Split(typeErr.Field, "."))
	if !found {
		return err
	}
	return fmt.Errorf("%s: %s is not %s", typeErr.Field, value, typeErr.Type)
}

func decodeEach[W, T any](url string, records []json.RawMessage, name func(W) string, convert func(W, json.RawMessage) (T, error)) ([]T, error) {
	out := make([]T, 0, len(records))
	for _, raw := range records {
		var w W
		err := decodeRecord(raw, &w)
		var v T
		if err == nil {
			v, err = convert(w, raw)
		}
		if err != nil {
			return nil, fmt.Errorf("GET %s: %s: %w", url, name(w), err)
		}
		out = append(out, v)
	}
	return out, nil
}

func valueAt(raw json.RawMessage, path []string) (json.RawMessage, bool) {
	for _, key := range path {
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil {
			return nil, false
		}
		next, ok := fields[key]
		if !ok {
			return nil, false
		}
		raw = next
	}
	return raw, true
}
