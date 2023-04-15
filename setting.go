package git

import (
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/enverbisevac/gitlib/log"
)

var (
	CacheService = struct {
		Cache struct {
			TTL time.Duration
		}
		LastCommit struct {
			Enabled      bool
			TTL          time.Duration
			CommitsCount int64
		}
	}{
		LastCommit: struct {
			Enabled      bool
			TTL          time.Duration
			CommitsCount int64
		}{
			Enabled:      true,
			TTL:          8760 * time.Hour,
			CommitsCount: 1000,
		},
	}

	Git = struct {
		EnableAutoGitWireProtocol bool
		DisableCoreProtectNTFS    bool
		DisablePartialClone       bool
		CommitsRangeSize          int
		Path                      string
		HomePath                  string
		Timeout                   struct {
			Default int
		}
		LargeObjectThreshold int64
	}{}
	LFS = struct {
		StartServer bool
	}{}
	Proxy = struct {
		Enabled       bool
		ProxyURL      string
		ProxyURLFixed *url.URL
		ProxyHosts    []string
	}{
		Enabled:    false,
		ProxyURL:   "",
		ProxyHosts: []string{},
	}
)

func newProxyService() {
	Proxy.Enabled = os.Getenv("PROXY_ENABLED") == "true"
	Proxy.ProxyURL = os.Getenv("PROXY_URL")
	if Proxy.ProxyURL != "" {
		var err error
		Proxy.ProxyURLFixed, err = url.Parse(Proxy.ProxyURL)
		if err != nil {
			log.Error("Global PROXY_URL is not valid")
			Proxy.ProxyURL = ""
		}
	}
	Proxy.ProxyHosts = strings.Split(os.Getenv("PROXY_HOSTS"), ",")
}

// LastCommitCacheTTLSeconds returns the TTLSeconds or unix timestamp for memcache
func LastCommitCacheTTLSeconds() int64 {
	return int64(CacheService.LastCommit.TTL.Seconds())
}

// NewServices initializes the services
func NewServices() {
	newProxyService()
}