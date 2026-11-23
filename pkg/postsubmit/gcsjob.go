package postsubmit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type finishedJSON struct {
	Timestamp int64  `json:"timestamp"`
	Passed    *bool  `json:"passed"`
	Result    string `json:"result"`
	Revision  string `json:"revision"`
}

// errNotFound marks a missing GCS object.
var errNotFound = fmt.Errorf("not found")

func gcsGet(path string) ([]byte, error) {
	resp, err := httpClient.Get(GCSBaseURL + "/" + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", path, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// JobHistoryURL is Prow's job history page for a job.
func JobHistoryURL(job string) string {
	return fmt.Sprintf("%s/job-history/gs/%s/logs/%s", ProwURL, GCSBucket, job)
}

// LatestResult reads the latest finished run of a job from GCS.
// Found=false means GCS has no record (no run in ~60 days, or never ran).
func LatestResult(job string) JobResult {
	var jr JobResult
	b, err := gcsGet("logs/" + job + "/latest-build.txt")
	if err == errNotFound {
		return jr
	}
	if err != nil {
		jr.Error = err.Error()
		return jr
	}
	id := strings.TrimSpace(string(b))
	jr.Found = true
	jr.BuildID = id
	jr.URL = fmt.Sprintf("%s/view/gs/%s/logs/%s/%s", ProwURL, GCSBucket, job, id)

	b, err = gcsGet("logs/" + job + "/" + id + "/finished.json")
	if err == errNotFound {
		// latest-build.txt is written at start; no finished.json means still running.
		jr.Result = "PENDING"
		return jr
	}
	if err != nil {
		jr.Error = err.Error()
		return jr
	}
	var f finishedJSON
	if err := json.Unmarshal(b, &f); err != nil {
		jr.Error = fmt.Sprintf("decoding finished.json: %v", err)
		return jr
	}
	jr.Result = f.Result
	if jr.Result == "" && f.Passed != nil {
		if *f.Passed {
			jr.Result = "SUCCESS"
		} else {
			jr.Result = "FAILURE"
		}
	}
	jr.Revision = f.Revision
	if f.Timestamp > 0 {
		jr.Finished = time.Unix(f.Timestamp, 0).UTC()
	}
	return jr
}
