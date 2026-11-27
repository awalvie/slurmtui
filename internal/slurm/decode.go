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
