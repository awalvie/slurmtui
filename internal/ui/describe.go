package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/awalvie/slurmtui/internal/auth"
	"github.com/awalvie/slurmtui/internal/slurm"
)

func isDetail(v view) bool {
	return v == describeView || v == jsonView
}

func (m Model) openDetail(kind view) Model {
	return m.pushSelected(frame{kind: kind, of: m.current().kind})
}

func (m Model) scrollTop(total, room int) int {
	return max(min(m.cursors[m.top().id()].index, total-room), 0)
}

func (m Model) scrollKey(key string, total, room int) Model {
	top := m.scrollTop(total, room)
	switch key {
	case "j", "down":
		top++
	case "k", "up":
		top--
	case "g":
		top = 0
	case "G":
		top = total
	default:
		return m.unknownKey(key)
	}
	return m.setCursor(m.top(), cursorAt{index: max(min(top, total-room), 0)})
}

const boxBorders = 2

func (m Model) detailRoom() int {
	return max(m.bodyHeight()-boxBorders, 1)
}

func absolute(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format(auth.ExpiryLayout)
}

func (m Model) detailLines(f frame) []string {
	if f.of == jobsView {
		j, ok := m.job(f.scope)
		switch {
		case !ok:
			return []string{dimStyle.Render(fmt.Sprintf("job %s is no longer in the reply from slurmrestd", f.scope))}
		case f.kind == jsonView:
			return rawLines(j.Raw)
		}
		return describeJob(j, m.timeLeftText(j, m.clock.Now()), m.clock.Now())
	}
	n, ok := m.node(f.scope)
	switch {
	case !ok:
		return []string{dimStyle.Render(fmt.Sprintf("node %s is no longer in the reply from slurmrestd", f.scope))}
	case f.kind == jsonView:
		return rawLines(n.Raw)
	}
	return describeNode(n)
}

func rawLines(raw json.RawMessage) []string {
	var out bytes.Buffer
	if err := json.Indent(&out, raw, "", "  "); err != nil {
		return []string{badStyle.Render(fmt.Sprintf("format the raw JSON: %s", err))}
	}
	lines := strings.Split(out.String(), "\n")
	for i, l := range lines {
		lines[i] = colourJSON(l)
	}
	return lines
}

type field struct {
	label string
	value string
	style lipgloss.Style
	group bool
}

func value(label, v string) field {
	return field{label: label, value: v, style: textStyle}
}

func group(name string) field {
	return field{label: name, group: true}
}

const describeKeyWidth = 16

func fieldLines(fields []field) []string {
	lines := make([]string, 0, len(fields))
	for _, f := range fields {
		switch {
		case f.group:
			lines = append(lines, "", keyStyle.Render(f.label))
		case f.value == "":
			lines = append(lines, accentStyle.Render(fitLeft(f.label, describeKeyWidth))+dimStyle.Render("—"))
		default:
			lines = append(lines, accentStyle.Render(fitLeft(f.label, describeKeyWidth))+f.style.Render(f.value))
		}
	}
	return lines
}

func describeNode(n slurm.Node) []string {
	free := ""
	if n.HasFreeMem {
		free = strconv.FormatInt(n.FreeMem, 10)
	}
	class := nodeClasses[classOfNode(n.State)]
	return fieldLines([]field{
		value("Name", n.Name),
		{label: "State", value: class.cell(strings.Join(n.State.Raw, "+")), style: class.style},
		value("Reason", n.Reason),
		value("Reason by", n.ReasonSetBy),
		value("Reason at", absolute(n.ReasonChangedAt)),
		value("Partitions", strings.Join(n.Partitions, ",")),
		group("CPU"),
		value("Allocated", strconv.Itoa(n.AllocCPUs)),
		value("Total", strconv.Itoa(n.CPUs)),
		value("Specialized", n.SpecializedCPUs),
		value("Load", strconv.FormatFloat(float64(n.CPULoad)/100, 'f', 2, 64)),
		group("Memory"),
		value("Allocated MiB", strconv.FormatInt(n.AllocMemory, 10)),
		value("Total MiB", strconv.FormatInt(n.RealMemory, 10)),
		value("Free MiB", free),
		value("Features", strings.Join(n.Features, ",")),
		value("Gres", n.Gres),
		value("Reboot pending", strconv.FormatBool(n.State.Has("REBOOT_REQUESTED"))),
		group("Times"),
		value("Boot time", absolute(n.BootTime)),
		value("Last busy", absolute(n.LastBusy)),
	})
}

func ageIf(d time.Duration, ok bool) string {
	if !ok {
		return ""
	}
	return age(d)
}

func describeJob(j slurm.Job, timeLeft string, now time.Time) []string {
	class := jobClasses[classOfJob(j.State)]
	state := strings.Join(append([]string{j.State.Base}, j.State.Flags...), "+")
	if j.StateDescription != "" {
		state += " (" + j.StateDescription + ")"
	}
	reason := j.StateReason
	if reason == "None" {
		reason = ""
	}
	pending := j.State.Base == "PENDING"
	start, expected := absolute(j.StartTime), ""
	if pending {
		start, expected = "", absolute(j.ExpectedStart)
	}
	return fieldLines([]field{
		value("ID", j.ID()),
		value("Name", j.Name),
		value("User", j.User),
		value("Account", j.Account),
		value("QOS", j.QOS),
		{label: "State", value: class.cell(state), style: class.style},
		value("Reason", reason),
		value("Priority", strconv.FormatInt(j.Priority, 10)),
		value("Dependency", j.Dependency),
		value("Partition", j.Partition),
		value("Nodes", j.Nodes),
		value("Node count", strconv.FormatInt(j.NodeCount, 10)),
		value("Requested TRES", j.TRESRequested),
		group("Times"),
		value("Submit", absolute(j.SubmitTime)),
		value("Waiting", ageIf(j.Waited(now))),
		value("Expected start", expected),
		value("Start", start),
		value("End", absolute(j.EndTime)),
		value("Elapsed", ageIf(j.Elapsed(now))),
		value("Time limit", j.TimeLimit.Label()),
		value("Time left", timeLeft),
		group("Files"),
		value("Exit code", j.ExitCode.Label()),
		value("Work dir", j.WorkingDirectory),
		value("StdOut", j.StandardOutput),
		value("Command", j.Command),
	})
}

func (m Model) detailPane(f frame, width, height int) []string {
	name := "Describe"
	if f.kind == jsonView {
		name = "JSON"
	}
	lines := m.detailLines(f)
	top := m.scrollTop(len(lines), m.detailRoom())
	lines = lines[top:min(top+m.detailRoom(), len(lines))]
	for i, line := range lines {
		lines[i] = " " + line
	}
	title := titleStyle.Render(name) + dimStyle.Render(factSeparator) + textStyle.Render(f.scope)
	return box{title: title, width: width, height: height, focused: true, lines: lines}.render()
}
