// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"net/http"
	"net/url"
	"os"
	"sync"

	"github.com/enverbisevac/gitlib/log"
	"github.com/gobwas/glob"
)

var (
	once         sync.Once
	hostMatchers []glob.Glob
)

// GetProxyURL returns proxy url
func GetProxyURL() string {
	if !Proxy.Enabled {
		return ""
	}

	if Proxy.ProxyURL == "" {
		if os.Getenv("http_proxy") != "" {
			return os.Getenv("http_proxy")
		}
		return os.Getenv("https_proxy")
	}
	return Proxy.ProxyURL
}

// Match return true if url needs to be proxied
func Match(u string) bool {
	if !Proxy.Enabled {
		return false
	}

	// enforce do once
	GetProxy()

	for _, v := range hostMatchers {
		if v.Match(u) {
			return true
		}
	}
	return false
}

// GetProxy returns the system proxy
func GetProxy() func(req *http.Request) (*url.URL, error) {
	if !Proxy.Enabled {
		return func(req *http.Request) (*url.URL, error) {
			return nil, nil
		}
	}
	if Proxy.ProxyURL == "" {
		return http.ProxyFromEnvironment
	}

	once.Do(func() {
		for _, h := range Proxy.ProxyHosts {
			if g, err := glob.Compile(h); err == nil {
				hostMatchers = append(hostMatchers, g)
			} else {
				log.Error("glob.Compile %s failed: %v", h, err)
			}
		}
	})

	return func(req *http.Request) (*url.URL, error) {
		for _, v := range hostMatchers {
			if v.Match(req.URL.Host) {
				return http.ProxyURL(Proxy.ProxyURLFixed)(req)
			}
		}
		return http.ProxyFromEnvironment(req)
	}
}
