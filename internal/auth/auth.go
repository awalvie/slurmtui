package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/awalvie/slurmtui/internal/clock"
)

const ExpiryLayout = "2006-01-02 15:04:05 -07:00"

type Credentials struct {
	User  string
	Token Token
}

type Source interface {
	Next() (Credentials, error)
	User() string
	Expires() time.Time
}

type ExpiredError struct {
	Env string
	At  time.Time
}

func (e ExpiredError) Error() string {
	return fmt.Sprintf("%s expired at %s; get a new token and restart", e.Env, e.At.Local().Format(ExpiryLayout))
}

type Provided struct {
	env     string
	token   Token
	user    string
	expires time.Time
	clock   clock.Clock
}

func NewProvided(env, token, wantUser string, clk clock.Clock) (*Provided, error) {
	claims, err := readClaims(token)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", env, err)
	}
	if claims.Sun == "" {
		return nil, fmt.Errorf("%s: the token has no sun claim", env)
	}
	if claims.Exp == 0 {
		return nil, fmt.Errorf("%s: the token has no exp claim", env)
	}
	if wantUser != "" && wantUser != claims.Sun {
		return nil, fmt.Errorf("user %q is not the same as the token's sun %q in %s", wantUser, claims.Sun, env)
	}
	return &Provided{env: env, token: Token(token), user: claims.Sun, expires: time.Unix(claims.Exp, 0), clock: clk}, nil
}

func (p *Provided) User() string { return p.user }

func (p *Provided) Expires() time.Time { return p.expires }

func (p *Provided) Next() (Credentials, error) {
	if !p.clock.Now().Before(p.expires) {
		return Credentials{}, ExpiredError{Env: p.env, At: p.expires}
	}
	return Credentials{User: p.user, Token: p.token}, nil
}

type claims struct {
	Sun string `json:"sun"`
	Exp int64  `json:"exp"`
}

func readClaims(token string) (claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims{}, errors.New("the token is not a JWT (it must have three parts separated by dots)")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return claims{}, payloadErrorWithoutTokenBytes("base64url")
	}
	var c claims
	if err := json.Unmarshal(payload, &c); err != nil {
		return claims{}, payloadErrorWithoutTokenBytes("a JSON object with sun and exp")
	}
	return c, nil
}

func payloadErrorWithoutTokenBytes(want string) error {
	return fmt.Errorf("the token payload is not %s", want)
}
