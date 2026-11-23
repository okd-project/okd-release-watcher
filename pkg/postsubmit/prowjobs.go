package postsubmit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"time"
)

// okdJobRe matches OKD SCOS postsubmit image jobs.
var okdJobRe = regexp.MustCompile(`^branch-ci-.+-okd-scos-images$`)

type prowJobList struct {
	Items []struct {
		Spec struct {
			Type string `json:"type"`
			Job  string `json:"job"`
			Refs *struct {
				Org     string `json:"org"`
				Repo    string `json:"repo"`
				BaseRef string `json:"base_ref"`
				BaseSHA string `json:"base_sha"`
			} `json:"refs"`
		} `json:"spec"`
		Status struct {
			State          string     `json:"state"`
			StartTime      *time.Time `json:"startTime"`
			CompletionTime *time.Time `json:"completionTime"`
			URL            string     `json:"url"`
		} `json:"status"`
	} `json:"items"`
}

// FetchRecentRuns returns OKD postsubmit runs started within window.
// Prow's live job list only holds about 24h, so larger windows are capped by it.
// The server ignores ?job= filtering, so we filter client-side.
func FetchRecentRuns(window time.Duration, now time.Time) ([]Run, error) {
	u := ProwURL + "/prowjobs.js?omit=annotations,labels,decoration_config,pod_spec"
	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("fetching prowjobs: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching prowjobs: HTTP %d", resp.StatusCode)
	}
	var list prowJobList
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, fmt.Errorf("decoding prowjobs: %w", err)
	}

	cutoff := now.Add(-window)
	var runs []Run
	for _, it := range list.Items {
		if it.Spec.Type != "postsubmit" || !okdJobRe.MatchString(it.Spec.Job) {
			continue
		}
		if it.Status.StartTime == nil || it.Status.StartTime.Before(cutoff) {
			continue
		}
		r := Run{
			Job:     it.Spec.Job,
			State:   it.Status.State,
			Started: *it.Status.StartTime,
			URL:     it.Status.URL,
		}
		if it.Status.CompletionTime != nil {
			r.Completed = *it.Status.CompletionTime
		}
		if it.Spec.Refs != nil {
			r.Org, r.Repo, r.Branch, r.SHA = it.Spec.Refs.Org, it.Spec.Refs.Repo, it.Spec.Refs.BaseRef, it.Spec.Refs.BaseSHA
		}
		if r.State == "" {
			r.State = "triggered"
		}
		runs = append(runs, r)
	}
	sort.SliceStable(runs, func(i, j int) bool {
		fi, fj := isFailState(runs[i].State), isFailState(runs[j].State)
		if fi != fj {
			return fi
		}
		return runs[i].Started.After(runs[j].Started)
	})
	return runs, nil
}
