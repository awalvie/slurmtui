package auth

import (
	"fmt"
	"io"
)

const Redacted = "[redacted]"

type Token string

func (Token) String() string { return Redacted }

func (Token) GoString() string { return Redacted }

func (Token) Format(f fmt.State, _ rune) { writeRedacted(f) }

func (Token) MarshalText() ([]byte, error) { return []byte(Redacted), nil }

func (Key) String() string { return Redacted }

func (Key) GoString() string { return Redacted }

func (Key) Format(f fmt.State, _ rune) { writeRedacted(f) }

func (Key) MarshalText() ([]byte, error) { return []byte(Redacted), nil }

func (s Signed) Format(f fmt.State, _ rune) {
	if _, err := fmt.Fprintf(f, "{user:%s key:%s}", s.user, Redacted); err != nil {
		panic(err)
	}
}

func (p Provided) Format(f fmt.State, _ rune) {
	if _, err := fmt.Fprintf(f, "{env:%s user:%s token:%s}", p.env, p.user, Redacted); err != nil {
		panic(err)
	}
}

func writeRedacted(w io.Writer) {
	if _, err := io.WriteString(w, Redacted); err != nil {
		panic(err)
	}
}
