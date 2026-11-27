package app

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/awalvie/slurmtui/internal/auth"
	"github.com/awalvie/slurmtui/internal/clock"
	"github.com/awalvie/slurmtui/internal/config"
	"github.com/awalvie/slurmtui/internal/ui"
)

type Process struct {
	Args   []string
	Env    map[string]string
	In     io.Reader
	Out    io.Writer
	Err    io.Writer
	Clock  clock.Clock
	Width  int
	Height int
}

func Main(p Process) int {
	err := run(p)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		if _, werr := fmt.Fprintf(p.Err, "slurmtui: %s\n", err); werr != nil {
			return 2
		}
		return 1
	}
	return 0
}

func parse(args []string, stderr io.Writer) (config.Flags, error) {
	var f config.Flags
	fs := flag.NewFlagSet("slurmtui", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.Context, "context", "", "context name from the config file")
	fs.StringVar(&f.URL, "url", "", "slurmrestd URL")
	fs.StringVar(&f.User, "user", "", "Slurm user name")
	fs.IntVar(&f.RefreshSeconds, "refresh", 0, "refresh interval in seconds (default 5)")
	fs.IntVar(&f.RefreshSeconds, "r", 0, "refresh interval in seconds (short)")
	readOnly := fs.Bool("readonly", false, "read-only mode: no drain, resume or cancel")
	write := fs.Bool("write", false, "allow actions, also for a read-only context")
	if err := fs.Parse(args); err != nil {
		return config.Flags{}, err
	}
	refreshSet := false
	fs.Visit(func(fl *flag.Flag) { refreshSet = refreshSet || fl.Name == "refresh" || fl.Name == "r" })
	if refreshSet && f.RefreshSeconds < config.MinRefreshSeconds {
		return config.Flags{}, fmt.Errorf("--refresh must be at least %d seconds, got %d", config.MinRefreshSeconds, f.RefreshSeconds)
	}
	switch {
	case *readOnly && *write:
		return config.Flags{}, errors.New("--readonly and --write cannot be used together")
	case *readOnly:
		f.ReadOnly = readOnly
	case *write:
		f.ReadOnly = new(bool)
	}
	if fs.NArg() > 0 {
		return config.Flags{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	return f, nil
}

func missingError(r config.Resolved) error {
	errs := make([]error, 0, len(r.Missing))
	for _, field := range r.Missing {
		switch field {
		case config.FieldURL:
			errs = append(errs, fmt.Errorf("context %q has no url", r.Context.Name))
		case config.FieldUser:
			errs = append(errs, fmt.Errorf("context %q: --user or user is required with keyFile", r.Context.Name))
		case config.FieldToken:
			errs = append(errs, fmt.Errorf("%s is not set", r.Context.Token.Env))
		}
	}
	return errors.Join(errs...)
}

func newAuth(ctx config.Context, env map[string]string, clk clock.Clock) (auth.Source, error) {
	if ctx.Token.KeyFile != "" {
		key, err := auth.ReadKeyFile(ctx.Token.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("context %q: keyFile: %w", ctx.Name, err)
		}
		return auth.NewSigned(ctx.User, key, clk), nil
	}
	return auth.NewProvided(ctx.Token.Env, env[ctx.Token.Env], ctx.User, clk)
}

func run(p Process) error {
	flags, err := parse(p.Args, p.Err)
	if err != nil {
		return err
	}
	resolved, err := config.Resolve(flags, p.Env)
	if err != nil {
		return err
	}
	interval := time.Duration(resolved.RefreshSeconds) * time.Second
	if resolved.InFile {
		if err := missingError(resolved); err != nil {
			return err
		}
	}
	if len(resolved.Missing) > 0 {
		return runModel(p, ui.NewSetup(draftFrom(resolved, p.Env), connector(resolved.Dir, p.Env, p.Clock, resolved.ReadOnly), p.Clock, interval))
	}
	ctx := resolved.Context
	source, err := newAuth(ctx, p.Env, p.Clock)
	if err != nil {
		return err
	}
	client, err := newClient(ctx, source)
	if err != nil {
		return err
	}
	return runModel(p, ui.New(writeGuard(client, resolved.ReadOnly), session(ctx, source, resolved.ReadOnly), p.Clock, interval))
}

func runModel(p Process, model tea.Model) error {
	env := make([]string, 0, len(p.Env))
	for k, v := range p.Env {
		env = append(env, k+"="+v)
	}
	opts := []tea.ProgramOption{
		tea.WithInput(p.In),
		tea.WithOutput(p.Out),
		tea.WithEnvironment(env),
	}
	if p.Width > 0 {
		opts = append(opts, tea.WithWindowSize(p.Width, p.Height))
	}
	_, err := tea.NewProgram(model, opts...).Run()
	return err
}
