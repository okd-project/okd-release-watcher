package gcs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

const (
	artifactSubpath = "artifacts/claude-payload-agent/openshift-claude-payload-agent/artifacts/"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

type gcsListResponse struct {
	Items []gcsItem `json:"items,omitempty"`
}

type gcsItem struct {
	Name string `json:"name"`
}

type autodlJSON struct {
	TableName string                       `json:"table_name"`
	Schema    map[string]string            `json:"schema"`
	Rows      []map[string]json.RawMessage `json:"rows"`
}

type PayloadTriageRow struct {
	PayloadTag             string `json:"payload_tag"`
	Stream                 string `json:"stream"`
	Phase                  string `json:"phase"`
	RejectionStreak        string `json:"rejection_streak"`
	TotalBlockingJobs      string `json:"total_blocking_jobs"`
	FailedBlockingJobs     string `json:"failed_blocking_jobs"`
	JobName                string `json:"job_name"`
	ProwURL                string `json:"prow_url"`
	FailureType            string `json:"failure_type"`
	RootCauseSummary       string `json:"root_cause_summary"`
	StreakLength            string `json:"streak_length"`
	IsNewFailure           string `json:"is_new_failure"`
	CandidatePRURL         string `json:"candidate_pr_url"`
	CandidateTitle         string `json:"candidate_title"`
	CandidateConfidence    string `json:"candidate_confidence_score"`
	ForceAcceptRecommended string `json:"force_accept_recommended"`
}

type AnalysisResult struct {
	Rows    []PayloadTriageRow
	HTMLURL string
}

// FetchAnalysisFromProwURL extracts the bucket and build ID from a Prow job URL
// and fetches the claude-payload-agent analysis artifacts from GCS.
func FetchAnalysisFromProwURL(prowURL string, tag string) (*AnalysisResult, error) {
	bucket, jobPath, err := parseProwURL(prowURL)
	if err != nil {
		return nil, fmt.Errorf("parsing Prow URL: %w", err)
	}

	gcsAPIBase := "https://storage.googleapis.com/storage/v1/b/" + bucket + "/o"
	prefix := jobPath + "/" + artifactSubpath

	autodlPath, htmlPath, err := findArtifacts(gcsAPIBase, prefix, tag)
	if err != nil {
		return nil, fmt.Errorf("finding artifacts: %w", err)
	}

	result := &AnalysisResult{}

	if htmlPath != "" {
		result.HTMLURL = fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, htmlPath)
	}

	if autodlPath != "" {
		rows, err := fetchAutodlJSON(gcsAPIBase, autodlPath)
		if err != nil {
			klog.V(2).Infof("Failed to fetch autodl.json for %s: %v", tag, err)
		} else {
			result.Rows = rows
		}
	}

	return result, nil
}

// parseProwURL extracts the bucket name and job path from a Prow URL.
// Input:  https://prow.ci.openshift.org/view/gs/test-platform-results-public/logs/job-name/12345
// Output: bucket="test-platform-results-public", jobPath="logs/job-name/12345"
func parseProwURL(prowURL string) (bucket, jobPath string, err error) {
	const marker = "/view/gs/"
	idx := strings.Index(prowURL, marker)
	if idx < 0 {
		return "", "", fmt.Errorf("URL does not contain %q: %s", marker, prowURL)
	}

	path := prowURL[idx+len(marker):]
	slashIdx := strings.Index(path, "/")
	if slashIdx < 0 {
		return "", "", fmt.Errorf("no path after bucket in URL: %s", prowURL)
	}

	bucket = path[:slashIdx]
	jobPath = path[slashIdx+1:]
	return bucket, jobPath, nil
}

func findArtifacts(gcsAPIBase, prefix, tag string) (autodlPath, htmlPath string, err error) {
	u := fmt.Sprintf("%s?prefix=%s", gcsAPIBase, url.QueryEscape(prefix))
	klog.V(2).Infof("Listing GCS artifacts: %s", u)

	resp, err := httpClient.Get(u)
	if err != nil {
		return "", "", fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("GET returned %d: %s", resp.StatusCode, string(body))
	}

	var result gcsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", fmt.Errorf("decoding response: %w", err)
	}

	expectedAutodl := fmt.Sprintf("payload-analysis-%s-autodl.json", tag)
	expectedHTML := fmt.Sprintf("payload-analysis-%s-summary.html", tag)

	for _, item := range result.Items {
		filename := item.Name[strings.LastIndex(item.Name, "/")+1:]
		if filename == expectedAutodl {
			autodlPath = item.Name
		}
		if filename == expectedHTML {
			htmlPath = item.Name
		}
	}

	if autodlPath == "" && htmlPath == "" {
		return "", "", fmt.Errorf("no analysis artifacts found for tag %s", tag)
	}

	return autodlPath, htmlPath, nil
}

func fetchAutodlJSON(gcsAPIBase, objectPath string) ([]PayloadTriageRow, error) {
	u := fmt.Sprintf("%s/%s?alt=media", gcsAPIBase, url.PathEscape(objectPath))

	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading body: %w", err)
	}

	var autodl autodlJSON
	if err := json.Unmarshal(body, &autodl); err != nil {
		return nil, fmt.Errorf("decoding autodl.json: %w", err)
	}

	var rows []PayloadTriageRow
	for _, rawRow := range autodl.Rows {
		rowBytes, err := json.Marshal(rawRow)
		if err != nil {
			continue
		}
		var row PayloadTriageRow
		if err := json.Unmarshal(rowBytes, &row); err != nil {
			klog.V(3).Infof("Error parsing row: %v", err)
			continue
		}
		rows = append(rows, row)
	}

	return rows, nil
}
