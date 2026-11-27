package config

import (
	"errors"
	"fmt"
	"strings"

	"go.yaml.in/yaml/v3"
)

type file struct {
	refreshSeconds int
	readOnly       *bool
	contexts       []Context
}

type fieldError struct {
	path string
	line int
	msg  string
}

func (e fieldError) Error() string {
	return fmt.Sprintf("line %d: %s: %s", e.line, e.path, e.msg)
}

func fail(n *yaml.Node, path, format string, args ...any) error {
	return fieldError{path: path, line: n.Line, msg: fmt.Sprintf(format, args...)}
}

func parse(data []byte) (file, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return file{}, err
	}
	if len(doc.Content) == 0 {
		return file{}, nil
	}
	var f file
	err := eachField(doc.Content[0], "", func(key string, value *yaml.Node, path string) error {
		if key != "slurmtui" {
			return fail(value, path, "unknown field")
		}
		return parseSlurmtui(value, path, &f)
	})
	return f, err
}

func eachField(n *yaml.Node, path string, fn func(key string, value *yaml.Node, path string) error) error {
	if n.Kind != yaml.MappingNode {
		return fail(n, strings.TrimPrefix(path, "."), "must be a mapping")
	}
	seen := map[string]bool{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i].Value
		sub := strings.TrimPrefix(path+"."+key, ".")
		if seen[key] {
			return fail(n.Content[i], sub, "is set twice")
		}
		seen[key] = true
		if err := fn(key, n.Content[i+1], sub); err != nil {
			return err
		}
	}
	return nil
}

func parseSlurmtui(n *yaml.Node, path string, f *file) error {
	return eachField(n, path, func(key string, value *yaml.Node, path string) error {
		switch key {
		case "refreshRate":
			if err := decode(value, path, &f.refreshSeconds); err != nil {
				return err
			}
			if f.refreshSeconds < MinRefreshSeconds {
				return fail(value, path, "must be at least %d seconds, got %d", MinRefreshSeconds, f.refreshSeconds)
			}
			return nil
		case "readOnly":
			f.readOnly = new(bool)
			return decode(value, path, f.readOnly)
		case "contexts":
			return eachField(value, path, func(name string, value *yaml.Node, path string) error {
				ctx, err := parseContext(name, value, path)
				if err != nil {
					return err
				}
				f.contexts = append(f.contexts, ctx)
				return nil
			})
		}
		return fail(value, path, "unknown field")
	})
}

func parseContext(name string, n *yaml.Node, path string) (Context, error) {
	ctx := Context{Name: name}
	if name == "" {
		return Context{}, fail(n, path, "context name must not be empty")
	}
	err := eachField(n, path, func(key string, value *yaml.Node, path string) error {
		switch key {
		case "url":
			if err := decodeText(value, path, &ctx.URL); err != nil {
				return err
			}
			if _, err := CheckURL(ctx.URL); err != nil {
				return fail(value, path, "%s: %s", ctx.URL, err)
			}
			return nil
		case "caFile":
			return decodeText(value, path, &ctx.CAFile)
		case "user":
			return decodeText(value, path, &ctx.User)
		case "keyFile":
			return decodeText(value, path, &ctx.Token.KeyFile)
		case "tokenEnv":
			return decodeText(value, path, &ctx.Token.Env)
		case "readOnly":
			ctx.ReadOnly = new(bool)
			return decode(value, path, ctx.ReadOnly)
		}
		return fail(value, path, "unknown field")
	})
	if err != nil {
		return Context{}, err
	}
	if (ctx.Token.KeyFile == "") == (ctx.Token.Env == "") {
		return Context{}, fail(n, path, "needs exactly one of keyFile or tokenEnv")
	}
	return ctx, nil
}

func decode(n *yaml.Node, path string, into any) error {
	if n.Kind != yaml.ScalarNode {
		return fail(n, path, "must be a single value")
	}
	if err := n.Decode(into); err != nil {
		var te *yaml.TypeError
		if errors.As(err, &te) {
			return fail(n, path, "%q is not %s", n.Value, kindOf(into))
		}
		return fail(n, path, "%s", err)
	}
	return nil
}

func decodeText(n *yaml.Node, path string, into *string) error {
	if n.Tag != "!!str" {
		return fail(n, path, "must be text")
	}
	if err := decode(n, path, into); err != nil {
		return err
	}
	if *into == "" {
		return fail(n, path, "must not be empty")
	}
	return nil
}

func kindOf(into any) string {
	switch into.(type) {
	case *int:
		return "a whole number"
	case *bool:
		return "true or false"
	}
	return "text"
}
