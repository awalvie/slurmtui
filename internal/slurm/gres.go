package slurm

import (
	"fmt"
	"strconv"
	"strings"
)

type GPUs struct {
	Used       int64
	Configured int64
}

func (n Node) GPUs() (GPUs, bool, error) {
	configured, has, err := gpuCount(n.Gres)
	if err != nil {
		return GPUs{}, false, fmt.Errorf("node %s: gres %q: %w", n.Name, n.Gres, err)
	}
	used, _, err := gpuCount(n.GresUsed)
	if err != nil {
		return GPUs{}, false, fmt.Errorf("node %s: gres_used %q: %w", n.Name, n.GresUsed, err)
	}
	return GPUs{Used: used, Configured: configured}, has, nil
}

func gpuCount(gres string) (int64, bool, error) {
	entries, err := splitGres(gres)
	if err != nil {
		return 0, false, err
	}
	var total int64
	has := false
	for _, e := range entries {
		name, count, err := parseGresEntry(e)
		if err != nil {
			return 0, false, err
		}
		if name == "gpu" {
			total += count
			has = true
		}
	}
	return total, has, nil
}

func splitGres(gres string) ([]string, error) {
	if gres == "" {
		return nil, nil
	}
	var entries []string
	depth, start := 0, 0
	for i, r := range gres {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unmatched )")
			}
		case ',':
			if depth == 0 {
				entries = append(entries, gres[start:i])
				start = i + 1
			}
		}
	}
	if depth != 0 {
		return nil, fmt.Errorf("unmatched (")
	}
	return append(entries, gres[start:]), nil
}

func parseGresEntry(entry string) (string, int64, error) {
	spec := entry
	if open := strings.IndexByte(entry, '('); open >= 0 {
		if !strings.HasSuffix(entry, ")") {
			return "", 0, fmt.Errorf("entry %q: text after )", entry)
		}
		spec = entry[:open]
	}
	parts := strings.Split(spec, ":")
	if parts[0] == "" || len(parts) > 3 {
		return "", 0, fmt.Errorf("entry %q: not name[:type][:count]", entry)
	}
	last := parts[len(parts)-1]
	count, err := strconv.ParseInt(last, 10, 64)
	switch {
	case len(parts) == 1:
		return parts[0], 1, nil
	case err == nil && count >= 0:
		return parts[0], count, nil
	case len(parts) == 2 && last != "" && !startsWithDigit(last):
		return parts[0], 1, nil
	}
	return "", 0, fmt.Errorf("entry %q: count %q is not a whole number", entry, last)
}

func startsWithDigit(s string) bool {
	return s[0] >= '0' && s[0] <= '9'
}
