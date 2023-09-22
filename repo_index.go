// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/enverbisevac/gitlib/log"
	"github.com/enverbisevac/gitlib/util"
)

// ReadTreeToIndex reads a treeish to the index
func (repo *Repository) ReadTreeToIndex(treeish string, indexFilename ...string) error {
	if len(treeish) != 40 {
		res, _, err := NewCommand(repo.Ctx, "rev-parse", "--verify").AddDynamicArguments(treeish).RunStdString(&RunOpts{Dir: repo.Path})
		if err != nil {
			return err
		}
		if len(res) > 0 {
			treeish = res[:len(res)-1]
		}
	}
	id, err := NewIDFromString(treeish)
	if err != nil {
		return err
	}
	return repo.readTreeToIndex(id, indexFilename...)
}

func (repo *Repository) readTreeToIndex(id SHA1, indexFilename ...string) error {
	var env []string
	if len(indexFilename) > 0 {
		env = append(os.Environ(), "GIT_INDEX_FILE="+indexFilename[0])
	}
	_, _, err := NewCommand(repo.Ctx, "read-tree").AddDynamicArguments(id.String()).RunStdString(&RunOpts{Dir: repo.Path, Env: env})
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
	cmd := NewCommand(repo.Ctx, "ls-files", "-z").AddDashesAndList(filenames...)
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

// RemoveFilesFromIndex removes given filenames from the index - it does not check whether they are present.
func (repo *Repository) RemoveFilesFromIndex(filenames ...string) error {
	cmd := NewCommand(repo.Ctx, "update-index", "--remove", "-z", "--index-info")
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	buffer := new(bytes.Buffer)
	for _, file := range filenames {
		if file != "" {
			buffer.WriteString("0 0000000000000000000000000000000000000000\t")
			buffer.WriteString(file)
			buffer.WriteByte('\000')
		}
	}
	return cmd.Run(&RunOpts{
		Dir:    repo.Path,
		Stdin:  bytes.NewReader(buffer.Bytes()),
		Stdout: stdout,
		Stderr: stderr,
	})
}

// AddObjectToIndex adds the provided object hash to the index at the provided filename
func (repo *Repository) AddObjectToIndex(mode string, object SHA1, filename string) error {
	cmd := NewCommand(repo.Ctx, "update-index", "--add", "--replace", "--cacheinfo").AddDynamicArguments(mode, object.String(), filename)
	_, stderr, err := cmd.RunStdString(&RunOpts{Dir: repo.Path})
	if err != nil {
		if matched, _ := regexp.MatchString(".*Invalid path '.*", stderr); matched {
			return ErrFilePathInvalid{
				Message: filename,
				Path:    filename,
			}
		}
		return fmt.Errorf("unable to add object to index at %s in repo %s Error: %w - %s", object, repo.Path, err, stderr)
	}

	return nil
}

// WriteTree writes the current index as a tree to the object db and returns its hash
func (repo *Repository) WriteTree() (*Tree, error) {
	stdout, _, runErr := NewCommand(repo.Ctx, "write-tree").RunStdString(&RunOpts{Dir: repo.Path})
	if runErr != nil {
		return nil, runErr
	}
	id, err := NewIDFromString(strings.TrimSpace(stdout))
	if err != nil {
		return nil, err
	}
	return NewTree(repo, id), nil
}
