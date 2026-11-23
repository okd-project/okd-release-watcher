package postsubmit

import (
	"errors"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
)

var amd64 = v1.Platform{OS: "linux", Architecture: "amd64"}

// ErrRegistryUnauthorized means quay-proxy rejected our credentials (or we have none).
var ErrRegistryUnauthorized = errors.New("registry: unauthorized (log in to quay-proxy.ci.openshift.org, see README)")

// registry reads image configs from quay-proxy.ci.openshift.org.
// quay-proxy requires credentials; they are taken from the standard docker/podman
// auth files (DOCKER_CONFIG, ~/.docker/config.json, REGISTRY_AUTH_FILE, ...).
// After the first 401 all further lookups are skipped.
type registry struct {
	puller       *remote.Puller
	unauthorized atomic.Bool
}

func newRegistry() (*registry, error) {
	p, err := remote.NewPuller(remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, err
	}
	return &registry{puller: p}, nil
}

// ImageInfo returns the linux/amd64 image's creation time and source commit label.
func (r *registry) ImageInfo(ref string) (created time.Time, commit string, err error) {
	if r.unauthorized.Load() {
		return time.Time{}, "", ErrRegistryUnauthorized
	}
	n, err := name.ParseReference(ref)
	if err != nil {
		return time.Time{}, "", err
	}
	img, err := remote.Image(n, remote.Reuse(r.puller), remote.WithPlatform(amd64))
	if err != nil {
		var te *transport.Error
		if errors.As(err, &te) && (te.StatusCode == http.StatusUnauthorized || te.StatusCode == http.StatusForbidden) {
			r.unauthorized.Store(true)
			return time.Time{}, "", ErrRegistryUnauthorized
		}
		return time.Time{}, "", err
	}
	cf, err := img.ConfigFile()
	if err != nil {
		return time.Time{}, "", err
	}
	return cf.Created.Time.UTC(), cf.Config.Labels["io.openshift.build.commit.id"], nil
}
