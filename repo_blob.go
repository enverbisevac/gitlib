// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import "github.com/go-git/go-git/v5/plumbing"

func (repo *Repository) getBlob(id SHA1) (*Blob, error) {
	encodedObj, err := repo.gogit.Storer.EncodedObject(plumbing.AnyObject, id)
	if err != nil {
		return nil, ErrNotExist{id.String(), ""}
	}

	return &Blob{
		ID:  id,
		obj: encodedObj,
	}, nil
}

// GetBlob finds the blob object in the repository.
func (repo *Repository) GetBlob(idStr string) (*Blob, error) {
	id, err := NewIDFromString(idStr)
	if err != nil {
		return nil, err
	}
	return repo.getBlob(id)
}
