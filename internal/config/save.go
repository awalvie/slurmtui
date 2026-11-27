package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"go.yaml.in/yaml/v3"
)

func CheckNewContextName(dir, name string) error {
	path := filepath.Join(dir, FileName)
	f, err := readFile(path)
	if err != nil {
		return err
	}
	return checkNewName(f, path, name)
}

func checkNewName(f file, path, name string) error {
	if name == "" {
		return errors.New("must not be empty")
	}
	if slices.ContainsFunc(f.contexts, func(c Context) bool { return c.Name == name }) {
		return fmt.Errorf("%q is already a context in %s; choose another name", name, path)
	}
	return nil
}

func AddContext(dir string, ctx Context) error {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("config %s: %w", path, err)
	}
	f, err := parse(data)
	if err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	if err := checkNewName(f, path, ctx.Name); err != nil {
		return fmt.Errorf("context name: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	if len(doc.Content) == 0 {
		doc.Kind = yaml.DocumentNode
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	contexts := mappingAt(mappingAt(doc.Content[0], "slurmtui"), "contexts")
	contexts.Content = append(contexts.Content, text(ctx.Name), contextNode(ctx))
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	if _, err := parse(out.Bytes()); err != nil {
		return fmt.Errorf("config %s: the new file does not parse: %w", path, err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("config dir %s: %w", dir, err)
	}
	return replaceFile(path, out.Bytes())
}

func mappingAt(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	child := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	m.Content = append(m.Content, text(key), child)
	return child
}

func text(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func contextNode(ctx Context) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, kv := range [][2]string{
		{"url", ctx.URL},
		{"caFile", ctx.CAFile},
		{"user", ctx.User},
		{"keyFile", ctx.Token.KeyFile},
		{"tokenEnv", ctx.Token.Env},
	} {
		if kv[1] != "" {
			n.Content = append(n.Content, text(kv[0]), text(kv[1]))
		}
	}
	return n
}

func replaceFile(path string, data []byte) error {
	target, err := filepath.EvalSymlinks(path)
	mode := fs.FileMode(0o600)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		target = path
	case err != nil:
		return fmt.Errorf("config %s: %w", path, err)
	default:
		info, err := os.Stat(target)
		if err != nil {
			return fmt.Errorf("config %s: %w", path, err)
		}
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".config.yaml.*")
	if err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	if err := writeAndRename(tmp, mode, data, target); err != nil {
		if rerr := os.Remove(tmp.Name()); rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
			err = errors.Join(err, rerr)
		}
		return fmt.Errorf("config %s: %w", path, err)
	}
	return nil
}

func writeAndRename(tmp *os.File, mode fs.FileMode, data []byte, target string) error {
	_, err := tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(mode)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), target)
}
