package git

import (
	"io"

	"github.com/go-git/go-git/v5/plumbing"
)

// ObjectType git object type
type ObjectType string

const (
	// ObjectCommit commit object type
	ObjectCommit ObjectType = "commit"
	// ObjectTree tree object type
	ObjectTree ObjectType = "tree"
	// ObjectBlob blob object type
	ObjectBlob ObjectType = "blob"
	// ObjectTag tag object type
	ObjectTag ObjectType = "tag"
	// ObjectBranch branch object type
	ObjectBranch ObjectType = "branch"
)

// Bytes returns the byte array for the Object Type
func (o ObjectType) Bytes() []byte {
	return []byte(o)
}

// HashObject takes a reader and returns SHA1 hash for that reader
func (repo *Repository) HashObject(reader io.Reader) (SHA1, error) {
	idStr, err := repo.hashObject(reader)
	if err != nil {
		return SHA1{}, err
	}
	return NewIDFromString(idStr)
}

func (repo *Repository) hashObject(reader io.Reader) (string, error) {
	obj := repo.gogit.Storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)

	w, err := obj.Writer()
	if err != nil {
		return "", err
	}

	_, err = io.Copy(w, reader)
	if err != nil {
		return "", err
	}

	err = w.Close()
	if err != nil {
		return "", err
	}

	h, err := repo.gogit.Storer.SetEncodedObject(obj)
	if err != nil {
		return "", err
	}

	return h.String(), nil
}

// GetRefType gets the type of the ref based on the string
func (repo *Repository) GetRefType(ref string) ObjectType {
	if repo.IsTagExist(ref) {
		return ObjectTag
	}
	if repo.IsBranchExist(ref) {
		return ObjectBranch
	}
	if repo.IsCommitExist(ref) {
		return ObjectCommit
	}
	if _, err := repo.GetBlob(ref); err == nil {
		return ObjectBlob
	}
	return ObjectType("invalid")
}
