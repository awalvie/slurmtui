package slurm

import (
	"context"
	"fmt"
	"strings"
)

const pingAuthenticationError = "Protocol authentication error"

type wirePing struct {
	Hostname   string `json:"hostname"`
	Responding bool   `json:"responding"`
	Status     string `json:"status"`
}

type wirePings struct {
	envelope
	Pings []wirePing `json:"pings"`
}

func (c *Client) Ping(ctx context.Context) error {
	var wire wirePings
	if err := c.get(ctx, "/ping", &wire); err != nil {
		return err
	}
	url := c.url("/ping")
	failures := make([]string, 0, len(wire.Pings))
	for _, p := range wire.Pings {
		if p.Responding {
			return nil
		}
		if why, ok := c.explainRefusal(); ok && p.Status == pingAuthenticationError {
			return fmt.Errorf("GET %s: %s", url, why)
		}
		failures = append(failures, p.Hostname+": "+p.Status)
	}
	if len(failures) == 0 {
		return fmt.Errorf("GET %s: the reply lists no slurmctld", url)
	}
	return fmt.Errorf("GET %s: no slurmctld responded (%s)", url, strings.Join(failures, "; "))
}
