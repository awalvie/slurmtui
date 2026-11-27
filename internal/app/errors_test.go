package app_test

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/awalvie/slurmtui/internal/fakeslurm"
	uv "github.com/charmbracelet/ultraviolet"
)

func (r *run) errorPopup(title string) (dialogBox, string) {
	r.t.Helper()
	r.waitFor("┌ " + title + " ")
	b := r.dialogBox(title + " ")
	text := strings.Join(strings.Fields(strings.Join(r.dialogRows(b), " ")), " ")
	return b, text
}

func (r *run) waitPopupText(title, want string) {
	r.t.Helper()
	r.waitFor("┌ " + title + " ")
	r.waitUntil(title+" pop-up with "+want, func() bool {
		b := r.dialogBox(title + " ")
		squash := func(s string) string { return strings.Join(strings.Fields(s), "") }
		return strings.Contains(squash(strings.Join(r.dialogRows(b), "")), squash(want))
	})
}

func (r *run) checkErrorPopup(b dialogBox) {
	r.t.Helper()
	if c := r.cellAt(b.x, b.y); !isANSI(c.Style.Fg, ansiRed) {
		r.t.Fatalf("error pop-up border is not red: %+v", c.Style)
	}
	if c := r.cellAt(b.x+2, b.y); !isANSI(c.Style.Fg, ansiRed) || c.Style.Attrs&uv.AttrBold == 0 {
		r.t.Fatalf("error pop-up title is not red bold: %+v", c.Style)
	}
	r.waitButton("OK")
	r.checkMarginAround(b, 6)
	r.checkFadedUnder(b)
}

func TestRefusedActionOpensErrorPopup(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		keys    string
		subject string
		refuse  func(t *testing.T, cluster *fakeslurm.Server)
		want    string
		close   string
	}{
		{
			name:    "drain not admin",
			keys:    "D",
			subject: "Drain ghost3",
			refuse: func(t *testing.T, c *fakeslurm.Server) {
				c.RefuseNodeUpdates(http.StatusUnprocessableEntity, recordedReply(t, "node-drain-non-admin-reply.json"))
			},
			want:  "Failure to update node ghost3: Invalid user id",
			close: "\r",
		},
		{
			name:    "resume HTTP 200 with errors",
			keys:    "U",
			subject: "Resume ghost3",
			refuse: func(t *testing.T, c *fakeslurm.Server) {
				c.RefuseNodeUpdates(http.StatusOK, recordedReply(t, "node-resume-invalid-transition-reply.json"))
			},
			want:  "Failure to update node ghost3: Invalid node state transition",
			close: escKey,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("ghost1"), idle("ghost3")}})
			r := startSizedOn(t, cluster, 80, 24)
			r.waitFor("┌ Nodes 2 ")
			r.moveTo("ghost3")
			tc.refuse(t, cluster)

			r.send(tc.keys)
			if tc.keys == "D" {
				r.waitFor("▸ Reason")
				r.send("bad DIMM\r")
			} else {
				r.waitButton("Keep")
				r.send("\x1b[D\r")
			}

			b, text := r.errorPopup("Error")
			r.checkErrorPopup(b)
			if !strings.Contains(text, tc.subject) || !strings.Contains(text, tc.want) {
				t.Fatalf("pop-up text = %q, want %q and %q", text, tc.subject, tc.want)
			}
			if strings.Contains(r.screen(), "…") {
				t.Fatalf("pop-up cuts the message\n%s", r.screen())
			}
			if strings.Contains(r.screen(), "accepted") {
				t.Fatalf("refused action shows as accepted\n%s", r.screen())
			}

			r.send(tc.close)
			r.waitGone("┌ Error ")
			r.send("j")
			r.waitSelected("ghost3")
		})
	}
}

func TestRefusedCancelOpensErrorPopup(t *testing.T) {
	t.Parallel()
	cases := []struct{ reply, want string }{
		{"job-cancel-not-owner-reply.json", "Access/permission denied"},
		{"job-cancel-finished-reply.json", "Job/step already completing or completed"},
		{"job-cancel-unknown-id-reply.json", "Invalid job id specified"},
	}
	for _, tc := range cases {
		t.Run(tc.reply, func(t *testing.T) {
			r, cluster := startOnJobs(t, []fakeslurm.Job{runningJob(1234)})
			cluster.RefuseJobCancels(recordedReply(t, tc.reply))
			r.waitSelected("1234")

			r.send(ctrlD)
			r.confirmCancel()

			_, text := r.errorPopup("Error")
			if !strings.Contains(text, "Cancel job 1234") || !strings.Contains(text, tc.want) {
				t.Fatalf("pop-up text = %q, want the job and %q", text, tc.want)
			}
			r.send(escKey)
			r.waitGone("┌ Error ")
		})
	}
}

func TestRefreshErrorOpensPopupOnce(t *testing.T) {
	t.Parallel()
	cluster := fakeslurm.Start(t, fakeslurm.Cluster{Name: "lamp", Nodes: []fakeslurm.Node{idle("n1")}})
	r := startProvided(t, cluster)
	r.waitFor("┌ Nodes 1 ")

	cluster.SetStatus(http.StatusInternalServerError)
	r.advance(5 * time.Second)

	b, text := r.errorPopup("Refresh failed")
	r.checkErrorPopup(b)
	if !strings.Contains(text, cluster.URL) || !strings.Contains(text, "/slurm/v0.0.45/nodes: HTTP 500") {
		t.Fatalf("pop-up text = %q, want the URL and the error", text)
	}
	for _, secret := range []string{aliceToken, "X-SLURM-USER-TOKEN"} {
		if strings.Contains(r.screen(), secret) {
			t.Fatalf("screen shows %q", secret)
		}
	}

	before := nodeRequests(cluster)
	r.advance(5 * time.Second)
	r.waitUntil("refresh under the pop-up", func() bool { return nodeRequests(cluster) > before })
	r.waitFor("○ stale 10s")

	r.send("\r")
	r.waitGone("┌ Refresh failed ")
	r.advance(5 * time.Second)
	r.waitFor("○ stale 15s")
	r.waitFor("HTTP 500")
	if strings.Contains(r.screen(), "┌ Refresh failed ") {
		t.Fatalf("the same error opened the pop-up again\n%s", r.screen())
	}

	cluster.SetStatus(http.StatusUnauthorized)
	r.advance(5 * time.Second)
	_, text = r.errorPopup("Refresh failed")
	if !strings.Contains(text, "HTTP 401") {
		t.Fatalf("pop-up text = %q, want the new error", text)
	}
	r.send(escKey)
	r.waitGone("┌ Refresh failed ")

	cluster.SetStatus(http.StatusOK)
	r.advance(5 * time.Second)
	r.waitFor("● 0s")
}
