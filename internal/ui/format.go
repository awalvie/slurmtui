package ui

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func age(d time.Duration) string {
	if d < 0 {
		return "-" + age(-d)
	}
	seconds := int(d.Seconds())
	if seconds < 120 {
		return fmt.Sprintf("%ds", seconds)
	}
	minutes := int(d / time.Minute)
	if minutes < 10 {
		if s := seconds % 60; s != 0 {
			return fmt.Sprintf("%dm%ds", minutes, s)
		}
		return fmt.Sprintf("%dm", minutes)
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	hours := int(d / time.Hour)
	if hours < 8 {
		if m := minutes % 60; m != 0 {
			return fmt.Sprintf("%dh%dm", hours, m)
		}
		return fmt.Sprintf("%dh", hours)
	}
	if hours < 48 {
		return fmt.Sprintf("%dh", hours)
	}
	if hours < 24*8 {
		if h := hours % 24; h != 0 {
			return fmt.Sprintf("%dd%dh", hours/24, h)
		}
		return fmt.Sprintf("%dd", hours/24)
	}
	if hours < 24*365*2 {
		return fmt.Sprintf("%dd", hours/24)
	}
	if hours < 24*365*8 {
		if d := (hours / 24) % 365; d != 0 {
			return fmt.Sprintf("%dy%dd", hours/24/365, d)
		}
	}
	return fmt.Sprintf("%dy", hours/24/365)
}

func gib(mib int64) string {
	v := float64(mib) / 1024
	if v < 10 {
		return strconv.FormatFloat(v, 'f', 1, 64)
	}
	return strconv.FormatFloat(v, 'f', 0, 64)
}

func naturalCompare(a, b string) int {
	ar, br := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ar) && j < len(br) {
		if unicode.IsDigit(ar[i]) && unicode.IsDigit(br[j]) {
			si, sj := i, j
			for i < len(ar) && unicode.IsDigit(ar[i]) {
				i++
			}
			for j < len(br) && unicode.IsDigit(br[j]) {
				j++
			}
			na := strings.TrimLeft(string(ar[si:i]), "0")
			nb := strings.TrimLeft(string(br[sj:j]), "0")
			if c := cmp.Or(cmp.Compare(len(na), len(nb)), strings.Compare(na, nb)); c != 0 {
				return c
			}
			continue
		}
		if c := cmp.Compare(ar[i], br[j]); c != 0 {
			return c
		}
		i++
		j++
	}
	return cmp.Compare(len(ar)-i, len(br)-j)
}
