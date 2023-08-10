// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"errors"
	"io"
	"path/filepath"

	"github.com/enverbisevac/gitlib/log"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
)

// contextKey is a value for use with context.WithValue.
type contextKey struct {
	name string
}

// RepositoryContextKey is a context key. It is used with context.Value() to get the current Repository for the context
var RepositoryContextKey = &contextKey{"repository"}

// RepositoryFromContext attempts to get the repository from the context
func RepositoryFromContext(ctx context.Context, path string) *Repository {
	value := ctx.Value(RepositoryContextKey)
	if value == nil {
		return nil
	}

	if repo, ok := value.(*Repository); ok && repo != nil {
		if repo.Path == path {
			return repo
		}
	}

	return nil
}

type nopCloser func()

func (nopCloser) Close() error { return nil }

// RepositoryFromContextOrOpen attempts to get the repository from the context or just opens it
func RepositoryFromContextOrOpen(ctx context.Context, path string) (*Repository, io.Closer, error) {
	gitRepo := RepositoryFromContext(ctx, path)
	if gitRepo != nil {
		return gitRepo, nopCloser(nil), nil
	}

	gitRepo, err := OpenRepository(ctx, path)
	return gitRepo, gitRepo, err
}

// Repository represents a Git repository.
type Repository struct {
	*gogit.Repository
	Path string

	tagCache *ObjectCache

	storage     *filesystem.Storage
	gpgSettings *GPGSettings

	Ctx             context.Context
	LastCommitCache *LastCommitCache
}

// openRepositoryWithDefaultContext opens the repository at the given path with DefaultContext.
func openRepositoryWithDefaultContext(repoPath string) (*Repository, error) {
	return OpenRepository(DefaultContext, repoPath)
}

// OpenRepository opens the repository at the given path within the context.Context
func OpenRepository(ctx context.Context, repoPath string) (*Repository, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, err
	} else if !isDir(repoPath) {
		return nil, errors.New("no such file or directory")
	}

	fs := osfs.New(repoPath)
	_, err = fs.Stat(".git")
	if err == nil {
		fs, err = fs.Chroot(".git")
		if err != nil {
			return nil, err
		}
	}
	storage := filesystem.NewStorageWithOptions(fs, cache.NewObjectLRUDefault(), filesystem.Options{KeepDescriptors: true, LargeObjectThreshold: Git.LargeObjectThreshold})
	repo, err := gogit.Open(storage, fs)
	if err != nil {
		return nil, err
	}

	return &Repository{
		Path:       repoPath,
		Repository: repo,
		storage:    storage,
		tagCache:   newObjectCache(),
		Ctx:        ctx,
	}, nil
}

// Close this repository, in particular close the underlying gogitStorage if this is not nil
func (repo *Repository) Close() (err error) {
	if repo == nil || repo.storage == nil {
		return
	}
	if err := repo.storage.Close(); err != nil {
		log.Error("Error closing storage: %v", err)
	}
	repo.LastCommitCache = nil
	repo.tagCache = nil
	return
}
