// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/enverbisevac/gitlib/log"
	"github.com/enverbisevac/gitlib/util"
	"github.com/go-git/go-git/v5/plumbing"
	git2go "github.com/libgit2/git2go/v34"
	"golang.org/x/exp/slices"
)

// ReadTreeToIndex reads a treeish to the index
func (repo *Repository) ReadTreeToIndex(treeish string, indexFilename string) (err error) {
	if len(treeish) != 40 {
		treeish, err = repo.GetFullCommitID(treeish)
		if err != nil {
			return err
		}
	}
	id, err := NewIDFromString(treeish)
	if err != nil {
		return err
	}
	return repo.readTreeToIndex(id, indexFilename)
}

func (repo *Repository) readTreeToIndex(id SHA1, indexFilename string) error {
	var (
		index *git2go.Index
		err   error
	)

	if indexFilename != "" {
		index, err = git2go.OpenIndex(indexFilename)
	} else {
		index, err = git2go.NewIndex()
	}
	if err != nil {
		return err
	}

	oid, err := git2go.NewOid(id.String())
	if err != nil {
		return err
	}

	ref, err := repo.git2go.LookupCommit(oid)
	if err != nil {
		return err
	}

	obj, err := ref.Peel(git2go.ObjectTree)
	if err != nil {
		return err
	}

	tree, err := obj.AsTree()
	if err != nil {
		return err
	}

	err = index.ReadTree(tree)
	if err != nil {
		return err
	}

	err = index.Write()
	if err != nil {
		return err
	}

	return nil
}

// ReadTreeToTemporaryIndex reads a treeish to a temporary index file
func (repo *Repository) ReadTreeToTemporaryIndex(treeish string) (filename, tmpDir string, cancel context.CancelFunc, err error) {
	tmpDir, err = os.MkdirTemp("", "index")
	if err != nil {
		return
	}

	filename = filepath.Join(tmpDir, ".tmp-index")
	cancel = func() {
		err := util.RemoveAll(tmpDir)
		if err != nil {
			log.Error("failed to remove tmp index file: %v", err)
		}
	}
	err = repo.ReadTreeToIndex(treeish, filename)
	if err != nil {
		defer cancel()
		return "", "", func() {}, err
	}
	return filename, tmpDir, cancel, err
}

// EmptyIndex empties the index
func (repo *Repository) EmptyIndex() error {
	_, _, err := NewCommand(repo.Ctx, "read-tree", "--empty").RunStdString(&RunOpts{Dir: repo.Path})
	return err
}

// LsFiles checks if the given filenames are in the index
func (repo *Repository) LsFiles(filenames ...string) ([]string, error) {

	ref, err := repo.gogit.Head()
	if err != nil {
		if strings.Contains(err.Error(), "reference not found") {
			return []string{}, nil
		}
		return nil, err
	}

	commit, err := repo.gogit.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(filenames))
	for _, entry := range tree.Entries {
		if slices.Contains(filenames, entry.Name) {
			paths = append(paths, entry.Name)
		}
	}

	return paths, err
}

// RemoveFilesFromIndex removes given filenames from the index - it does not check whether they are present.
func (repo *Repository) RemoveFilesFromIndex(filenames ...string) error {
	ndx, err := repo.git2go.Index()
	if err != nil {
		return err
	}

	for _, file := range filenames {
		if file != "" {
			err = ndx.RemoveByPath(file)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// AddObjectToIndex adds the provided object hash to the index at the provided filename
func (repo *Repository) AddObjectToIndex(mode string, object SHA1, filename string) error {
	ndx, err := repo.git2go.Index()
	if err != nil {
		return err
	}

	oid, err := git2go.NewOid(object.String())
	if err != nil {
		return err
	}

	err = ndx.Add(&git2go.IndexEntry{
		Mode: git2go.FilemodeBlob,
		Id:   oid,
		Path: filename,
	})
	if err != nil {
		return fmt.Errorf("unable to add object to index at %s in repo %s: %w", object, repo.Path, err)
	}

	return nil
}

// WriteTree writes the current index as a tree to the object db and returns its hash
func (repo *Repository) WriteTree() (*Tree, error) {
	ndx, err := repo.git2go.Index()
	if err != nil {
		return nil, err
	}
	oid, err := ndx.WriteTree()
	if err != nil {
		return nil, err
	}

	return NewTree(repo, plumbing.NewHash(oid.String())), nil
}
