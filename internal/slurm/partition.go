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
	return decodeEach(c.url(partitionsPath), wire.Partitions, func(w wirePartition) string { return fmt.Sprintf("partition %q", w.Name) }, wirePartition.partition)
}

func (w wirePartition) partition(json.RawMessage) (Partition, error) {
	return Partition{Name: w.Name, DefaultTime: timeLimitOf(w.Defaults.Time), MaxTime: timeLimitOf(w.Maximums.Time)}, nil
}
