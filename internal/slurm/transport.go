package slurm

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"time"
)

func NewHTTPClient(roots *x509.CertPool) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			TLSClientConfig:       &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          10,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func redirectRefused(resp *http.Response) error {
	target, err := resp.Location()
	if err != nil {
		return fmt.Errorf("HTTP %d: redirect refused: %w", resp.StatusCode, err)
	}
	return fmt.Errorf("HTTP %d: redirect to %s refused", resp.StatusCode, target.Redacted())
}
