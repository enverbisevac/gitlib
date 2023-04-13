// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"io"
	"path/filepath"
	"strings"

	"github.com/go-enry/go-enry/v2"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

const (
	fileSizeLimit int64 = 16 * 1024   // 16 KiB
	bigFileSize   int64 = 1024 * 1024 // 1 MiB
)

// GetLanguageStats calculates language stats for git repository at specified commit
func (repo *Repository) GetLanguageStats(commitID string) (map[string]int64, error) {
	r, err := git.PlainOpen(repo.Path)
	if err != nil {
		return nil, err
	}

	rev, err := r.ResolveRevision(plumbing.Revision(commitID))
	if err != nil {
		return nil, err
	}

	commit, err := r.CommitObject(*rev)
	if err != nil {
		return nil, err
	}

	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}

	checker, deferable := repo.CheckAttributeReader(commitID)
	defer deferable()

	sizes := make(map[string]int64)
	err = tree.Files().ForEach(func(f *object.File) error {
		if f.Size == 0 {
			return nil
		}

		notVendored := false
		notGenerated := false

		if checker != nil {
			attrs, err := checker.CheckPath(f.Name)
			if err == nil {
				if vendored, has := attrs["linguist-vendored"]; has {
					if vendored == "set" || vendored == "true" {
						return nil
					}
					notVendored = vendored == "false"
				}
				if generated, has := attrs["linguist-generated"]; has {
					if generated == "set" || generated == "true" {
						return nil
					}
					notGenerated = generated == "false"
				}
				if language, has := attrs["linguist-language"]; has && language != "unspecified" && language != "" {
					// group languages, such as Pug -> HTML; SCSS -> CSS
					group := enry.GetLanguageGroup(language)
					if len(group) != 0 {
						language = group
					}

					sizes[language] += f.Size

					return nil
				} else if language, has := attrs["gitlab-language"]; has && language != "unspecified" && language != "" {
					// strip off a ? if present
					if idx := strings.IndexByte(language, '?'); idx >= 0 {
						language = language[:idx]
					}
					if len(language) != 0 {
						// group languages, such as Pug -> HTML; SCSS -> CSS
						group := enry.GetLanguageGroup(language)
						if len(group) != 0 {
							language = group
						}

						sizes[language] += f.Size
						return nil
					}
				}
			}
		}

		if (!notVendored && enry.IsVendor(f.Name)) || enry.IsDotFile(f.Name) ||
			enry.IsDocumentation(f.Name) || enry.IsConfiguration(f.Name) {
			return nil
		}

		// If content can not be read or file is too big just do detection by filename
		var content []byte
		if f.Size <= bigFileSize {
			content, _ = readFile(f, fileSizeLimit)
		}
		if !notGenerated && enry.IsGenerated(f.Name, content) {
			return nil
		}

		// TODO: Use .gitattributes file for linguist overrides

		language := GetCodeLanguage(f.Name, content)
		if language == enry.OtherLanguage || language == "" {
			return nil
		}

		// group languages, such as Pug -> HTML; SCSS -> CSS
		group := enry.GetLanguageGroup(language)
		if group != "" {
			language = group
		}

		sizes[language] += f.Size

		return nil
	})
	if err != nil {
		return nil, err
	}

	// filter special languages unless they are the only language
	if len(sizes) > 1 {
		for language := range sizes {
			langtype := enry.GetLanguageType(language)
			if langtype != enry.Programming && langtype != enry.Markup {
				delete(sizes, language)
			}
		}
	}

	return sizes, nil
}

func readFile(f *object.File, limit int64) ([]byte, error) {
	r, err := f.Reader()
	if err != nil {
		return nil, err
	}
	defer r.Close()

	if limit <= 0 {
		return io.ReadAll(r)
	}

	size := f.Size
	if limit > 0 && size > limit {
		size = limit
	}
	buf := bytes.NewBuffer(nil)
	buf.Grow(int(size))
	_, err = io.Copy(buf, io.LimitReader(r, limit))
	return buf.Bytes(), err
}

// GetCodeLanguage detects code language based on file name and content
func GetCodeLanguage(filename string, content []byte) string {
	if language, ok := enry.GetLanguageByExtension(filename); ok {
		return language
	}

	if language, ok := enry.GetLanguageByFilename(filename); ok {
		return language
	}

	if len(content) == 0 {
		return enry.OtherLanguage
	}

	return enry.GetLanguage(filepath.Base(filename), content)
}
