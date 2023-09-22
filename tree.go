// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"io"
	"path"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Tree represents a flat directory listing.
type Tree struct {
	ID         SHA1
	ResolvedID SHA1
	repo       *Repository

	gogitTree *object.Tree

	// parent tree
	ptree *Tree
}

func (t *Tree) loadTreeObject() error {
	gogitTree, err := t.repo.gogit.TreeObject(t.ID)
	if err != nil {
		return err
	}

	t.gogitTree = gogitTree
	return nil
}

// ListEntries returns all entries of current tree.
func (t *Tree) ListEntries() (Entries, error) {
	if t.gogitTree == nil {
		err := t.loadTreeObject()
		if err != nil {
			return nil, err
		}
	}

	entries := make([]*TreeEntry, len(t.gogitTree.Entries))
	for i, entry := range t.gogitTree.Entries {
		entries[i] = &TreeEntry{
			ID:    entry.Hash,
			entry: &t.gogitTree.Entries[i],
			ptree: t,
		}
	}

	return entries, nil
}

// ListEntriesRecursiveWithSize returns all entries of current tree recursively including all subtrees
func (t *Tree) ListEntriesRecursiveWithSize() (Entries, error) {
	if t.gogitTree == nil {
		err := t.loadTreeObject()
		if err != nil {
			return nil, err
		}
	}

	var entries []*TreeEntry
	seen := map[plumbing.Hash]bool{}
	walker := object.NewTreeWalker(t.gogitTree, true, seen)
	for {
		fullName, entry, err := walker.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if seen[entry.Hash] {
			continue
		}

		convertedEntry := &TreeEntry{
			ID:       entry.Hash,
			entry:    &entry,
			ptree:    t,
			fullName: fullName,
		}
		entries = append(entries, convertedEntry)
	}

	return entries, nil
}

// ListEntriesRecursiveFast is the alias of ListEntriesRecursiveWithSize for the gogit version
func (t *Tree) ListEntriesRecursiveFast() (Entries, error) {
	return t.ListEntriesRecursiveWithSize()
}

// NewTree create a new tree according the repository and tree id
func NewTree(repo *Repository, id SHA1) *Tree {
	return &Tree{
		ID:   id,
		repo: repo,
	}
}

// SubTree get a sub tree by the sub dir path
func (t *Tree) SubTree(rpath string) (*Tree, error) {
	if len(rpath) == 0 {
		return t, nil
	}

	paths := strings.Split(rpath, "/")
	var (
		err error
		g   = t
		p   = t
		te  *TreeEntry
	)
	for _, name := range paths {
		te, err = p.GetTreeEntryByPath(name)
		if err != nil {
			return nil, err
		}

		g, err = t.repo.getTree(te.ID)
		if err != nil {
			return nil, err
		}
		g.ptree = p
		p = g
	}
	return g, nil
}

// GetTreeEntryByPath get the tree entries according the sub dir
func (t *Tree) GetTreeEntryByPath(relpath string) (*TreeEntry, error) {
	if len(relpath) == 0 {
		return &TreeEntry{
			ID: t.ID,
			// Type: ObjectTree,
			entry: &object.TreeEntry{
				Name: "",
				Mode: filemode.Dir,
				Hash: t.ID,
			},
		}, nil
	}

	relpath = path.Clean(relpath)
	parts := strings.Split(relpath, "/")
	var err error
	tree := t
	for i, name := range parts {
		if i == len(parts)-1 {
			entries, err := tree.ListEntries()
			if err != nil {
				if err == plumbing.ErrObjectNotFound {
					return nil, ErrNotExist{
						RelPath: relpath,
					}
				}
				return nil, err
			}
			for _, v := range entries {
				if v.Name() == name {
					return v, nil
				}
			}
		} else {
			tree, err = tree.SubTree(name)
			if err != nil {
				if err == plumbing.ErrObjectNotFound {
					return nil, ErrNotExist{
						RelPath: relpath,
					}
				}
				return nil, err
			}
		}
	}
	return nil, ErrNotExist{"", relpath}
}

// LsTree checks if the given filenames are in the tree
func (repo *Repository) LsTree(ref string, filenames ...string) ([]string, error) {
	cmd := NewCommand(repo.Ctx, "ls-tree", "-z", "--name-only").
		AddDashesAndList(append([]string{ref}, filenames...)...)

	res, _, err := cmd.RunStdBytes(&RunOpts{Dir: repo.Path})
	if err != nil {
		return nil, err
	}
	filelist := make([]string, 0, len(filenames))
	for _, line := range bytes.Split(res, []byte{'\000'}) {
		filelist = append(filelist, string(line))
	}

	return filelist, err
}
