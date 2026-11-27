package config

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	FileName              = "config.yaml"
	DefaultRefreshSeconds = 5
	MinRefreshSeconds     = 2
	DefaultTokenEnv       = "SLURM_JWT"
)

type Field string

const (
	FieldURL   Field = "url"
	FieldUser  Field = "user"
	FieldToken Field = "token"
)

type TokenSource struct {
	Env     string
	KeyFile string
}

type Context struct {
	Name     string
	URL      string
	CAFile   string
	User     string
	Token    TokenSource
	ReadOnly *bool
}

type Flags struct {
	Context        string
	URL            string
	User           string
	RefreshSeconds int
	ReadOnly       *bool
}

type Resolved struct {
	Dir            string
	Context        Context
	RefreshSeconds int
	ReadOnly       bool
	Missing        []Field
	InFile         bool
}

func Dir(env map[string]string) (string, error) {
	if dir := env["SLURMTUI_CONFIG_DIR"]; dir != "" {
		return dir, nil
	}
	if xdg := env["XDG_CONFIG_HOME"]; xdg != "" {
		return filepath.Join(xdg, "slurmtui"), nil
	}
	if home := env["HOME"]; home != "" {
		return filepath.Join(home, ".config", "slurmtui"), nil
	}
	return "", errors.New("config directory: SLURMTUI_CONFIG_DIR, XDG_CONFIG_HOME and HOME are all unset")
}

func Resolve(flags Flags, env map[string]string) (Resolved, error) {
	dir, err := Dir(env)
	if err != nil {
		return Resolved{}, err
	}
	path := filepath.Join(dir, FileName)
	f, err := readFile(path)
	if err != nil {
		return Resolved{}, err
	}
	ctx, err := selectContext(flags, f)
	if err != nil {
		return Resolved{}, err
	}
	if ctx, err = ResolvePaths(ctx, dir, env["HOME"]); err != nil {
		return Resolved{}, err
	}
	refresh := DefaultRefreshSeconds
	if f.refreshSeconds != 0 {
		refresh = f.refreshSeconds
	}
	if flags.RefreshSeconds != 0 {
		refresh = flags.RefreshSeconds
	}
	return Resolved{
		Dir:            dir,
		Context:        ctx,
		RefreshSeconds: refresh,
		ReadOnly:       readOnly(flags.ReadOnly, ctx.ReadOnly, f.readOnly),
		Missing:        missing(ctx, env),
		InFile:         flags.Context != "" || (flags.URL == "" && len(f.contexts) == 1),
	}, nil
}

func ResolvePaths(ctx Context, dir, home string) (Context, error) {
	for _, p := range []struct {
		field string
		path  *string
	}{{"caFile", &ctx.CAFile}, {"keyFile", &ctx.Token.KeyFile}} {
		if *p.path == "" {
			continue
		}
		resolved, err := resolvePath(dir, home, *p.path)
		if err != nil {
			return Context{}, fmt.Errorf("context %q: %s: %w", ctx.Name, p.field, err)
		}
		*p.path = resolved
	}
	return ctx, nil
}

func readOnly(byPrecedence ...*bool) bool {
	for _, v := range byPrecedence {
		if v != nil {
			return *v
		}
	}
	return false
}

func readFile(path string) (file, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return file{}, nil
	}
	if err != nil {
		return file{}, fmt.Errorf("config %s: %w", path, err)
	}
	f, err := parse(data)
	if err != nil {
		return file{}, fmt.Errorf("config %s: %w", path, err)
	}
	return f, nil
}

func selectContext(flags Flags, f file) (Context, error) {
	names := make([]string, 0, len(f.contexts))
	for _, c := range f.contexts {
		names = append(names, c.Name)
	}
	slices.Sort(names)
	var u *url.URL
	if flags.URL != "" {
		var err error
		u, err = CheckURL(flags.URL)
		if err != nil {
			return Context{}, fmt.Errorf("--url %s: %w", flags.URL, err)
		}
	}
	ctx := Context{Token: TokenSource{Env: DefaultTokenEnv}}
	switch {
	case flags.Context != "":
		i := slices.IndexFunc(f.contexts, func(c Context) bool { return c.Name == flags.Context })
		if i < 0 {
			return Context{}, fmt.Errorf("--context %q: no such context in the config file (contexts: %s)", flags.Context, listOrNone(names))
		}
		ctx = f.contexts[i]
	case flags.URL != "":
		ctx.Name = u.Host
	case len(f.contexts) > 1:
		return Context{}, fmt.Errorf("--context is required when the config file has more than one context (contexts: %s)", listOrNone(names))
	case len(f.contexts) == 1:
		ctx = f.contexts[0]
	}
	if flags.URL != "" {
		ctx.URL = flags.URL
	}
	if flags.User != "" {
		ctx.User = flags.User
	}
	return ctx, nil
}

func listOrNone(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, ", ")
}

func missing(ctx Context, env map[string]string) []Field {
	var out []Field
	if ctx.URL == "" {
		out = append(out, FieldURL)
	}
	if ctx.Token.KeyFile != "" && ctx.User == "" {
		out = append(out, FieldUser)
	}
	if ctx.Token.Env != "" && env[ctx.Token.Env] == "" {
		out = append(out, FieldToken)
	}
	return out
}
