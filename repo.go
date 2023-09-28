// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/enverbisevac/gitlib/util"
	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/cache"
	"github.com/go-git/go-git/v5/storage/filesystem"
	git2go "github.com/libgit2/git2go/v34"
)

// GPGSettings represents the default GPG settings for this repository
type GPGSettings struct {
	Sign             bool
	KeyID            string
	Email            string
	Name             string
	PublicKeyContent string
}

const prettyLogFormat = `--pretty=format:%H`

// GetAllCommitsCount returns count of all commits in repository
func (repo *Repository) GetAllCommitsCount() (int64, error) {
	return AllCommitsCount(repo.Ctx, repo.Path, false)
}

func (repo *Repository) parsePrettyFormatLogToList(logs []byte) ([]*Commit, error) {
	var commits []*Commit
	if len(logs) == 0 {
		return commits, nil
	}

	parts := bytes.Split(logs, []byte{'\n'})

	for _, commitID := range parts {
		commit, err := repo.GetCommit(string(commitID))
		if err != nil {
			return nil, err
		}
		commits = append(commits, commit)
	}

	return commits, nil
}

// IsRepoURLAccessible checks if given repository URL is accessible.
func IsRepoURLAccessible(ctx context.Context, url string) bool {
	_, _, err := NewCommand(ctx, "ls-remote", "-q", "-h").AddDynamicArguments(url, "HEAD").RunStdString(nil)
	return err == nil
}

type InitRepositoryConfig struct {
	bare          bool
	defaultBranch string
	description   string
}

type InitRepositoryFunc func(c *InitRepositoryConfig)

func (f InitRepositoryFunc) Apply(c *InitRepositoryConfig) {
	f(c)
}

func InitWithBare(value bool) InitRepositoryFunc {
	return func(c *InitRepositoryConfig) {
		c.bare = value
	}
}

func InitWithDefaultBranch(value string) InitRepositoryFunc {
	return func(c *InitRepositoryConfig) {
		c.defaultBranch = value
	}
}

func InitWithDescription(value string) InitRepositoryFunc {
	return func(c *InitRepositoryConfig) {
		c.description = value
	}
}

type InitRepositoryOption interface {
	Apply(c *InitRepositoryConfig)
}

// InitRepository initializes a new Git repository.
func InitRepository(ctx context.Context, repoPath string, opts ...InitRepositoryOption) (*Repository, error) {
	var wt, dot billy.Filesystem

	c := InitRepositoryConfig{}
	for _, opt := range opts {
		opt.Apply(&c)
	}

	if c.bare {
		dot = osfs.New(repoPath)
	} else {
		wt = osfs.New(repoPath)
		dot, _ = wt.Chroot(gogit.GitDirName)
	}

	s := filesystem.NewStorage(dot, cache.NewObjectLRUDefault())

	if c.defaultBranch == "" {
		c.defaultBranch = "main"
	}

	if !strings.Contains(c.defaultBranch, "refs/heads") {
		c.defaultBranch = "refs/heads/" + c.defaultBranch
	}

	// gogit
	repo, err := gogit.InitWithOptions(s, wt, gogit.InitOptions{
		DefaultBranch: plumbing.ReferenceName(c.defaultBranch),
	})
	if err != nil {
		return nil, err
	}

	//libgit2
	git2gorepo, err := git2go.OpenRepository(repoPath)
	if err != nil {
		return nil, err
	}

	f, err := os.Create(path.Join(repoPath, "description"))
	if err == nil {
		defer f.Close()
		f.WriteString(c.description)
		f.Sync()
	} else {
		log.Printf("error writing description file for repository '%s'", repoPath)
	}

	return &Repository{
		Path:     repoPath,
		gogit:    repo,
		git2go:   git2gorepo,
		storage:  s,
		tagCache: newObjectCache(),
		Ctx:      ctx,
	}, nil
}

// IsEmpty Check if repository is empty.
func (repo *Repository) IsEmpty() (bool, error) {
	_, err := repo.gogit.Head()
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			return true, nil
		}
		return false, err
	}

	return false, nil
}

// CloneRepoOptions options when clone a repository
type CloneRepoOptions struct {
	Timeout       time.Duration
	Mirror        bool
	Bare          bool
	Quiet         bool
	Branch        string
	Shared        bool
	NoCheckout    bool
	Depth         int
	Filter        string
	SkipTLSVerify bool
}

// Clone clones original repository to target path.
func Clone(ctx context.Context, from, to string, opts CloneRepoOptions) error {
	return CloneWithArgs(ctx, globalCommandArgs, from, to, opts)
}

// CloneWithArgs original repository to target path.
func CloneWithArgs(ctx context.Context, args []CmdArg, from, to string, opts CloneRepoOptions) (err error) {
	toDir := path.Dir(to)
	if err = os.MkdirAll(toDir, os.ModePerm); err != nil {
		return err
	}

	cmd := NewCommandContextNoGlobals(ctx, args...).AddArguments("clone")
	if opts.SkipTLSVerify {
		cmd.AddArguments("-c", "http.sslVerify=false")
	}
	if opts.Mirror {
		cmd.AddArguments("--mirror")
	}
	if opts.Bare {
		cmd.AddArguments("--bare")
	}
	if opts.Quiet {
		cmd.AddArguments("--quiet")
	}
	if opts.Shared {
		cmd.AddArguments("-s")
	}
	if opts.NoCheckout {
		cmd.AddArguments("--no-checkout")
	}
	if opts.Depth > 0 {
		cmd.AddArguments("--depth").AddDynamicArguments(strconv.Itoa(opts.Depth))
	}
	if opts.Filter != "" {
		cmd.AddArguments("--filter").AddDynamicArguments(opts.Filter)
	}
	if len(opts.Branch) > 0 {
		cmd.AddArguments("-b").AddDynamicArguments(opts.Branch)
	}
	cmd.AddDashesAndList(from, to)

	if strings.Contains(from, "://") && strings.Contains(from, "@") {
		cmd.SetDescription(fmt.Sprintf("clone branch %s from %s to %s (shared: %t, mirror: %t, depth: %d)", opts.Branch, util.SanitizeCredentialURLs(from), to, opts.Shared, opts.Mirror, opts.Depth))
	} else {
		cmd.SetDescription(fmt.Sprintf("clone branch %s from %s to %s (shared: %t, mirror: %t, depth: %d)", opts.Branch, from, to, opts.Shared, opts.Mirror, opts.Depth))
	}

	if opts.Timeout <= 0 {
		opts.Timeout = -1
	}

	envs := os.Environ()
	u, err := url.Parse(from)
	if err == nil && (strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https")) {
		if Match(u.Host) {
			envs = append(envs, fmt.Sprintf("https_proxy=%s", GetProxyURL()))
		}
	}

	stderr := new(bytes.Buffer)
	if err = cmd.Run(&RunOpts{
		Timeout: opts.Timeout,
		Env:     envs,
		Stdout:  io.Discard,
		Stderr:  stderr,
	}); err != nil {
		err = ConcatenateError(err, stderr.String())
		if matched, _ := regexp.MatchString(".*Remote branch .* not found in upstream origin.*", err.Error()); matched {
			return ErrBranchNotExist{
				Name: opts.Branch,
			}
		} else if matched, _ := regexp.MatchString(".* repository .* does not exist.*", err.Error()); matched {
			return fmt.Errorf("repository not found: %w", err)
		} else {
			return fmt.Errorf("error while cloning repository: %w", err)
		}
	}
	return nil
}

// PushOptions options when push to remote
type PushOptions struct {
	Remote  string
	Branch  string
	Force   bool
	Mirror  bool
	Env     []string
	Timeout time.Duration
}

// Push pushs local commits to given remote branch.
func Push(ctx context.Context, repoPath string, opts PushOptions) error {
	cmd := NewCommand(ctx, "push")
	if opts.Force {
		cmd.AddArguments("-f")
	}
	if opts.Mirror {
		cmd.AddArguments("--mirror")
	}
	remoteBranchArgs := []string{opts.Remote}
	if len(opts.Branch) > 0 {
		remoteBranchArgs = append(remoteBranchArgs, opts.Branch)
	}
	cmd.AddDashesAndList(remoteBranchArgs...)

	if strings.Contains(opts.Remote, "://") && strings.Contains(opts.Remote, "@") {
		cmd.SetDescription(fmt.Sprintf("push branch %s to %s (force: %t, mirror: %t)", opts.Branch, util.SanitizeCredentialURLs(opts.Remote), opts.Force, opts.Mirror))
	} else {
		cmd.SetDescription(fmt.Sprintf("push branch %s to %s (force: %t, mirror: %t)", opts.Branch, opts.Remote, opts.Force, opts.Mirror))
	}
	var outbuf, errbuf strings.Builder

	if opts.Timeout == 0 {
		opts.Timeout = -1
	}

	err := cmd.Run(&RunOpts{
		Env:     opts.Env,
		Timeout: opts.Timeout,
		Dir:     repoPath,
		Stdout:  &outbuf,
		Stderr:  &errbuf,
	})
	if err != nil {
		if strings.Contains(errbuf.String(), "non-fast-forward") {
			return &ErrPushOutOfDate{
				StdOut: outbuf.String(),
				StdErr: errbuf.String(),
				Err:    err,
			}
		} else if strings.Contains(errbuf.String(), "! [remote rejected]") {
			err := &ErrPushRejected{
				StdOut: outbuf.String(),
				StdErr: errbuf.String(),
				Err:    err,
			}
			err.GenerateMessage()
			return err
		} else if strings.Contains(errbuf.String(), "matches more than one") {
			err := &ErrMoreThanOne{
				StdOut: outbuf.String(),
				StdErr: errbuf.String(),
				Err:    err,
			}
			return err
		}
	}

	if errbuf.Len() > 0 && err != nil {
		return fmt.Errorf("%w - %s", err, errbuf.String())
	}

	return err
}

// GetLatestCommitTime returns time for latest commit in repository (across all branches)
func GetLatestCommitTime(ctx context.Context, repoPath string) (time.Time, error) {
	cmd := NewCommand(ctx, "for-each-ref", "--sort=-committerdate", BranchPrefix, "--count", "1", "--format=%(committerdate)")
	stdout, _, err := cmd.RunStdString(&RunOpts{Dir: repoPath})
	if err != nil {
		return time.Time{}, err
	}
	commitTime := strings.TrimSpace(stdout)
	return time.Parse(GitTimeLayout, commitTime)
}

// DivergeObject represents commit count diverging commits
type DivergeObject struct {
	Ahead  int
	Behind int
}

func checkDivergence(ctx context.Context, repoPath, baseBranch, targetBranch string) (int, error) {
	branches := fmt.Sprintf("%s..%s", baseBranch, targetBranch)
	cmd := NewCommand(ctx, "rev-list", "--count").AddDynamicArguments(branches)
	stdout, _, err := cmd.RunStdString(&RunOpts{Dir: repoPath})
	if err != nil {
		return -1, err
	}
	outInteger, errInteger := strconv.Atoi(strings.Trim(stdout, "\n"))
	if errInteger != nil {
		return -1, errInteger
	}
	return outInteger, nil
}

// GetDivergingCommits returns the number of commits a targetBranch is ahead or behind a baseBranch
func GetDivergingCommits(ctx context.Context, repoPath, baseBranch, targetBranch string) (DivergeObject, error) {
	// $(git rev-list --count master..feature) commits ahead of master
	ahead, errorAhead := checkDivergence(ctx, repoPath, baseBranch, targetBranch)
	if errorAhead != nil {
		return DivergeObject{}, errorAhead
	}

	// $(git rev-list --count feature..master) commits behind master
	behind, errorBehind := checkDivergence(ctx, repoPath, targetBranch, baseBranch)
	if errorBehind != nil {
		return DivergeObject{}, errorBehind
	}

	return DivergeObject{ahead, behind}, nil
}

// CreateBundle create bundle content to the target path
func (repo *Repository) CreateBundle(ctx context.Context, commit string, out io.Writer) error {
	tmp, err := os.MkdirTemp(os.TempDir(), "gitlib-bundle")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	env := append(os.Environ(), "GIT_OBJECT_DIRECTORY="+filepath.Join(repo.Path, "objects"))
	_, _, err = NewCommand(ctx, "init", "--bare").RunStdString(&RunOpts{Dir: tmp, Env: env})
	if err != nil {
		return err
	}

	_, _, err = NewCommand(ctx, "reset", "--soft").AddDynamicArguments(commit).RunStdString(&RunOpts{Dir: tmp, Env: env})
	if err != nil {
		return err
	}

	_, _, err = NewCommand(ctx, "branch", "-m", "bundle").RunStdString(&RunOpts{Dir: tmp, Env: env})
	if err != nil {
		return err
	}

	tmpFile := filepath.Join(tmp, "bundle")
	_, _, err = NewCommand(ctx, "bundle", "create").AddDynamicArguments(tmpFile, "bundle", "HEAD").RunStdString(&RunOpts{Dir: tmp, Env: env})
	if err != nil {
		return err
	}

	fi, err := os.Open(tmpFile)
	if err != nil {
		return err
	}
	defer fi.Close()

	_, err = io.Copy(out, fi)
	return err
}
