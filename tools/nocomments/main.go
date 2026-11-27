package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	found, err := scan(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "nocomments: %s\n", err)
		os.Exit(2)
	}
	for _, f := range found {
		fmt.Println(f)
	}
	if len(found) > 0 {
		fmt.Fprintf(os.Stderr, "nocomments: %d code comments found; the repo rule is no code comments\n", len(found))
		os.Exit(1)
	}
}

func scan(root string) ([]string, error) {
	var found []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (strings.HasPrefix(d.Name(), ".") && path != root || d.Name() == "result") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		for _, group := range file.Comments {
			for _, c := range group.List {
				if strings.HasPrefix(c.Text, "//go:") || strings.HasPrefix(c.Text, "//nolint") {
					continue
				}
				found = append(found, fmt.Sprintf("%s: %s", fset.Position(c.Pos()), c.Text))
			}
		}
		return nil
	})
	return found, err
}
