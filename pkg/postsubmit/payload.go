package postsubmit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LatestPayload is the latest accepted payload of a stream.
type LatestPayload struct {
	Name     string `json:"name"`
	Phase    string `json:"phase"`
	PullSpec string `json:"pullSpec"`
}

// FetchLatestAccepted returns the latest accepted payload of a release stream.
func FetchLatestAccepted(stream string) (*LatestPayload, error) {
	u := fmt.Sprintf("%s/api/v1/releasestream/%s/latest", ReleaseControllerURL, url.PathEscape(stream))
	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", u, resp.StatusCode)
	}
	var p LatestPayload
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		return nil, err
	}
	if p.PullSpec == "" {
		return nil, fmt.Errorf("stream %s: no accepted payload", stream)
	}
	return &p, nil
}

// imageStream is the subset of the payload's image-references we need.
type imageStream struct {
	Spec struct {
		Tags []struct {
			Name        string            `json:"name"`
			Annotations map[string]string `json:"annotations"`
			From        struct {
				Name string `json:"name"`
			} `json:"from"`
		} `json:"tags"`
	} `json:"spec"`
}

// FetchImageReferences returns the payload's image list (with source repo,
// commit and branch annotations) from the release controller's public
// /releasetag/<tag>/json endpoint. No registry credentials needed.
func FetchImageReferences(tag string) (*imageStream, error) {
	u := fmt.Sprintf("%s/releasetag/%s/json", ReleaseControllerURL, url.PathEscape(tag))
	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", u, resp.StatusCode)
	}
	var body struct {
		References imageStream `json:"references"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding %s: %w", u, err)
	}
	if len(body.References.Spec.Tags) == 0 {
		return nil, fmt.Errorf("payload %s: no image references", tag)
	}
	return &body.References, nil
}

// PayloadCreated parses the creation time from a payload name such as
// 5.1.0-0.okd-scos-2026-10-28-203704.
func PayloadCreated(name string) (time.Time, bool) {
	const layout = "2006-01-02-150405"
	if len(name) < len(layout) {
		return time.Time{}, false
	}
	t, err := time.Parse(layout, name[len(name)-len(layout):])
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

// StreamVersion turns "5.0.0-0.okd-scos" into "5.0".
func StreamVersion(stream string) string {
	parts := strings.SplitN(stream, ".", 3)
	if len(parts) < 2 {
		return stream
	}
	return parts[0] + "." + parts[1]
}

// PayloadURL is the release controller page for a payload.
func PayloadURL(stream, payload string) string {
	return fmt.Sprintf("%s/releasestream/%s/release/%s", ReleaseControllerURL, stream, payload)
}

// JobName derives the OKD postsubmit job from an image's source annotations.
// sourceLocation is like https://github.com/openshift/machine-config-operator.
func JobName(sourceLocation, branch string) (org, repo, job string) {
	s := strings.TrimSuffix(strings.TrimSuffix(sourceLocation, "/"), ".git")
	parts := strings.Split(s, "/")
	if len(parts) < 2 || branch == "" {
		return "", "", ""
	}
	org, repo = parts[len(parts)-2], parts[len(parts)-1]
	return org, repo, fmt.Sprintf("branch-ci-%s-%s-%s-okd-scos-images", org, repo, branch)
}
