package hostlist_test

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/awalvie/slurmtui/internal/hostlist"
)

func TestExpand(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"n01", []string{"n01"}},
		{"n01,n02", []string{"n01", "n02"}},
		{"n[1-3]", []string{"n1", "n2", "n3"}},
		{"n[1-3,7]", []string{"n1", "n2", "n3", "n7"}},
		{"n[01-03]", []string{"n01", "n02", "n03"}},
		{"n[08-10]", []string{"n08", "n09", "n10"}},
		{"n[1-10]", []string{"n1", "n2", "n3", "n4", "n5", "n6", "n7", "n8", "n9", "n10"}},
		{"n[001-002]", []string{"n001", "n002"}},
		{"r[1-2]n[1-3]", []string{"r1n1", "r1n2", "r1n3", "r2n1", "r2n2", "r2n3"}},
		{"n[1-2]-ib", []string{"n1-ib", "n2-ib"}},
		{"gpu[1-2],cpu[3-4]", []string{"gpu1", "gpu2", "cpu3", "cpu4"}},
		{"n1 n2", []string{"n1", "n2"}},
		{"n[5]", []string{"n5"}},
		{"", []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := hostlist.Expand(tc.in)
			if err != nil {
				t.Fatalf("Expand(%q) error: %s", tc.in, err)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("Expand(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestExpandRejectsMalformedInput(t *testing.T) {
	cases := []string{
		"n[1-3",
		"n1-3]",
		"n[[1-2]]",
		"n[3-1]",
		"n[-1-2]",
		"n[]",
		"n[1-]",
		"n[a-b]",
		"n[1,,2]",
		"n[0-65536]",
	}
	for _, in := range cases {
		t.Run(in, func(t *testing.T) {
			got, err := hostlist.Expand(in)
			if err == nil {
				t.Fatalf("Expand(%q) = %v, want an error", in, got)
			}
			if !strings.Contains(err.Error(), in) {
				t.Fatalf("error %q does not contain the input %q", err, in)
			}
		})
	}
}

func TestExpandRejectsTooManyHosts(t *testing.T) {
	in := "r[1-65536]n[1-65536]"
	_, err := hostlist.Expand(in)
	if err == nil || !strings.Contains(err.Error(), "more than 1048576 hosts") {
		t.Fatalf("Expand(%q) error = %v, want a host limit error", in, err)
	}
}

func TestExpandRejectsTooManyRanges(t *testing.T) {
	parts := make([]string, 262145)
	for i := range parts {
		parts[i] = strconv.Itoa(i * 2)
	}
	in := "n[" + strings.Join(parts, ",") + "]"
	_, err := hostlist.Expand(in)
	if err == nil || !strings.Contains(err.Error(), "more than 262144 ranges") {
		t.Fatalf("Expand error = %v, want a range limit error", err)
	}
}

func TestExpandSplitsOnTabsAndNewlines(t *testing.T) {
	got, err := hostlist.Expand("n1\tn2\nn3")
	if err != nil || !slices.Equal(got, []string{"n1", "n2", "n3"}) {
		t.Fatalf("Expand = %v, %v", got, err)
	}
}

func TestExpandAcceptsLargestRange(t *testing.T) {
	got, err := hostlist.Expand("n[1-65536]")
	if err != nil {
		t.Fatalf("Expand error: %s", err)
	}
	if len(got) != 65536 || got[65535] != "n65536" {
		t.Fatalf("got %d hosts, last %q", len(got), got[len(got)-1])
	}
}
