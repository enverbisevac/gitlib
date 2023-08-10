// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/storer"
)

// BranchPrefix base dir of the branch information file store on git
const BranchPrefix = "refs/heads/"

// AGit Flow

// PullRequestPrefix special ref to create a pull request: refs/for/<targe-branch>/<topic-branch>
// or refs/for/<targe-branch> -o topic='<topic-branch>'
const PullRequestPrefix = "refs/for/"

// TODO: /refs/for-review for suggest change interface

// IsReferenceExist returns true if given reference exists in the repository.
func IsReferenceExist(ctx context.Context, repoPath, name string) bool {
	_, _, err := NewCommand(ctx, "show-ref", "--verify").AddDashesAndList(name).RunStdString(&RunOpts{Dir: repoPath})
	return err == nil
}

// IsBranchExist returns true if given branch exists in the repository.
func IsBranchExist(ctx context.Context, repoPath, name string) bool {
	return IsReferenceExist(ctx, repoPath, BranchPrefix+name)
}

// Branch represents a Git branch.
type Branch struct {
	Name string
	Path string

	gitRepo *Repository
}

// GetHEADBranch returns corresponding branch of HEAD.
func (repo *Repository) GetHEADBranch() (*Branch, error) {
	if repo == nil {
		return nil, fmt.Errorf("nil repo")
	}
	stdout, _, err := NewCommand(repo.Ctx, "symbolic-ref", "HEAD").RunStdString(&RunOpts{Dir: repo.Path})
	if err != nil {
		return nil, err
	}
	stdout = strings.TrimSpace(stdout)

	if !strings.HasPrefix(stdout, BranchPrefix) {
		return nil, fmt.Errorf("invalid HEAD branch: %v", stdout)
	}

	return &Branch{
		Name:    stdout[len(BranchPrefix):],
		Path:    stdout,
		gitRepo: repo,
	}, nil
}

// SetDefaultBranch sets default branch of repository.
func (repo *Repository) SetDefaultBranch(name string) error {
	_, _, err := NewCommand(repo.Ctx, "symbolic-ref", "HEAD").AddDynamicArguments(BranchPrefix + name).RunStdString(&RunOpts{Dir: repo.Path})
	return err
}

// GetDefaultBranch gets default branch of repository.
func (repo *Repository) GetDefaultBranch() (string, error) {
	stdout, _, err := NewCommand(repo.Ctx, "symbolic-ref", "HEAD").RunStdString(&RunOpts{Dir: repo.Path})
	if err != nil {
		return "", err
	}
	stdout = strings.TrimSpace(stdout)
	if !strings.HasPrefix(stdout, BranchPrefix) {
		return "", errors.New("the HEAD is not a branch: " + stdout)
	}
	return strings.TrimPrefix(stdout, BranchPrefix), nil
}

// GetBranch returns a branch by it's name
func (repo *Repository) GetBranch(branch string) (*Branch, error) {
	if !repo.IsBranchExist(branch) {
		return nil, ErrBranchNotExist{branch}
	}
	return &Branch{
		Path:    repo.Path,
		Name:    branch,
		gitRepo: repo,
	}, nil
}

// GetBranchesByPath returns a branch by it's path
// if limit = 0 it will not limit
func GetBranchesByPath(ctx context.Context, path string, skip, limit int) ([]*Branch, int, error) {
	gitRepo, err := OpenRepository(ctx, path)
	if err != nil {
		return nil, 0, err
	}
	defer gitRepo.Close()

	return gitRepo.GetBranches(skip, limit)
}

// GetBranches returns a slice of *git.Branch
func (repo *Repository) GetBranches(skip, limit int) ([]*Branch, int, error) {
	brs, countAll, err := repo.GetBranchNames(skip, limit)
	if err != nil {
		return nil, 0, err
	}

	branches := make([]*Branch, len(brs))
	for i := range brs {
		branches[i] = &Branch{
			Path:    repo.Path,
			Name:    brs[i],
			gitRepo: repo,
		}
	}

	return branches, countAll, nil
}

// DeleteBranchOptions Option(s) for delete branch
type DeleteBranchOptions struct {
	Force bool
}

// DeleteBranch delete a branch by name on repository.
func (repo *Repository) DeleteBranch(name string, opts DeleteBranchOptions) error {
	cmd := NewCommand(repo.Ctx, "branch")

	if opts.Force {
		cmd.AddArguments("-D")
	} else {
		cmd.AddArguments("-d")
	}

	cmd.AddDashesAndList(name)
	_, _, err := cmd.RunStdString(&RunOpts{Dir: repo.Path})

	return err
}

// CreateBranch create a new branch
func (repo *Repository) CreateBranch(branch, oldbranchOrCommit string) error {
	cmd := NewCommand(repo.Ctx, "branch")
	cmd.AddDashesAndList(branch, oldbranchOrCommit)

	_, _, err := cmd.RunStdString(&RunOpts{Dir: repo.Path})

	return err
}

// AddRemote adds a new remote to repository.
func (repo *Repository) AddRemote(name, url string, fetch bool) error {
	cmd := NewCommand(repo.Ctx, "remote", "add")
	if fetch {
		cmd.AddArguments("-f")
	}
	cmd.AddDynamicArguments(name, url)

	_, _, err := cmd.RunStdString(&RunOpts{Dir: repo.Path})
	return err
}

// RemoveRemote removes a remote from repository.
func (repo *Repository) RemoveRemote(name string) error {
	_, _, err := NewCommand(repo.Ctx, "remote", "rm").AddDynamicArguments(name).RunStdString(&RunOpts{Dir: repo.Path})
	return err
}

// GetCommit returns the head commit of a branch
func (branch *Branch) GetCommit() (*Commit, error) {
	return branch.gitRepo.GetBranchCommit(branch.Name)
}

// RenameBranch rename a branch
func (repo *Repository) RenameBranch(from, to string) error {
	_, _, err := NewCommand(repo.Ctx, "branch", "-m").AddDynamicArguments(from, to).RunStdString(&RunOpts{Dir: repo.Path})
	return err
}

// IsObjectExist returns true if given reference exists in the repository.
func (repo *Repository) IsObjectExist(name string) bool {
	if name == "" {
		return false
	}

	_, err := repo.ResolveRevision(plumbing.Revision(name))

	return err == nil
}

// IsReferenceExist returns true if given reference exists in the repository.
func (repo *Repository) IsReferenceExist(name string) bool {
	if name == "" {
		return false
	}

	reference, err := repo.Reference(plumbing.ReferenceName(name), true)
	if err != nil {
		return false
	}
	return reference.Type() != plumbing.InvalidReference
}

// IsBranchExist returns true if given branch exists in current repository.
func (repo *Repository) IsBranchExist(name string) bool {
	if name == "" {
		return false
	}
	reference, err := repo.Reference(plumbing.ReferenceName(BranchPrefix+name), true)
	if err != nil {
		return false
	}
	return reference.Type() != plumbing.InvalidReference
}

// GetBranches returns branches from the repository, skipping skip initial branches and
// returning at most limit branches, or all branches if limit is 0.
func (repo *Repository) GetBranchNames(skip, limit int) ([]string, int, error) {
	var branchNames []string

	branches, err := repo.Branches()
	if err != nil {
		return nil, 0, err
	}

	i := 0
	count := 0
	_ = branches.ForEach(func(branch *plumbing.Reference) error {
		count++
		if i < skip {
			i++
			return nil
		} else if limit != 0 && count > skip+limit {
			return nil
		}

		branchNames = append(branchNames, strings.TrimPrefix(branch.Name().String(), BranchPrefix))
		return nil
	})

	// TODO: Sort?

	return branchNames, count, nil
}

// WalkReferences walks all the references from the repository
// refType should be empty, ObjectTag or ObjectBranch. All other values are equivalent to empty.
func WalkReferences(ctx context.Context, repoPath string, walkfn func(sha1, refname string) error) (int, error) {
	repo := RepositoryFromContext(ctx, repoPath)
	if repo == nil {
		var err error
		repo, err = OpenRepository(ctx, repoPath)
		if err != nil {
			return 0, err
		}
		defer repo.Close()
	}

	i := 0
	iter, err := repo.References()
	if err != nil {
		return i, err
	}
	defer iter.Close()

	err = iter.ForEach(func(ref *plumbing.Reference) error {
		err := walkfn(ref.Hash().String(), string(ref.Name()))
		i++
		return err
	})
	return i, err
}

// WalkReferences walks all the references from the repository
func (repo *Repository) WalkReferences(arg ObjectType, skip, limit int, walkfn func(sha1, refname string) error) (int, error) {
	i := 0
	var iter storer.ReferenceIter
	var err error
	switch arg {
	case ObjectTag:
		iter, err = repo.Tags()
	case ObjectBranch:
		iter, err = repo.Branches()
	default:
		iter, err = repo.References()
	}
	if err != nil {
		return i, err
	}
	defer iter.Close()

	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if i < skip {
			i++
			return nil
		}
		err := walkfn(ref.Hash().String(), string(ref.Name()))
		i++
		if err != nil {
			return err
		}
		if limit != 0 && i >= skip+limit {
			return storer.ErrStop
		}
		return nil
	})
	return i, err
}

// GetRefsBySha returns all references filtered with prefix that belong to a sha commit hash
func (repo *Repository) GetRefsBySha(sha, prefix string) ([]string, error) {
	var revList []string
	iter, err := repo.References()
	if err != nil {
		return nil, err
	}
	err = iter.ForEach(func(ref *plumbing.Reference) error {
		if ref.Hash().String() == sha && strings.HasPrefix(string(ref.Name()), prefix) {
			revList = append(revList, string(ref.Name()))
		}
		return nil
	})
	return revList, err
}
