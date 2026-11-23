// Package postsubmit reports on OKD SCOS postsubmit image jobs
// (branch-ci-<org>-<repo>-<branch>-okd-scos-images) and on how fresh the
// images in the latest accepted OKD payloads are.
package postsubmit

import (
	"net/http"
	"time"
)

const (
	ReleaseControllerURL = "https://amd64.origin.releases.ci.openshift.org"
	ProwURL              = "https://prow.ci.openshift.org"
	GCSBucket            = "test-platform-results-public"
	GCSBaseURL           = "https://storage.googleapis.com/" + GCSBucket
	// GCSRetention is roughly how long Prow job results are kept in GCS.
	GCSRetention = 60 * 24 * time.Hour
)

// DefaultStreams are the OKD SCOS CI release streams to report on.
var DefaultStreams = []string{"5.0.0-0.okd-scos", "5.1.0-0.okd-scos"}

var httpClient = &http.Client{Timeout: 60 * time.Second}

// Flag values attached to an ImageRow.
const (
	FlagFailing = "failing" // latest postsubmit run failed: image not updated
	FlagPending = "pending" // job's latest successful run built a newer image than the payload's
	FlagStale   = "stale"   // image built longer ago than --stale-after
	FlagNoRun   = "no-run"  // no job results in GCS (no run in ~60 days, or never)
)

// AllFlags in display order.
var AllFlags = []string{FlagFailing, FlagPending, FlagStale, FlagNoRun}

// Options controls report generation.
type Options struct {
	Streams    []string
	Window     time.Duration
	StaleAfter time.Duration
	// PayloadStaleAfter warns when a stream's latest accepted payload is older than this.
	PayloadStaleAfter time.Duration
	Concurrency       int
}

// Run is one execution of an OKD postsubmit job, from Prow's live job list.
type Run struct {
	Job       string    `json:"job"`
	Org       string    `json:"org"`
	Repo      string    `json:"repo"`
	Branch    string    `json:"branch"`
	SHA       string    `json:"sha,omitempty"`
	State     string    `json:"state"`
	Started   time.Time `json:"started"`
	Completed time.Time `json:"completed,omitempty"`
	URL       string    `json:"url,omitempty"`
	// Streams whose latest accepted payload uses this job (e.g. ["5.0"]).
	// Empty if no current payload image is built by this job.
	Streams []string `json:"streams,omitempty"`
}

// StreamActivity summarises postsubmit activity for one stream.
type StreamActivity struct {
	Version        string         `json:"version"`
	RunsInWindow   int            `json:"runs_in_window"`
	FailedInWindow int            `json:"failed_in_window"`
	LastRun        *LastStreamRun `json:"last_run,omitempty"`
}

// LastStreamRun is the most recent finished run of any job feeding a stream.
type LastStreamRun struct {
	Repo     string    `json:"repo"`
	Branch   string    `json:"branch"`
	Job      string    `json:"job"`
	Result   string    `json:"result"`
	Finished time.Time `json:"finished"`
	URL      string    `json:"url"`
}

// JobResult is the latest finished run of a job, from GCS.
type JobResult struct {
	Found    bool      `json:"found"`
	BuildID  string    `json:"build_id,omitempty"`
	Result   string    `json:"result,omitempty"`
	Finished time.Time `json:"finished,omitempty"`
	Revision string    `json:"revision,omitempty"`
	URL      string    `json:"url,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// ImageRow describes one image of a payload.
type ImageRow struct {
	Name          string    `json:"name"`
	Org           string    `json:"org,omitempty"`
	Repo          string    `json:"repo,omitempty"`
	Branch        string    `json:"branch,omitempty"`       // branch whose job promotes to this stream
	BuiltBranch   string    `json:"built_branch,omitempty"` // branch recorded on the payload image
	Job           string    `json:"job,omitempty"`
	JobNote       string    `json:"job_note,omitempty"`
	JobHistoryURL string    `json:"job_history_url,omitempty"`
	PayloadCommit string    `json:"payload_commit,omitempty"`
	Image         string    `json:"image"` // digest-pinned image in the payload
	BuiltAt       time.Time `json:"built_at,omitempty"`
	BuiltCommit   string    `json:"built_commit,omitempty"`
	ImageError    string    `json:"image_error,omitempty"`
	LatestRun     JobResult `json:"latest_run"`
	Flags         []string  `json:"flags,omitempty"`
}

// HasFlag reports whether the row carries flag f.
func (r ImageRow) HasFlag(f string) bool {
	for _, x := range r.Flags {
		if x == f {
			return true
		}
	}
	return false
}

// StreamState is the image table for one release stream.
type StreamState struct {
	Stream       string `json:"stream"`
	Version      string `json:"version"`
	Payload      string `json:"payload,omitempty"`
	PayloadPhase string `json:"payload_phase,omitempty"`
	PayloadURL   string `json:"payload_url,omitempty"`
	// PayloadCreated is parsed from the payload name (…-YYYY-MM-DD-HHMMSS).
	PayloadCreated time.Time `json:"payload_created,omitempty"`
	// PayloadStale: latest accepted payload is older than --payload-stale-after,
	// so newer builds are piling up ("pending") behind it.
	PayloadStale bool           `json:"payload_stale,omitempty"`
	PullSpec     string         `json:"pull_spec,omitempty"`
	Images       []ImageRow     `json:"images"`
	Counts       map[string]int `json:"counts"`
	Error        string         `json:"error,omitempty"`
}

// Report is the full output.
type Report struct {
	Generated         time.Time     `json:"generated"`
	Window            time.Duration `json:"window_ns"`
	StaleAfter        time.Duration `json:"stale_after_ns"`
	PayloadStaleAfter time.Duration `json:"payload_stale_after_ns"`
	Runs              []Run         `json:"runs"`
	RunsError         string        `json:"runs_error,omitempty"`
	// RegistryError is set when image build times could not be read
	// (quay-proxy needs credentials). Image columns are then empty.
	RegistryError string           `json:"registry_error,omitempty"`
	Streams       []StreamState    `json:"streams"`
	Activity      []StreamActivity `json:"activity"`
}

// RunCounts tallies runs by state.
func (r *Report) RunCounts() map[string]int {
	c := map[string]int{}
	for _, run := range r.Runs {
		c[run.State]++
	}
	return c
}

// FailedRuns returns failed or errored runs within the window.
func (r *Report) FailedRuns() []Run {
	var out []Run
	for _, run := range r.Runs {
		if isFailState(run.State) {
			out = append(out, run)
		}
	}
	return out
}

func isFailState(s string) bool {
	switch s {
	case "failure", "error", "aborted", "FAILURE", "ERROR", "ABORTED":
		return true
	}
	return false
}
