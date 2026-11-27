package hostlist

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	maxRange  = 65536
	maxRanges = 262144
	maxHosts  = 1048576
)

func Expand(expr string) ([]string, error) {
	tokens, err := split(expr)
	if err != nil {
		return nil, fmt.Errorf("hostlist %q: %w", expr, err)
	}
	if ranges := strings.Count(expr, ",") + 1; ranges > maxRanges {
		return nil, fmt.Errorf("hostlist %q: more than %d ranges", expr, maxRanges)
	}
	hosts := []string{}
	for _, tok := range tokens {
		names, err := expandToken(tok, maxHosts-len(hosts))
		if err != nil {
			return nil, fmt.Errorf("hostlist %q: %w", expr, err)
		}
		hosts = append(hosts, names...)
	}
	return hosts, nil
}

func split(expr string) ([]string, error) {
	var tokens []string
	depth := 0
	start := 0
	for i, r := range expr {
		switch r {
		case '[':
			if depth > 0 {
				return nil, fmt.Errorf("nested '[' at %d", i)
			}
			depth++
		case ']':
			if depth == 0 {
				return nil, fmt.Errorf("unmatched ']' at %d", i)
			}
			depth--
		case ',', ' ', '\t', '\n':
			if depth == 0 {
				if i > start {
					tokens = append(tokens, expr[start:i])
				}
				start = i + 1
			}
		}
	}
	if depth > 0 {
		return nil, fmt.Errorf("missing ']'")
	}
	if start < len(expr) {
		tokens = append(tokens, expr[start:])
	}
	return tokens, nil
}

func expandToken(tok string, room int) ([]string, error) {
	names := []string{""}
	rest := tok
	for rest != "" {
		open := strings.IndexByte(rest, '[')
		if open < 0 {
			names = appendAll(names, []string{rest})
			break
		}
		closing := strings.IndexByte(rest, ']')
		names = appendAll(names, []string{rest[:open]})
		values, err := expandRanges(rest[open+1 : closing])
		if err != nil {
			return nil, err
		}
		if len(names)*len(values) > room {
			return nil, fmt.Errorf("more than %d hosts", maxHosts)
		}
		names = appendAll(names, values)
		rest = rest[closing+1:]
	}
	if len(names) > room {
		return nil, fmt.Errorf("more than %d hosts", maxHosts)
	}
	return names, nil
}

func appendAll(prefixes, suffixes []string) []string {
	out := make([]string, 0, len(prefixes)*len(suffixes))
	for _, p := range prefixes {
		for _, s := range suffixes {
			out = append(out, p+s)
		}
	}
	return out
}

func expandRanges(body string) ([]string, error) {
	if body == "" {
		return nil, fmt.Errorf("empty range")
	}
	var out []string
	for part := range strings.SplitSeq(body, ",") {
		loText, hiText, isRange := strings.Cut(part, "-")
		if !isRange {
			hiText = loText
		}
		lo, err := parseNumber(loText)
		if err != nil {
			return nil, fmt.Errorf("range %q: %w", part, err)
		}
		hi, err := parseNumber(hiText)
		if err != nil {
			return nil, fmt.Errorf("range %q: %w", part, err)
		}
		if lo > hi {
			return nil, fmt.Errorf("range %q: low is larger than high", part)
		}
		if hi-lo+1 > maxRange {
			return nil, fmt.Errorf("range %q: more than %d hosts", part, maxRange)
		}
		width := len(loText)
		for n := lo; n <= hi; n++ {
			out = append(out, fmt.Sprintf("%0*d", width, n))
		}
	}
	return out, nil
}

func parseNumber(text string) (uint64, error) {
	if text == "" {
		return 0, fmt.Errorf("missing number")
	}
	n, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number", text)
	}
	return n, nil
}
