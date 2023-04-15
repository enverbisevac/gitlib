// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"
)

// CommitTreeOpts represents the possible options to CommitTree
type CommitTreeOpts struct {
	Parents    []string
	Message    string
	KeyID      string
	NoGPGSign  bool
	AlwaysSign bool
}

func (repo *Repository) getTree(id SHA1) (*Tree, error) {
	gogitTree, err := repo.gogitRepo.TreeObject(id)
	if err != nil {
		return nil, err
	}

	tree := NewTree(repo, id)
	tree.gogitTree = gogitTree
	return tree, nil
}

// GetTree find the tree object in the repository.
func (repo *Repository) GetTree(idStr string) (*Tree, error) {
	if len(idStr) != 40 {
		res, _, err := NewCommand(repo.Ctx, "rev-parse", "--verify").AddDynamicArguments(idStr).RunStdString(&RunOpts{Dir: repo.Path})
		if err != nil {
			return nil, err
		}
		if len(res) > 0 {
			idStr = res[:len(res)-1]
		}
	}
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}
	resolvedID := id
	commitObject, err := repo.gogitRepo.CommitObject(id)
	if err == nil {
		id = SHA1(commitObject.TreeHash)
	}
	treeObject, err := repo.getTree(id)
	if err != nil {
		return nil, err
	}
	treeObject.ResolvedID = resolvedID
	return treeObject, nil
}

// CommitTree creates a commit from a given tree id for the user with provided message
func (repo *Repository) CommitTree(author, committer *Signature, tree *Tree, opts CommitTreeOpts) (SHA1, error) {
	commitTimeStr := time.Now().Format(time.RFC3339)

	// Because this may call hooks we should pass in the environment
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME="+author.Name,
		"GIT_AUTHOR_EMAIL="+author.Email,
		"GIT_AUTHOR_DATE="+commitTimeStr,
		"GIT_COMMITTER_NAME="+committer.Name,
		"GIT_COMMITTER_EMAIL="+committer.Email,
		"GIT_COMMITTER_DATE="+commitTimeStr,
	)
	cmd := NewCommand(repo.Ctx, "commit-tree").AddDynamicArguments(tree.ID.String())

	for _, parent := range opts.Parents {
		cmd.AddArguments("-p").AddDynamicArguments(parent)
	}

	messageBytes := new(bytes.Buffer)
	_, _ = messageBytes.WriteString(opts.Message)
	_, _ = messageBytes.WriteString("\n")

	if opts.KeyID != "" || opts.AlwaysSign {
		cmd.AddArguments(CmdArg(fmt.Sprintf("-S%s", opts.KeyID)))
	}

	if opts.NoGPGSign {
		cmd.AddArguments("--no-gpg-sign")
	}

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	err := cmd.Run(&RunOpts{
		Env:    env,
		Dir:    repo.Path,
		Stdin:  messageBytes,
		Stdout: stdout,
		Stderr: stderr,
	})
	if err != nil {
		return SHA1{}, ConcatenateError(err, stderr.String())
	}
	return NewIDFromString(strings.TrimSpace(stdout.String()))
}