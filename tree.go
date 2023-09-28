package git

import (
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// EntryMode the type of the object in the git tree
type EntryMode int

// There are only a few file modes in Git. They look like unix file modes, but they can only be
// one of these.
const (
	// EntryModeBlob
	EntryModeBlob EntryMode = 0o100644
	// EntryModeExec
	EntryModeExec EntryMode = 0o100755
	// EntryModeSymlink
	EntryModeSymlink EntryMode = 0o120000
	// EntryModeCommit
	EntryModeCommit EntryMode = 0o160000
	// EntryModeTree
	EntryModeTree EntryMode = 0o040000
)

// String converts an EntryMode to a string
func (e EntryMode) String() string {
	return strconv.FormatInt(int64(e), 8)
}

// ToEntryMode converts a string to an EntryMode
func ToEntryMode(value string) EntryMode {
	v, _ := strconv.ParseInt(value, 8, 32)
	return EntryMode(v)
}

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

// GetBlobByPath get the blob object according the path
func (t *Tree) GetBlobByPath(relpath string) (*Blob, error) {
	entry, err := t.GetTreeEntryByPath(relpath)
	if err != nil {
		return nil, err
	}

	if !entry.IsDir() && !entry.IsSubModule() {
		return entry.Blob(), nil
	}

	return nil, ErrNotExist{"", relpath}
}
