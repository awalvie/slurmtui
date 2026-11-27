package fakeslurm

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/awalvie/slurmtui/internal/clock"
)

type Claims struct {
	Sun string `json:"sun"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

type signedTokens struct {
	key   []byte
	clock clock.Clock
}

func (s *Server) RequireSignedTokens(key []byte, clk clock.Clock) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signed = &signedTokens{key: key, clock: clk}
}

func (s *Server) SignedClaims() []Claims {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Claims(nil), s.claims...)
}

func (v *signedTokens) verify(token, user string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, errors.New("not a JWT")
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := decodePart(parts[0], &header); err != nil {
		return Claims{}, err
	}
	if header.Alg != "HS256" {
		return Claims{}, errors.New("alg is not HS256")
	}
	mac := hmac.New(sha256.New, v.key)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return Claims{}, errors.New("bad signature")
	}
	var c Claims
	if err := decodePart(parts[1], &c); err != nil {
		return Claims{}, err
	}
	if c.Exp < v.clock.Now().Unix() {
		return Claims{}, errors.New("expired")
	}
	if c.Sun != user && c.Sun != "root" {
		return Claims{}, errors.New("sun is not the user header")
	}
	return c, nil
}

func decodePart(part string, into any) error {
	b, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, into)
}

func (s *Server) checkSignedToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		signed := s.signed
		s.mu.Unlock()
		if signed == nil {
			next.ServeHTTP(w, r)
			return
		}
		c, err := signed.verify(r.Header.Get("X-SLURM-USER-TOKEN"), r.Header.Get("X-SLURM-USER-NAME"))
		if err != nil && strings.HasSuffix(r.URL.Path, "/ping") {
			s.rejectPing(w)
			return
		}
		if err != nil {
			s.rejectToken(w)
			return
		}
		s.mu.Lock()
		s.claims = append(s.claims, c)
		s.mu.Unlock()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rejectPing(w http.ResponseWriter) {
	s.mu.Lock()
	body := map[string]any{
		"pings": []map[string]any{{
			"hostname":   "ctld",
			"responding": false,
			"latency":    10727,
			"primary":    true,
			"status":     "Protocol authentication error",
		}},
		"errors":   []any{},
		"warnings": []any{},
		"meta":     s.meta(),
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, body)
}

func (s *Server) rejectToken(w http.ResponseWriter) {
	s.mu.Lock()
	body := map[string]any{
		"errors": []map[string]any{{
			"description":  "Failure to query nodes",
			"error_number": 1007,
			"error":        "Protocol authentication error",
			"source":       "_dump_nodes",
		}},
		"warnings": []any{},
		"meta":     s.meta(),
	}
	s.mu.Unlock()
	writeJSON(w, http.StatusNetworkAuthenticationRequired, body)
}
