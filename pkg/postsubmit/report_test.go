package postsubmit

import (
	"reflect"
	"testing"
	"time"
)

func TestJobName(t *testing.T) {
	org, repo, job := JobName("https://github.com/openshift/machine-config-operator", "release-5.0")
	if org != "openshift" || repo != "machine-config-operator" ||
		job != "branch-ci-openshift-machine-config-operator-release-5.0-okd-scos-images" {
		t.Fatalf("got %q %q %q", org, repo, job)
	}
	if _, _, job := JobName("https://github.com/openshift/x", ""); job != "" {
		t.Fatalf("expected empty job without branch, got %q", job)
	}
}

func TestStreamVersion(t *testing.T) {
	if v := StreamVersion("5.0.0-0.okd-scos"); v != "5.0" {
		t.Fatalf("got %q", v)
	}
}

func TestComputeFlags(t *testing.T) {
	now := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	fresh, old := now.Add(-24*time.Hour), now.Add(-40*24*time.Hour)
	cases := []struct {
		name string
		row  ImageRow
		want []string
	}{
		{"healthy", ImageRow{Job: "j", PayloadCommit: "a", BuiltAt: fresh,
			LatestRun: JobResult{Found: true, Result: "SUCCESS", Revision: "a"}}, nil},
		{"failing", ImageRow{Job: "j", PayloadCommit: "a", BuiltAt: fresh,
			LatestRun: JobResult{Found: true, Result: "FAILURE", Revision: "b"}}, []string{FlagFailing}},
		{"pending", ImageRow{Job: "j", PayloadCommit: "a", BuiltAt: fresh,
			LatestRun: JobResult{Found: true, Result: "SUCCESS", Revision: "b"}}, []string{FlagPending}},
		{"same-commit rebuild after payload image is pending", ImageRow{Job: "j", PayloadCommit: "a", BuiltAt: old,
			LatestRun: JobResult{Found: true, Result: "SUCCESS", Revision: "a", Finished: fresh}}, []string{FlagPending, FlagStale}},
		{"run that built the payload image is not pending", ImageRow{Job: "j", PayloadCommit: "a", BuiltAt: fresh,
			LatestRun: JobResult{Found: true, Result: "SUCCESS", Revision: "a", Finished: fresh.Add(10 * time.Minute)}}, nil},
		{"stale no-run", ImageRow{Job: "j", PayloadCommit: "a", BuiltAt: old}, []string{FlagStale, FlagNoRun}},
		{"gcs error is not no-run", ImageRow{Job: "j", BuiltAt: fresh, LatestRun: JobResult{Error: "boom"}}, nil},
		{"unknown build time is not stale", ImageRow{Job: "j", PayloadCommit: "a",
			LatestRun: JobResult{Found: true, Result: "SUCCESS", Revision: "a"}}, nil},
	}
	for _, c := range cases {
		if got := computeFlags(c.row, 30*24*time.Hour, now); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}

func TestHTMLRenders(t *testing.T) {
	r := &Report{Generated: time.Now(), Window: 12 * time.Hour, StaleAfter: 30 * 24 * time.Hour,
		Runs: []Run{{Job: "branch-ci-openshift-x-main-okd-scos-images", Org: "openshift", Repo: "x", Branch: "main", State: "failure", Started: time.Now()}},
		Streams: []StreamState{{Stream: "5.0.0-0.okd-scos", Version: "5.0", Counts: map[string]int{},
			Images: []ImageRow{{Name: "x", Org: "openshift", Repo: "x", Branch: "main", Job: "j", Flags: []string{FlagFailing},
				LatestRun: JobResult{Found: true, Result: "FAILURE", Revision: "abcdef123"}}}}}}
	if _, err := r.HTML(); err != nil {
		t.Fatal(err)
	}
}

func TestParsePromotionTarget(t *testing.T) {
	cfg := "images:\n  items: []\npromotion:\n  to:\n  - name: scos-5.0\n    namespace: origin\nreleases:\n  latest: {}\n"
	if got := parsePromotionTarget(cfg); got != "scos-5.0" {
		t.Fatalf("got %q", got)
	}
	cfg2 := "promotion:\n  to:\n  - namespace: origin\n    name: scos-5.1\nresources: {}\n"
	if got := parsePromotionTarget(cfg2); got != "scos-5.1" {
		t.Fatalf("got %q", got)
	}
	if got := parsePromotionTarget("images: {}\n"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestPayloadCreated(t *testing.T) {
	got, ok := PayloadCreated("5.1.0-0.okd-scos-2026-10-28-203704")
	want := time.Date(2026, 10, 28, 20, 37, 4, 0, time.UTC)
	if !ok || !got.Equal(want) {
		t.Fatalf("got %v %v", got, ok)
	}
	if _, ok := PayloadCreated("bogus"); ok {
		t.Fatal("expected failure")
	}
}
