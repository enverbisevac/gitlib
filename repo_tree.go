package git

import (
	"bytes"

	git2go "github.com/libgit2/git2go/v34"
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
	gogitTree, err := repo.gogit.TreeObject(id)
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
	commitObject, err := repo.gogit.CommitObject(id)
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
	oid, err := git2go.NewOid(tree.ID.String())
	if err != nil {
		return SHA1{}, err
	}

	t, err := repo.git2go.LookupTree(oid)
	if err != nil {
		return SHA1{}, err
	}

	parents := make([]*git2go.Commit, 0, len(opts.Parents))
	// for i, parent := range opts.Parents {
	// 	oidf, err := git2go
	// 	parents[i] =
	// }

	oid, err = repo.git2go.CreateCommit("HEAD",
		&git2go.Signature{
			Name:  author.Name,
			Email: author.Email,
			When:  author.When,
		}, &git2go.Signature{
			Name:  committer.Name,
			Email: committer.Email,
			When:  committer.When,
		},
		opts.Message,
		t,
		parents...,
	)
	if err != nil {
		return SHA1{}, err
	}
	sha1, err := NewIDFromString(oid.String())
	if err != nil {
		return SHA1{}, err
	}
	return sha1, nil

	// commitTimeStr := time.Now().Format(time.RFC3339)

	// // Because this may call hooks we should pass in the environment
	// env := append(os.Environ(),
	// 	"GIT_AUTHOR_NAME="+author.Name,
	// 	"GIT_AUTHOR_EMAIL="+author.Email,
	// 	"GIT_AUTHOR_DATE="+commitTimeStr,
	// 	"GIT_COMMITTER_NAME="+committer.Name,
	// 	"GIT_COMMITTER_EMAIL="+committer.Email,
	// 	"GIT_COMMITTER_DATE="+commitTimeStr,
	// )
	// cmd := NewCommand(repo.Ctx, "commit-tree").AddDynamicArguments(tree.ID.String())

	// for _, parent := range opts.Parents {
	// 	cmd.AddArguments("-p").AddDynamicArguments(parent)
	// }

	// messageBytes := new(bytes.Buffer)
	// _, _ = messageBytes.WriteString(opts.Message)
	// _, _ = messageBytes.WriteString("\n")

	// if opts.KeyID != "" || opts.AlwaysSign {
	// 	cmd.AddArguments(CmdArg(fmt.Sprintf("-S%s", opts.KeyID)))
	// }

	// if opts.NoGPGSign {
	// 	cmd.AddArguments("--no-gpg-sign")
	// }

	// stdout := new(bytes.Buffer)
	// stderr := new(bytes.Buffer)
	// err := cmd.Run(&RunOpts{
	// 	Env:    env,
	// 	Dir:    repo.Path,
	// 	Stdin:  messageBytes,
	// 	Stdout: stdout,
	// 	Stderr: stderr,
	// })
	// if err != nil {
	// 	return SHA1{}, ConcatenateError(err, stderr.String())
	// }
	// return NewIDFromString(strings.TrimSpace(stdout.String()))
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
