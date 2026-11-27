package config

import (
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

var loopbackHosts = []string{"127.0.0.1", "::1", "localhost"}

func CheckURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Host == "" {
		return nil, errors.New("has no host")
	}
	switch u.Scheme {
	case "https":
		return u, nil
	case "http":
		if slices.Contains(loopbackHosts, u.Hostname()) {
			return u, nil
		}
		return nil, fmt.Errorf("http is allowed only to a loopback host (%s); use https for %s", strings.Join(loopbackHosts, ", "), u.Hostname())
	}
	return nil, fmt.Errorf("scheme must be https or http, got %q", u.Scheme)
}

func resolvePath(dir, home, path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home == "" {
			return "", fmt.Errorf("%s: HOME is not set, so ~ cannot be expanded", path)
		}
		return filepath.Join(home, path[1:]), nil
	}
	if strings.HasPrefix(path, "~") {
		return "", fmt.Errorf("%s: only ~ and ~/ expand, not ~user", path)
	}
	if filepath.IsAbs(path) {
		return path, nil
	}
	return filepath.Join(dir, path), nil
}

func CAPool(path string) (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil {
		return nil, fmt.Errorf("system certificate roots: %w", err)
	}
	if path == "" {
		return pool, nil
	}
	pem, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("%s: no PEM certificate found", path)
	}
	return pool, nil
}
