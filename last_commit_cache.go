// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"crypto/sha256"
	"fmt"

	"github.com/enverbisevac/gitlib/log"
)

func getCacheKey(repoPath, commitID, entryPath string) string {
	hashBytes := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s", repoPath, commitID, entryPath)))
	return fmt.Sprintf("last_commit:%x", hashBytes)
}

// LastCommitCache represents a cache to store last commit
type LastCommitCache struct {
	repoPath    string
	ttl         func() int64
	repo        *Repository
	commitCache map[string]*Commit
	cache       Cache
}

// NewLastCommitCache creates a new last commit cache for repo
func NewLastCommitCache(count int64, repoPath string, gitRepo *Repository, cache Cache) *LastCommitCache {
	if cache == nil {
		return nil
	}
	if !CacheService.LastCommit.Enabled || count < CacheService.LastCommit.CommitsCount {
		return nil
	}

	return &LastCommitCache{
		repoPath: repoPath,
		repo:     gitRepo,
		ttl:      LastCommitCacheTTLSeconds,
		cache:    cache,
	}
}

// Put put the last commit id with commit and entry path
func (c *LastCommitCache) Put(ref, entryPath, commitID string) error {
	if c == nil || c.cache == nil {
		return nil
	}
	log.Info("LastCommitCache save: [%s:%s:%s]", ref, entryPath, commitID)
	return c.cache.Put(getCacheKey(c.repoPath, ref, entryPath), commitID, c.ttl())
}

// Get gets the last commit information by commit id and entry path
func (c *LastCommitCache) Get(ref, entryPath string) (*Commit, error) {
	if c == nil || c.cache == nil {
		return nil, nil
	}

	commitID, ok := c.cache.Get(getCacheKey(c.repoPath, ref, entryPath)).(string)
	if !ok || commitID == "" {
		return nil, nil
	}

	log.Info("LastCommitCache hit level 1: [%s:%s:%s]", ref, entryPath, commitID)
	if c.commitCache != nil {
		if commit, ok := c.commitCache[commitID]; ok {
			log.Info("LastCommitCache hit level 2: [%s:%s:%s]", ref, entryPath, commitID)
			return commit, nil
		}
	}

	commit, err := c.repo.GetCommit(commitID)
	if err != nil {
		return nil, err
	}
	if c.commitCache == nil {
		c.commitCache = make(map[string]*Commit)
	}
	c.commitCache[commitID] = commit
	return commit, nil
}

// GetCommitByPath gets the last commit for the entry in the provided commit
func (c *LastCommitCache) GetCommitByPath(commitID, entryPath string) (*Commit, error) {
	sha1, err := NewIDFromString(commitID)
	if err != nil {
		return nil, err
	}

	lastCommit, err := c.Get(sha1.String(), entryPath)
	if err != nil || lastCommit != nil {
		return lastCommit, err
	}

	lastCommit, err = c.repo.getCommitByPathWithID(sha1, entryPath)
	if err != nil {
		return nil, err
	}

	if err := c.Put(commitID, entryPath, lastCommit.ID.String()); err != nil {
		log.Error("Unable to cache %s as the last commit for %q in %s %s. Error %v", lastCommit.ID.String(), entryPath, commitID, c.repoPath, err)
	}

	return lastCommit, nil
}
