package auth_test

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/auth"
	"github.com/awalvie/slurmtui/internal/clock"
)

const fakeKey = "fake-cluster-key-for-tests-only\n"

func TestSecretsPrintRedacted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prod.key")
	if err := os.WriteFile(path, []byte(fakeKey), 0o400); err != nil {
		t.Fatal(err)
	}
	key, err := auth.ReadKeyFile(path)
	if err != nil {
		t.Fatal(err)
	}
	signed := auth.NewSigned("alice", key, clock.NewFake(time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)))
	creds, err := signed.Next()
	if err != nil {
		t.Fatal(err)
	}
	token := string(creds.Token)
	provided, err := auth.NewProvided("SLURM_JWT", token, "alice", clock.NewFake(time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)))
	if err != nil {
		t.Fatal(err)
	}
	secrets := []string{fakeKey, strings.TrimSpace(fakeKey), hex.EncodeToString([]byte(fakeKey)), token, strings.Split(token, ".")[2]}
	values := map[string]any{"key": key, "token": creds.Token, "credentials": creds, "signed": signed, "signed value": *signed, "provided": provided, "provided value": *provided}

	for name, v := range values {
		for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X", "%d"} {
			out := fmt.Sprintf(verb, v)
			for _, s := range secrets {
				if strings.Contains(out, s) {
					t.Errorf("%s printed with %s shows a secret: %s", name, verb, out)
				}
			}
		}
		b, err := json.Marshal(v)
		if err == nil && containsAny(string(b), secrets) {
			t.Errorf("%s as JSON shows a secret: %s", name, b)
		}
	}
	if got := fmt.Sprint(key); got != "[redacted]" {
		t.Errorf("key prints %q, want [redacted]", got)
	}
	if got := fmt.Sprint(creds.Token); got != "[redacted]" {
		t.Errorf("token prints %q, want [redacted]", got)
	}
}

func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
