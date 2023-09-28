// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"fmt"
	"io"
	"strings"

	"github.com/enverbisevac/gitlib/foreachref"
	"github.com/enverbisevac/gitlib/log"
	"github.com/enverbisevac/gitlib/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// TagPrefix tags prefix path on the repository
const TagPrefix = "refs/tags/"

// CreateTag create one tag in the repository
func (repo *Repository) CreateTag(name, revision string) error {
	_, err := repo.gogit.CreateTag(name, plumbing.NewHash(revision), nil)
	return err
}

// CreateAnnotatedTag create one annotated tag in the repository
func (repo *Repository) CreateAnnotatedTag(name, message, revision string) error {
	_, err := repo.gogit.CreateTag(name, plumbing.NewHash(revision), &git.CreateTagOptions{Message: message})
	return err
}

// GetTagNameBySHA returns the name of a tag from its tag object SHA or commit SHA
func (repo *Repository) GetTagNameBySHA(sha string) (s string, err error) {
	if len(sha) < 5 {
		return "", fmt.Errorf("SHA is too short: %s", sha)
	}

	iter, err := repo.gogit.Tags()
	if err != nil {
		return "", err
	}

	if err := iter.ForEach(func(ref *plumbing.Reference) error {
		tag, err := repo.gogit.TagObject(plumbing.NewHash(sha))
		switch err {
		case nil:
			s = strings.TrimPrefix(tag.Name, TagPrefix)
			return nil
		case plumbing.ErrObjectNotFound:
			return ErrNotExist{ID: sha}
		default:
			return err
		}
	}); err != nil {
		return "", err
	}
	return s, nil
}

// GetTagID returns the object ID for a tag (annotated tags have both an object SHA AND a commit SHA)
func (repo *Repository) GetTagID(name string) (string, error) {
	ref, err := repo.gogit.Tag(name)
	if err != nil {
		return "", err
	}
	return ref.Hash().String(), nil
}

// GetTag returns a Git tag by given name.
func (repo *Repository) GetTag(name string) (*Tag, error) {
	idStr, err := repo.GetTagID(name)
	if err != nil {
		return nil, err
	}

	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}

	tag, err := repo.getTag(id, name)
	if err != nil {
		return nil, err
	}
	return tag, nil
}

// GetTagWithID returns a Git tag by given name and ID
func (repo *Repository) GetTagWithID(idStr, name string) (*Tag, error) {
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}

	tag, err := repo.getTag(id, name)
	if err != nil {
		return nil, err
	}
	return tag, nil
}

// GetTagInfos returns all tag infos of the repository.
func (repo *Repository) GetTagInfos(page, pageSize int) ([]*Tag, int, error) {
	forEachRefFmt := foreachref.NewFormat("objecttype", "refname:short", "object", "objectname", "creator", "contents", "contents:signature")

	stdoutReader, stdoutWriter := io.Pipe()
	defer stdoutReader.Close()
	defer stdoutWriter.Close()
	stderr := strings.Builder{}
	rc := &RunOpts{Dir: repo.Path, Stdout: stdoutWriter, Stderr: &stderr}

	go func() {
		err := NewCommand(repo.Ctx, "for-each-ref", CmdArg("--format="+forEachRefFmt.Flag()), "--sort", "-*creatordate", "refs/tags").Run(rc)
		if err != nil {
			_ = stdoutWriter.CloseWithError(ConcatenateError(err, stderr.String()))
		} else {
			_ = stdoutWriter.Close()
		}
	}()

	var tags []*Tag
	parser := forEachRefFmt.Parser(stdoutReader)
	for {
		ref := parser.Next()
		if ref == nil {
			break
		}

		tag, err := parseTagRef(ref)
		if err != nil {
			return nil, 0, fmt.Errorf("GetTagInfos: parse tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := parser.Err(); err != nil {
		return nil, 0, fmt.Errorf("GetTagInfos: parse output: %w", err)
	}

	sortTagsByTime(tags)
	tagsTotal := len(tags)
	if page != 0 {
		tags = util.PaginateSlice(tags, page, pageSize).([]*Tag)
	}

	return tags, tagsTotal, nil
}

// parseTagRef parses a tag from a 'git for-each-ref'-produced reference.
func parseTagRef(ref map[string]string) (tag *Tag, err error) {
	tag = &Tag{
		Type: ref["objecttype"],
		Name: ref["refname:short"],
	}

	tag.ID, err = NewIDFromString(ref["objectname"])
	if err != nil {
		return nil, fmt.Errorf("parse objectname '%s': %w", ref["objectname"], err)
	}

	if tag.Type == "commit" {
		// lightweight tag
		tag.Object = tag.ID
	} else {
		// annotated tag
		tag.Object, err = NewIDFromString(ref["object"])
		if err != nil {
			return nil, fmt.Errorf("parse object '%s': %w", ref["object"], err)
		}
	}

	tag.Tagger, err = newSignatureFromCommitline([]byte(ref["creator"]))
	if err != nil {
		return nil, fmt.Errorf("parse tagger: %w", err)
	}

	tag.Message = ref["contents"]
	// strip PGP signature if present in contents field
	pgpStart := strings.Index(tag.Message, beginpgp)
	if pgpStart >= 0 {
		tag.Message = tag.Message[0:pgpStart]
	}

	// annotated tag with GPG signature
	if tag.Type == "tag" && ref["contents:signature"] != "" {
		payload := fmt.Sprintf("object %s\ntype commit\ntag %s\ntagger %s\n\n%s\n",
			tag.Object, tag.Name, ref["creator"], strings.TrimSpace(tag.Message))
		tag.Signature = &CommitGPGSignature{
			Signature: ref["contents:signature"],
			Payload:   payload,
		}
	}

	return tag, nil
}

// GetAnnotatedTag returns a Git tag by its SHA, must be an annotated tag
func (repo *Repository) GetAnnotatedTag(sha string) (*Tag, error) {
	id, err := NewIDFromString(sha)
	if err != nil {
		return nil, err
	}

	// Tag type must be "tag" (annotated) and not a "commit" (lightweight) tag
	if tagType, err := repo.GetTagType(id); err != nil {
		return nil, err
	} else if ObjectType(tagType) != ObjectTag {
		// not an annotated tag
		return nil, ErrNotExist{ID: id.String()}
	}

	// Get tag name
	name, err := repo.GetTagNameBySHA(id.String())
	if err != nil {
		return nil, err
	}

	tag, err := repo.getTag(id, name)
	if err != nil {
		return nil, err
	}
	return tag, nil
}

// IsTagExist returns true if given tag exists in the repository.
func (repo *Repository) IsTagExist(name string) bool {
	_, err := repo.gogit.Reference(plumbing.ReferenceName(TagPrefix+name), true)
	return err == nil
}

// GetTags returns all tags of the repository.
// returning at most limit tags, or all if limit is 0.
func (repo *Repository) GetTags(skip, limit int) ([]string, error) {
	var tagNames []string

	tags, err := repo.gogit.Tags()
	if err != nil {
		return nil, err
	}

	_ = tags.ForEach(func(tag *plumbing.Reference) error {
		tagNames = append(tagNames, strings.TrimPrefix(tag.Name().String(), TagPrefix))
		return nil
	})

	// Reverse order
	for i := 0; i < len(tagNames)/2; i++ {
		j := len(tagNames) - i - 1
		tagNames[i], tagNames[j] = tagNames[j], tagNames[i]
	}

	// since we have to reverse order we can paginate only afterwards
	if len(tagNames) < skip {
		tagNames = []string{}
	} else {
		tagNames = tagNames[skip:]
	}
	if limit != 0 && len(tagNames) > limit {
		tagNames = tagNames[:limit]
	}

	return tagNames, nil
}

// GetTagType gets the type of the tag, either commit (simple) or tag (annotated)
func (repo *Repository) GetTagType(id SHA1) (string, error) {
	// Get tag type
	obj, err := repo.gogit.Object(plumbing.AnyObject, id)
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return "", &ErrNotExist{ID: id.String()}
		}
		return "", err
	}

	return obj.Type().String(), nil
}

func (repo *Repository) getTag(tagID SHA1, name string) (*Tag, error) {
	t, ok := repo.tagCache.Get(tagID.String())
	if ok {
		log.Info("Hit cache: %s", tagID)
		tagClone := *t.(*Tag)
		tagClone.Name = name // This is necessary because lightweight tags may have same id
		return &tagClone, nil
	}

	tp, err := repo.GetTagType(tagID)
	if err != nil {
		return nil, err
	}

	// Get the commit ID and tag ID (may be different for annotated tag) for the returned tag object
	commitIDStr, err := repo.GetTagCommitID(name)
	if err != nil {
		// every tag should have a commit ID so return all errors
		return nil, err
	}
	commitID, err := NewIDFromString(commitIDStr)
	if err != nil {
		return nil, err
	}

	// If type is "commit, the tag is a lightweight tag
	if ObjectType(tp) == ObjectCommit {
		commit, err := repo.GetCommit(commitIDStr)
		if err != nil {
			return nil, err
		}
		tag := &Tag{
			Name:    name,
			ID:      tagID,
			Object:  commitID,
			Type:    tp,
			Tagger:  commit.Committer,
			Message: commit.Message(),
		}

		repo.tagCache.Set(tagID.String(), tag)
		return tag, nil
	}

	gogitTag, err := repo.gogit.TagObject(tagID)
	if err != nil {
		if err == plumbing.ErrReferenceNotFound {
			return nil, &ErrNotExist{ID: tagID.String()}
		}

		return nil, err
	}

	tag := &Tag{
		Name:    name,
		ID:      tagID,
		Object:  gogitTag.Target,
		Type:    tp,
		Tagger:  &gogitTag.Tagger,
		Message: gogitTag.Message,
	}

	repo.tagCache.Set(tagID.String(), tag)
	return tag, nil
}
