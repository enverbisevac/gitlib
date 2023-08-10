// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/enverbisevac/gitlib/log"
	"github.com/go-git/go-git/v5/plumbing/format/commitgraph"
	cgobject "github.com/go-git/go-git/v5/plumbing/object/commitgraph"
)

// WriteCommitGraph write commit graph to speed up repo access
// this requires git v2.18 to be installed
func WriteCommitGraph(ctx context.Context, repoPath string) error {
	if CheckGitVersionAtLeast("2.18") == nil {
		if _, _, err := NewCommand(ctx, "commit-graph", "write").RunStdString(&RunOpts{Dir: repoPath}); err != nil {
			return fmt.Errorf("unable to write commit-graph for '%s' : %w", repoPath, err)
		}
	}
	return nil
}

// CommitNodeIndex returns the index for walking commit graph
func (r *Repository) CommitNodeIndex() (cgobject.CommitNodeIndex, *os.File) {
	indexPath := path.Join(r.Path, "objects", "info", "commit-graph")

	file, err := os.Open(indexPath)
	if err == nil {
		var index commitgraph.Index
		index, err = commitgraph.OpenFileIndex(file)
		if err == nil {
			return cgobject.NewGraphCommitNodeIndex(index, r.Storer), file
		}
	}

	if !os.IsNotExist(err) {
		log.Info("Unable to read commit-graph for %s: %v", r.Path, err)
	}

	return cgobject.NewObjectCommitNodeIndex(r.Storer), nil
}
