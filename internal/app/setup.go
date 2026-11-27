package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/awalvie/slurmtui/internal/auth"
	"github.com/awalvie/slurmtui/internal/clock"
	"github.com/awalvie/slurmtui/internal/config"
	"github.com/awalvie/slurmtui/internal/slurm"
	"github.com/awalvie/slurmtui/internal/ui"
)

const pingTimeout = 20 * time.Second

func draftFrom(r config.Resolved, env map[string]string) ui.Draft {
	ctx := r.Context
	d := ui.Draft{
		Name:    ctx.Name,
		URL:     ctx.URL,
		CAFile:  ctx.CAFile,
		User:    ctx.User,
		Kind:    ui.KeyFileToken,
		KeyFile: ctx.Token.KeyFile,
		Env:     ctx.Token.Env,
	}
	if d.Env == "" {
		d.Env = config.DefaultTokenEnv
	}
	if ctx.Token.KeyFile == "" && (env[d.Env] != "" || d.Env != config.DefaultTokenEnv) {
		d.Kind = ui.EnvToken
	}
	return d
}

func connector(dir string, env map[string]string, clk clock.Clock, readOnly bool) ui.Connect {
	return func(d ui.Draft) (ui.Connected, error) {
		c, err := checkDraft(d, dir, env, clk)
		if err != nil {
			return ui.Connected{}, err
		}
		pingCtx, cancel := context.WithTimeout(context.Background(), pingTimeout)
		defer cancel()
		if err := c.client.Ping(pingCtx); err != nil {
			return ui.Connected{}, err
		}
		if d.Kind != ui.PastedToken {
			saved := c.ctx
			saved.CAFile = d.CAFile
			saved.Token.KeyFile = d.KeyFile
			if err := config.AddContext(dir, saved); err != nil {
				return ui.Connected{}, err
			}
		}
		return ui.Connected{Source: writeGuard(c.client, readOnly), Session: session(c.ctx, c.source, readOnly)}, nil
	}
}

func inField(label string, err error) error {
	return fmt.Errorf("%s: %w", label, err)
}

var errEmpty = errors.New("must not be empty")

type checked struct {
	ctx    config.Context
	source auth.Source
	client *slurm.Client
}

func draftContext(d ui.Draft) (config.Context, error) {
	ctx := config.Context{Name: d.Name, URL: d.URL, CAFile: d.CAFile, User: d.User}
	switch d.Kind {
	case ui.KeyFileToken:
		if d.KeyFile == "" {
			return ctx, inField("Key file", errEmpty)
		}
		if d.User == "" {
			return ctx, inField("User", errors.New("must not be empty with a key file"))
		}
		ctx.Token.KeyFile = d.KeyFile
	case ui.EnvToken:
		if d.Env == "" {
			return ctx, inField("Env var", errEmpty)
		}
		ctx.Token.Env = d.Env
	case ui.PastedToken:
		if d.Pasted == "" {
			return ctx, inField("Token", errEmpty)
		}
	}
	return ctx, nil
}

func checkDraft(d ui.Draft, dir string, env map[string]string, clk clock.Clock) (checked, error) {
	if err := config.CheckNewContextName(dir, d.Name); err != nil {
		return checked{}, inField("Context name", err)
	}
	if d.URL == "" {
		return checked{}, inField("URL", errEmpty)
	}
	if _, err := config.CheckURL(d.URL); err != nil {
		return checked{}, inField("URL", fmt.Errorf("%s: %w", d.URL, err))
	}
	ctx, err := draftContext(d)
	if err != nil {
		return checked{}, err
	}
	if ctx, err = config.ResolvePaths(ctx, dir, env["HOME"]); err != nil {
		return checked{}, err
	}
	source, err := draftAuth(d, ctx, env, clk)
	if err != nil {
		return checked{}, err
	}
	client, err := newClient(ctx, source)
	if err != nil {
		return checked{}, err
	}
	return checked{ctx: ctx, source: source, client: client}, nil
}

func draftAuth(d ui.Draft, ctx config.Context, env map[string]string, clk clock.Clock) (auth.Source, error) {
	switch {
	case d.Kind == ui.PastedToken:
		source, err := auth.NewProvided("pasted token", string(d.Pasted), d.User, clk)
		if err != nil {
			return nil, inField("Token", err)
		}
		return source, nil
	case d.Kind == ui.EnvToken && env[d.Env] == "":
		return nil, inField("Env var", fmt.Errorf("%s is not set", d.Env))
	}
	return newAuth(ctx, env, clk)
}

func newClient(ctx config.Context, source auth.Source) (*slurm.Client, error) {
	roots, err := config.CAPool(ctx.CAFile)
	if err != nil {
		return nil, fmt.Errorf("context %q: caFile: %w", ctx.Name, err)
	}
	return &slurm.Client{BaseURL: ctx.URL, Auth: source, HTTP: slurm.NewHTTPClient(roots)}, nil
}

func session(ctx config.Context, source auth.Source, readOnly bool) ui.Session {
	return ui.Session{Context: ctx.Name, User: source.User(), Expires: source.Expires(), ReadOnly: readOnly}
}

var errReadOnly = errors.New("read-only mode: slurmtui sends no change to Slurm")

type readOnlySource struct {
	ui.Source
}

func (readOnlySource) UpdateNode(context.Context, string, slurm.NodeUpdate) error {
	return errReadOnly
}

func (readOnlySource) CancelJob(context.Context, string) error {
	return errReadOnly
}

func writeGuard(source ui.Source, readOnly bool) ui.Source {
	if readOnly {
		return readOnlySource{source}
	}
	return source
}
