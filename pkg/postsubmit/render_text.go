package postsubmit

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// JSON renders the report as indented JSON.
func (r *Report) JSON() string {
	b, _ := json.MarshalIndent(r, "", "  ")
	return string(b)
}

func shortSHA(s string) string {
	if len(s) > 7 {
		return s[:7]
	}
	return s
}

func age(t time.Time, now time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return FormatAge(now.Sub(t))
}

func runLine(run Run) string {
	return fmt.Sprintf("%s %s@%s (%s)", run.State, run.Repo, run.Branch, run.Started.Format("01-02 15:04Z"))
}

// String renders a terminal report.
func (r *Report) String() string {
	var b strings.Builder
	now := r.Generated
	fmt.Fprintf(&b, "OKD Postsubmit Report — %s (window: %s)\n\n", now.Format("2006-01-02 15:04 UTC"), FormatAge(r.Window))

	c := r.RunCounts()
	fmt.Fprintf(&b, "▶ Runs in last %s: %d total · %d passed · %d failed · %d running\n",
		FormatAge(r.Window), len(r.Runs), c["success"], len(r.FailedRuns()), c["pending"]+c["triggered"])
	if r.RunsError != "" {
		fmt.Fprintf(&b, "  ⚠ could not fetch runs: %s\n", r.RunsError)
	}
	for _, a := range r.Activity {
		fmt.Fprintf(&b, "  %s\n", a.line(r.Window, now))
	}
	for _, run := range r.Runs {
		icon := "🟢"
		switch {
		case isFailState(run.State):
			icon = "🔴"
		case run.State != "success":
			icon = "🔵"
		}
		fmt.Fprintf(&b, "  %s %-9s %-5s %-60s %s %s\n    %s\n", icon, run.State, streamsLabel(run), run.Job, run.Started.Format("01-02 15:04Z"), shortSHA(run.SHA), run.URL)
	}

	for _, s := range r.Streams {
		fmt.Fprintf(&b, "\n▶ Stream %s (%s)", s.Version, s.Stream)
		if s.Error != "" {
			fmt.Fprintf(&b, "\n  ⚠ %s\n", s.Error)
			continue
		}
		fmt.Fprintf(&b, " — %d images, payload %s\n", len(s.Images), s.Payload)
		if s.PayloadStale {
			fmt.Fprintf(&b, "  ⚠ no accepted payload for %s: newer builds are waiting (pending) until one is accepted\n", age(s.PayloadCreated, now))
		}
		fmt.Fprintf(&b, "  failing: %d · pending: %d · stale: %d · no run in 60d: %d\n",
			s.Counts[FlagFailing], s.Counts[FlagPending], s.Counts[FlagStale], s.Counts[FlagNoRun])
		fmt.Fprintf(&b, "  %-45s %-8s %-8s %-10s %-9s %s\n", "IMAGE", "BUILT", "PAYLOAD", "JOB", "LAST RUN", "FLAGS")
		for _, img := range s.Images {
			res, last := "-", "-"
			if img.LatestRun.Found {
				res, last = img.LatestRun.Result, age(img.LatestRun.Finished, now)
			} else if img.Job != "" {
				res = "none(60d)"
			}
			fmt.Fprintf(&b, "  %-45s %-8s %-8s %-10s %-9s %s\n", img.Name, age(img.BuiltAt, now), shortSHA(img.PayloadCommit), res, last, strings.Join(img.Flags, ","))
		}
	}
	if r.RegistryError != "" {
		fmt.Fprintf(&b, "\n⚠ image build times unavailable: %s\n", r.RegistryError)
	}
	return b.String()
}

// Slack renders a short mrkdwn summary suitable for a webhook post.
func (r *Report) Slack(reportURL string) string {
	var b strings.Builder
	now := r.Generated
	fails := r.FailedRuns()
	c := r.RunCounts()
	fmt.Fprintf(&b, "*OKD Postsubmit Report* — %s\n", now.Format("2006-01-02 15:04 UTC"))
	fmt.Fprintf(&b, "Last %s: %d runs · ✅ %d passed · ❌ %d failed · 🔵 %d running\n",
		FormatAge(r.Window), len(r.Runs), c["success"], len(fails), c["pending"]+c["triggered"])
	for _, run := range fails {
		fmt.Fprintf(&b, "• ❌ [%s] <%s|%s>\n", streamsLabel(run), run.URL, runLine(run))
	}
	for _, a := range r.Activity {
		fmt.Fprintf(&b, "    %s\n", a.line(r.Window, now))
	}
	for _, s := range r.Streams {
		if s.Error != "" {
			fmt.Fprintf(&b, "*%s*: ⚠ %s\n", s.Version, s.Error)
			continue
		}
		fmt.Fprintf(&b, "*%s* (<%s|%s>): %d images · 🔴 %d failing · 🟡 %d pending · ⚠️ %d stale · ⚪ %d no run in 60d\n",
			s.Version, s.PayloadURL, s.Payload, len(s.Images), s.Counts[FlagFailing], s.Counts[FlagPending], s.Counts[FlagStale], s.Counts[FlagNoRun])
		if s.PayloadStale {
			fmt.Fprintf(&b, "  • ⚠️ no accepted %s payload for %s — %d newer builds waiting\n", s.Version, age(s.PayloadCreated, now), s.Counts[FlagPending])
		}
		for _, img := range s.Images {
			if img.HasFlag(FlagFailing) {
				fmt.Fprintf(&b, "  • 🔴 <%s|%s> — latest run failed %s ago, image built %s ago\n",
					img.LatestRun.URL, img.Name, age(img.LatestRun.Finished, now), age(img.BuiltAt, now))
			}
		}
	}
	if reportURL != "" {
		fmt.Fprintf(&b, "<%s|Full report>\n", reportURL)
	}
	return b.String()
}

func streamsLabel(run Run) string {
	if len(run.Streams) == 0 {
		return "-"
	}
	return strings.Join(run.Streams, ",")
}

// line renders a one-line activity summary for a stream.
func (a StreamActivity) line(window time.Duration, now time.Time) string {
	var s string
	if a.RunsInWindow == 0 {
		s = fmt.Sprintf("%s: no runs in the last %s", a.Version, FormatAge(window))
	} else {
		plural := "s"
		if a.RunsInWindow == 1 {
			plural = ""
		}
		s = fmt.Sprintf("%s: %d run%s in window", a.Version, a.RunsInWindow, plural)
		if a.FailedInWindow > 0 {
			s += fmt.Sprintf(", %d failed", a.FailedInWindow)
		}
	}
	if a.LastRun != nil {
		s += fmt.Sprintf(" · most recent: %s@%s %s ago, %s", a.LastRun.Repo, a.LastRun.Branch, age(a.LastRun.Finished, now), a.LastRun.Result)
	}
	return s
}
