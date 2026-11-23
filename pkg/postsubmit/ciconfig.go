package postsubmit

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

// CIConfigRawURL is where ci-operator configs are read from (openshift/release).
const CIConfigRawURL = "https://raw.githubusercontent.com/openshift/release/main/ci-operator/config"

var promotionNameRe = regexp.MustCompile(`(?m)^\s+(?:-\s+)?name:\s*"?([^"\s]+)"?`)

// ciConfigs looks up which ImageStream an OKD variant config promotes to.
// The branch recorded on an image is the branch it was *built* from, which can
// be stale: images built before a release branch was cut still say main/master,
// although main/master now promotes to the next version. So the job feeding a
// stream must be resolved from the configs' promotion targets.
type ciConfigs struct {
	mu sync.Mutex
	m  map[string]*cfgEntry
}

type cfgEntry struct {
	once   sync.Once
	target string // e.g. "scos-5.0"; "" if config missing or no promotion
	err    error
}

func newCIConfigs() *ciConfigs { return &ciConfigs{m: map[string]*cfgEntry{}} }

// promotionTarget returns the origin ImageStream the <org>/<repo>@<branch>
// okd-scos config promotes to, or "" if there is no such config.
func (c *ciConfigs) promotionTarget(org, repo, branch string) (string, error) {
	key := org + "/" + repo + "@" + branch
	c.mu.Lock()
	e, ok := c.m[key]
	if !ok {
		e = &cfgEntry{}
		c.m[key] = e
	}
	c.mu.Unlock()
	e.once.Do(func() { e.target, e.err = fetchPromotionTarget(org, repo, branch) })
	return e.target, e.err
}

func fetchPromotionTarget(org, repo, branch string) (string, error) {
	u := fmt.Sprintf("%s/%s/%s/%s-%s-%s__okd-scos.yaml", CIConfigRawURL, org, repo, org, repo, branch)
	resp, err := httpClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return "", nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: HTTP %d", u, resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return parsePromotionTarget(string(b)), nil
}

// parsePromotionTarget extracts promotion.to[0].name from a ci-operator config
// without a YAML dependency: it scans the top-level `promotion:` block.
func parsePromotionTarget(cfg string) string {
	i := strings.Index("\n"+cfg, "\npromotion:")
	if i < 0 {
		return ""
	}
	block := cfg[i:]
	// Block ends at the next top-level key.
	lines := strings.Split(block, "\n")
	var b strings.Builder
	for n, l := range lines {
		if n > 0 && l != "" && l[0] != ' ' && l[0] != '-' {
			break
		}
		b.WriteString(l + "\n")
	}
	m := promotionNameRe.FindStringSubmatch(b.String())
	if m == nil {
		return ""
	}
	return m[1]
}

// resolveJob finds the branch whose okd-scos postsubmit promotes to scos-<version>.
// It tries release-<version> first, then the branch the image was built from.
// ok=false means neither candidate currently promotes to this stream.
func (c *ciConfigs) resolveJob(org, repo, builtBranch, version string) (branch string, ok bool, err error) {
	want := "scos-" + version
	candidates := []string{"release-" + version}
	if builtBranch != "" && builtBranch != candidates[0] {
		candidates = append(candidates, builtBranch)
	}
	var firstErr error
	for _, br := range candidates {
		t, err := c.promotionTarget(org, repo, br)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if t == want {
			return br, true, nil
		}
	}
	return builtBranch, false, firstErr
}
