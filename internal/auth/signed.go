package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/awalvie/slurmtui/internal/clock"
)

const SignedLifetime = 5 * time.Minute

var hs256Header = base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))

type Key struct {
	bytes []byte
}

func ReadKeyFile(path string) (Key, error) {
	key, err := readKeyFile(path)
	if err != nil {
		return Key{}, fmt.Errorf("key file %s: %w", path, err)
	}
	return key, nil
}

func readKeyFile(path string) (Key, error) {
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return Key{}, err
	}
	key, err := readOpenKeyFile(f)
	return key, errors.Join(err, f.Close())
}

func readOpenKeyFile(f *os.File) (Key, error) {
	info, err := f.Stat()
	if err != nil {
		return Key{}, err
	}
	if !info.Mode().IsRegular() {
		return Key{}, fmt.Errorf("is not a regular file (%s)", info.Mode().Type())
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return Key{}, errors.New("cannot read the file owner")
	}
	if uid := os.Getuid(); int(stat.Uid) != uid {
		return Key{}, fmt.Errorf("is owned by uid %d, not by the current user (uid %d)", stat.Uid, uid)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		return Key{}, fmt.Errorf("mode %04o gives access to group or others; run chmod 400 on it", perm)
	}
	if info.Size() == 0 {
		return Key{}, errors.New("is empty")
	}
	b := make([]byte, info.Size())
	if _, err := f.ReadAt(b, 0); err != nil {
		return Key{}, err
	}
	return Key{bytes: b}, nil
}

type Signed struct {
	user  string
	key   Key
	clock clock.Clock
}

func NewSigned(user string, key Key, clk clock.Clock) *Signed {
	return &Signed{user: user, key: key, clock: clk}
}

func (s *Signed) User() string { return s.user }

func (s *Signed) Expires() time.Time { return time.Time{} }

func (s *Signed) ExplainRefusal() string {
	return "the cluster refused the signed token; check that this computer's clock is not slow, and that keyFile holds the cluster key"
}

func (s *Signed) Next() (Credentials, error) {
	now := s.clock.Now()
	claims, err := json.Marshal(map[string]any{"sun": s.user, "iat": now.Unix(), "exp": now.Add(SignedLifetime).Unix()})
	if err != nil {
		return Credentials{}, err
	}
	signing := hs256Header + "." + base64.RawURLEncoding.EncodeToString(claims)
	mac := hmac.New(sha256.New, s.key.bytes)
	mac.Write([]byte(signing))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return Credentials{User: s.user, Token: Token(signing + "." + sig)}, nil
}
