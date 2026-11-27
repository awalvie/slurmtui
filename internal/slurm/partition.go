package slurm

import (
	"context"
	"encoding/json"
	"fmt"
)

type Partition struct {
	Name        string
	DefaultTime TimeLimit
	MaxTime     TimeLimit
}

func (p Partition) JobTimeLimit() TimeLimit {
	switch {
	case p.DefaultTime.Set || p.DefaultTime.Unlimited:
		return p.DefaultTime
	case p.MaxTime.Set:
		return p.MaxTime
	}
	return TimeLimit{Unlimited: true}
}

type wirePartition struct {
	Name     string `json:"name"`
	Defaults struct {
		Time noVal `json:"time"`
	} `json:"defaults"`
	Maximums struct {
		Time noVal `json:"time"`
	} `json:"maximums"`
}

type wirePartitions struct {
	envelope
	Partitions []json.RawMessage `json:"partitions"`
}

func timeLimitOf(v noVal) TimeLimit {
	return TimeLimit{Minutes: v.Number, Set: v.Set, Unlimited: v.Infinite}
}

const partitionsPath = "/partitions?flags=ALL"

func (c *Client) Partitions(ctx context.Context) ([]Partition, error) {
	var wire wirePartitions
	if err := c.get(ctx, partitionsPath, &wire); err != nil {
		return nil, err
	}
	out := make([]Partition, 0, len(wire.Partitions))
	for _, raw := range wire.Partitions {
		var w wirePartition
		if err := decodeRecord(raw, &w); err != nil {
			return nil, fmt.Errorf("GET %s: partition %q: %w", c.url(partitionsPath), w.Name, err)
		}
		out = append(out, Partition{Name: w.Name, DefaultTime: timeLimitOf(w.Defaults.Time), MaxTime: timeLimitOf(w.Maximums.Time)})
	}
	return out, nil
}
