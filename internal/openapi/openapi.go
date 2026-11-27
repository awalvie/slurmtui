package openapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
)

const SlurmSpecFile = "testdata/slurm-26.05/openapi-slurmctld-v0.0.45.json"

type schema struct {
	Ref         string             `json:"$ref"`
	Type        string             `json:"type"`
	Format      string             `json:"format"`
	Properties  map[string]*schema `json:"properties"`
	Items       *schema            `json:"items"`
	Enum        []string           `json:"enum"`
	Required    []string           `json:"required"`
	Description string             `json:"description"`
	Deprecated  bool               `json:"deprecated"`
}

type mediaType struct {
	Schema *schema `json:"schema"`
}

type response struct {
	Content map[string]mediaType `json:"content"`
}

type operation struct {
	Responses map[string]response `json:"responses"`
}

type document struct {
	Paths      map[string]map[string]json.RawMessage `json:"paths"`
	Components struct {
		Schemas map[string]json.RawMessage `json:"schemas"`
	} `json:"components"`
}

type Spec struct {
	paths   map[string]map[string]operation
	schemas map[string]*schema
	names   map[*schema]string
}

var nullableInRealReplies = map[string]map[string]bool{
	"v0.0.45_job_info": {"job_resources": true},
}

var (
	slurmOnce sync.Once
	slurmSpec *Spec
	slurmErr  error
)

func Slurm() (*Spec, error) {
	slurmOnce.Do(func() {
		_, file, _, ok := runtime.Caller(0)
		if !ok {
			slurmErr = fmt.Errorf("find the source of package openapi to locate %s", SlurmSpecFile)
			return
		}
		slurmSpec, slurmErr = Load(filepath.Join(filepath.Dir(file), "..", "..", SlurmSpecFile))
	})
	return slurmSpec, slurmErr
}

func Load(path string) (*Spec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read OpenAPI spec: %w", err)
	}
	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse OpenAPI spec %s: %w", path, err)
	}
	s := &Spec{paths: map[string]map[string]operation{}, schemas: map[string]*schema{}, names: map[*schema]string{}}
	for name, rawSchema := range doc.Components.Schemas {
		sc, err := decodeStrict[schema](rawSchema)
		if err != nil {
			return nil, fmt.Errorf("spec %s: schema %s uses a keyword the checker does not know: %w", path, name, err)
		}
		s.schemas["#/components/schemas/"+name] = sc
		s.names[sc] = name
	}
	for p, methods := range doc.Paths {
		ops := map[string]operation{}
		for method, rawOp := range methods {
			if method == "parameters" {
				continue
			}
			var op operation
			if err := json.Unmarshal(rawOp, &op); err != nil {
				return nil, fmt.Errorf("spec %s: %s %s: %w", path, method, p, err)
			}
			ops[strings.ToUpper(method)] = op
		}
		s.paths[strings.TrimSuffix(p, "/")] = ops
	}
	if err := s.checkRefs(); err != nil {
		return nil, fmt.Errorf("spec %s: %w", path, err)
	}
	return s, nil
}

func decodeStrict[T any](raw []byte) (*T, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var v T
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (s *Spec) checkRefs() error {
	var walk func(sc *schema) error
	walk = func(sc *schema) error {
		if sc == nil {
			return nil
		}
		if sc.Ref != "" {
			if _, ok := s.schemas[sc.Ref]; !ok {
				return fmt.Errorf("unknown $ref %s", sc.Ref)
			}
		}
		for _, p := range sc.Properties {
			if err := walk(p); err != nil {
				return err
			}
		}
		return walk(sc.Items)
	}
	for _, sc := range s.schemas {
		if err := walk(sc); err != nil {
			return err
		}
	}
	for p, ops := range s.paths {
		for m, op := range ops {
			for status, r := range op.Responses {
				for ct, mt := range r.Content {
					if err := walk(mt.Schema); err != nil {
						return fmt.Errorf("%s %s %s %s: %w", m, p, status, ct, err)
					}
				}
			}
		}
	}
	return nil
}

func (s *Spec) CheckResponse(method, path string, status int, body []byte) error {
	where := fmt.Sprintf("%s %s %d", method, path, status)
	sc, err := s.responseSchema(method, path, status)
	if err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return fmt.Errorf("%s: body is not JSON: %w", where, err)
	}
	if dec.More() {
		return fmt.Errorf("%s: body has data after the JSON value", where)
	}
	if isFailureReply(v) {
		sc = s.withoutRequired(sc)
	}
	if err := s.check(sc, v, ""); err != nil {
		return fmt.Errorf("%s: %w", where, err)
	}
	return nil
}

func isFailureReply(v any) bool {
	obj, ok := v.(map[string]any)
	if !ok {
		return false
	}
	errs, ok := obj["errors"].([]any)
	return ok && len(errs) > 0
}

func (s *Spec) withoutRequired(sc *schema) *schema {
	for sc.Ref != "" {
		sc = s.schemas[sc.Ref]
	}
	waived := *sc
	waived.Required = nil
	return &waived
}

func (s *Spec) responseSchema(method, path string, status int) (*schema, error) {
	ops, err := s.operations(path)
	if err != nil {
		return nil, err
	}
	op, ok := ops[method]
	if !ok {
		return nil, fmt.Errorf("spec has no %s operation for this path", method)
	}
	r, ok := op.Responses[strconv.Itoa(status)]
	if !ok {
		r, ok = op.Responses["default"]
	}
	if !ok {
		return nil, fmt.Errorf("spec has no response for status %d", status)
	}
	mt, ok := r.Content["application/json"]
	if !ok || mt.Schema == nil {
		return nil, fmt.Errorf("spec has no application/json schema for status %d", status)
	}
	return mt.Schema, nil
}

func (s *Spec) operations(path string) (map[string]operation, error) {
	path = strings.TrimSuffix(path, "/")
	if ops, ok := s.paths[path]; ok {
		return ops, nil
	}
	parts := strings.Split(path, "/")
	for template, ops := range s.paths {
		if matchTemplate(strings.Split(template, "/"), parts) {
			return ops, nil
		}
	}
	return nil, fmt.Errorf("spec has no path that matches")
}

func matchTemplate(template, parts []string) bool {
	if len(template) != len(parts) {
		return false
	}
	for i, t := range template {
		isParam := strings.HasPrefix(t, "{") && strings.HasSuffix(t, "}")
		if isParam && parts[i] == "" {
			return false
		}
		if !isParam && t != parts[i] {
			return false
		}
	}
	return true
}

type FieldError struct {
	Field   string
	Problem string
}

func (e *FieldError) Error() string {
	return e.Field + ": " + e.Problem
}

func fieldError(field, format string, args ...any) error {
	return &FieldError{Field: field, Problem: fmt.Sprintf(format, args...)}
}

func fieldPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

func describe(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case string:
		return fmt.Sprintf("string %q", x)
	case bool:
		return fmt.Sprintf("boolean %t", x)
	case json.Number:
		return "number " + x.String()
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	panic(fmt.Sprintf("decoded JSON value of unexpected Go type %T", v))
}

func (s *Spec) check(sc *schema, v any, at string) error {
	if sc.Ref != "" {
		return s.check(s.schemas[sc.Ref], v, at)
	}
	name := at
	if name == "" {
		name = "(body)"
	}
	mismatch := func() error {
		return fieldError(name, "want %s, got %s", sc.Type, describe(v))
	}
	switch sc.Type {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return mismatch()
		}
		return s.checkObject(sc, obj, at)
	case "array":
		arr, ok := v.([]any)
		if !ok {
			return mismatch()
		}
		for i, item := range arr {
			if err := s.check(sc.Items, item, fmt.Sprintf("%s[%d]", at, i)); err != nil {
				return err
			}
		}
		return nil
	case "string":
		str, ok := v.(string)
		if !ok {
			return mismatch()
		}
		if len(sc.Enum) > 0 && !slices.Contains(sc.Enum, str) {
			return fieldError(name, "%q is not one of the spec values %v", str, sc.Enum)
		}
		return nil
	case "boolean":
		if _, ok := v.(bool); !ok {
			return mismatch()
		}
		return nil
	case "integer":
		n, ok := v.(json.Number)
		if !ok {
			return mismatch()
		}
		return checkInteger(name, sc.Format, n)
	case "number":
		if _, ok := v.(json.Number); !ok {
			return mismatch()
		}
		return nil
	}
	return fieldError(name, "spec type %q is not known to the checker", sc.Type)
}

func (s *Spec) checkObject(sc *schema, obj map[string]any, at string) error {
	for _, req := range sc.Required {
		if !hasPath(obj, strings.Split(req, "/")) {
			return fieldError(fieldPath(at, strings.ReplaceAll(req, "/", ".")), "required field is missing")
		}
	}
	for _, k := range slices.Sorted(maps.Keys(obj)) {
		prop, ok := sc.Properties[k]
		if !ok {
			return fieldError(fieldPath(at, k), "field is not in the spec")
		}
		if obj[k] == nil && nullableInRealReplies[s.names[sc]][k] {
			continue
		}
		if err := s.check(prop, obj[k], fieldPath(at, k)); err != nil {
			return err
		}
	}
	return nil
}

func hasPath(obj map[string]any, path []string) bool {
	v, ok := obj[path[0]]
	if !ok {
		return false
	}
	if len(path) == 1 {
		return true
	}
	child, ok := v.(map[string]any)
	return ok && hasPath(child, path[1:])
}

func checkInteger(name, format string, n json.Number) error {
	i, ok := new(big.Int).SetString(n.String(), 10)
	if !ok {
		return fieldError(name, "want integer, got number %s", n)
	}
	var lo, hi int64
	switch format {
	case "int32":
		lo, hi = math.MinInt32, math.MaxInt32
	case "int64", "":
		lo, hi = math.MinInt64, math.MaxInt64
	default:
		return fieldError(name, "spec integer format %q is not known to the checker", format)
	}
	if i.Cmp(big.NewInt(lo)) < 0 || i.Cmp(big.NewInt(hi)) > 0 {
		return fieldError(name, "%s is outside the %s range", n, format)
	}
	return nil
}
