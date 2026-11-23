package postsubmit

import (
	"sort"
	"strconv"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// Generate builds the full report.
func Generate(o *Options) (*Report, error) {
	if o.Concurrency <= 0 {
		o.Concurrency = 20
	}
	now := time.Now().UTC()
	rep := &Report{Generated: now, Window: o.Window, StaleAfter: o.StaleAfter, PayloadStaleAfter: o.PayloadStaleAfter}

	reg, err := newRegistry()
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		runs, err := FetchRecentRuns(o.Window, now)
		if err != nil {
			klog.Errorf("recent runs: %v", err)
			rep.RunsError = err.Error()
			return
		}
		rep.Runs = runs
	}()

	jobs := newJobCache()
	cfgs := newCIConfigs()
	rep.Streams = make([]StreamState, len(o.Streams))
	for i, s := range o.Streams {
		rep.Streams[i] = buildStream(s, o, now, reg, jobs, cfgs)
	}
	wg.Wait()
	annotateStreams(rep)
	if reg.unauthorized.Load() {
		rep.RegistryError = ErrRegistryUnauthorized.Error()
	}
	return rep, nil
}

// annotateStreams tags each recent run with the stream(s) whose latest payload
// uses that job, and builds a per-stream activity summary for the window.
func annotateStreams(rep *Report) {
	jobStreams := map[string][]string{}
	for _, s := range rep.Streams {
		seen := map[string]bool{}
		for _, img := range s.Images {
			if img.Job != "" && !seen[img.Job] {
				seen[img.Job] = true
				jobStreams[img.Job] = append(jobStreams[img.Job], s.Version)
			}
		}
	}
	for i := range rep.Runs {
		rep.Runs[i].Streams = jobStreams[rep.Runs[i].Job]
	}

	for _, s := range rep.Streams {
		a := StreamActivity{Version: s.Version}
		for _, run := range rep.Runs {
			for _, v := range run.Streams {
				if v == s.Version {
					a.RunsInWindow++
					if isFailState(run.State) {
						a.FailedInWindow++
					}
				}
			}
		}
		// Most recent finished run of any job feeding this stream (from GCS).
		for _, img := range s.Images {
			lr := img.LatestRun
			if !lr.Found || lr.Finished.IsZero() {
				continue
			}
			if a.LastRun == nil || lr.Finished.After(a.LastRun.Finished) {
				a.LastRun = &LastStreamRun{Repo: img.Repo, Branch: img.Branch, Job: img.Job,
					Result: lr.Result, Finished: lr.Finished, URL: lr.URL}
			}
		}
		rep.Activity = append(rep.Activity, a)
	}
}

// jobCache dedupes GCS lookups across images that share a job.
type jobCache struct {
	mu sync.Mutex
	m  map[string]*jobEntry
}

type jobEntry struct {
	once sync.Once
	res  JobResult
}

func newJobCache() *jobCache { return &jobCache{m: map[string]*jobEntry{}} }

func (c *jobCache) get(job string) JobResult {
	c.mu.Lock()
	e, ok := c.m[job]
	if !ok {
		e = &jobEntry{}
		c.m[job] = e
	}
	c.mu.Unlock()
	e.once.Do(func() { e.res = LatestResult(job) })
	return e.res
}

func buildStream(stream string, o *Options, now time.Time, reg *registry, jobs *jobCache, cfgs *ciConfigs) StreamState {
	ss := StreamState{Stream: stream, Version: StreamVersion(stream), Counts: map[string]int{}}

	p, err := FetchLatestAccepted(stream)
	if err != nil {
		ss.Error = err.Error()
		klog.Errorf("stream %s: %v", stream, err)
		return ss
	}
	ss.Payload, ss.PayloadPhase, ss.PullSpec = p.Name, p.Phase, p.PullSpec
	ss.PayloadURL = PayloadURL(stream, p.Name)
	if t, ok := PayloadCreated(p.Name); ok {
		ss.PayloadCreated = t
		ss.PayloadStale = o.PayloadStaleAfter > 0 && now.Sub(t) > o.PayloadStaleAfter
	}

	is, err := FetchImageReferences(p.Name)
	if err != nil {
		ss.Error = err.Error()
		klog.Errorf("stream %s: %v", stream, err)
		return ss
	}

	rows := make([]ImageRow, len(is.Spec.Tags))
	sem := make(chan struct{}, o.Concurrency)
	var wg sync.WaitGroup
	for i, t := range is.Spec.Tags {
		row := ImageRow{
			Name:          t.Name,
			BuiltBranch:   t.Annotations["io.openshift.build.commit.ref"],
			PayloadCommit: t.Annotations["io.openshift.build.commit.id"],
			Image:         t.From.Name,
		}
		row.Branch = row.BuiltBranch
		row.Org, row.Repo, _ = JobName(t.Annotations["io.openshift.build.source-location"], row.BuiltBranch)
		rows[i] = row

		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			r := &rows[i]
			// Find the job that currently promotes to this stream; the image's
			// recorded branch may predate a release branch cut.
			if r.Repo != "" {
				br, ok, err := cfgs.resolveJob(r.Org, r.Repo, r.BuiltBranch, ss.Version)
				switch {
				case err != nil && !ok:
					r.JobNote = "could not read CI config: " + err.Error()
				case !ok:
					r.JobNote = "no okd-scos config currently promotes to scos-" + ss.Version
				}
				r.Branch = br
				_, _, r.Job = JobName("https://github.com/"+r.Org+"/"+r.Repo, br)
				r.JobHistoryURL = JobHistoryURL(r.Job)
				r.LatestRun = jobs.get(r.Job)
			}
			// Read build time from the payload's digest-pinned image. (Tag lookups on
			// quay-proxy can return stale cached images, so tags are not used.)
			created, commit, err := reg.ImageInfo(r.Image)
			if err == ErrRegistryUnauthorized {
				// reported once at report level
			} else if err != nil {
				r.ImageError = err.Error()
			} else {
				r.BuiltAt, r.BuiltCommit = created, commit
			}
			r.Flags = computeFlags(*r, o.StaleAfter, now)
		}(i)
	}
	wg.Wait()

	for _, r := range rows {
		for _, f := range r.Flags {
			ss.Counts[f]++
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		si, sj := severity(rows[i]), severity(rows[j])
		if si != sj {
			return si < sj
		}
		return rows[i].Name < rows[j].Name
	})
	ss.Images = rows
	return ss
}

func computeFlags(r ImageRow, staleAfter time.Duration, now time.Time) []string {
	var f []string
	if r.LatestRun.Found && isFailState(r.LatestRun.Result) {
		f = append(f, FlagFailing)
	}
	if isPending(r) {
		f = append(f, FlagPending)
	}
	if !r.BuiltAt.IsZero() && staleAfter > 0 && now.Sub(r.BuiltAt) > staleAfter {
		f = append(f, FlagStale)
	}
	if r.Job != "" && !r.LatestRun.Found && r.LatestRun.Error == "" {
		f = append(f, FlagNoRun)
	}
	return f
}

// rebuildMargin is how long after an image's creation the job that built it
// can still finish (remaining build steps + promotion). A successful run that
// finished later than BuiltAt+rebuildMargin must have produced a newer image.
const rebuildMargin = 2 * time.Hour

// isPending reports whether the job's latest successful run produced an image
// newer than the one in the payload: either a different commit, or a rebuild
// of the same commit (manual/Gangway re-trigger) that finished after the
// payload image was built.
func isPending(r ImageRow) bool {
	lr := r.LatestRun
	if !lr.Found || lr.Result != "SUCCESS" {
		return false
	}
	if lr.Revision != "" && r.PayloadCommit != "" && lr.Revision != r.PayloadCommit {
		return true
	}
	return !r.BuiltAt.IsZero() && !lr.Finished.IsZero() && lr.Finished.After(r.BuiltAt.Add(rebuildMargin))
}

// severity orders rows: failing, pending, stale, no-run, healthy.
func severity(r ImageRow) int {
	for i, f := range AllFlags {
		if r.HasFlag(f) {
			return i
		}
	}
	return len(AllFlags)
}

// FormatAge renders a duration as a short human-readable age.
func FormatAge(d time.Duration) string {
	switch h := d.Hours(); {
	case h < 1:
		return strconv.Itoa(int(d.Minutes())) + "m"
	case h < 48:
		return strconv.Itoa(int(h)) + "h"
	default:
		return strconv.Itoa(int(h/24)) + "d"
	}
}
