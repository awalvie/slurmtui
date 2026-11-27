package main

import (
	"os"
	"strings"

	"github.com/awalvie/slurmtui/internal/app"
	"github.com/awalvie/slurmtui/internal/clock"
)

func main() {
	env := map[string]string{}
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		env[k] = v
	}
	os.Exit(app.Main(app.Process{
		Args:  os.Args[1:],
		Env:   env,
		In:    os.Stdin,
		Out:   os.Stdout,
		Err:   os.Stderr,
		Clock: clock.Real{},
	}))
}
