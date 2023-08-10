// Copyright 2015 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"bytes"
	"encoding/base64"
	"io"

	"github.com/enverbisevac/gitlib/typesniffer"
	"github.com/enverbisevac/gitlib/util"
	"github.com/go-git/go-git/v5/plumbing"
)

// Blob represents a Git object.
type Blob struct {
	ID SHA1

	obj  plumbing.EncodedObject
	name string
}

// DataAsync gets a ReadCloser for the contents of a blob without reading it all.
// Calling the Close function on the result will discard all unread output.
func (b *Blob) DataAsync() (io.ReadCloser, error) {
	return b.obj.Reader()
}

// Size returns the uncompressed size of the blob
func (b *Blob) Size() int64 {
	return b.obj.Size()
}

// Name returns name of the tree entry this blob object was created from (or empty string)
func (b *Blob) Name() string {
	return b.name
}

// GetBlobContent Gets the content of the blob as raw text
func (b *Blob) GetBlobContent() (string, error) {
	dataRc, err := b.DataAsync()
	if err != nil {
		return "", err
	}
	defer dataRc.Close()
	buf := make([]byte, 1024)
	n, _ := util.ReadAtMost(dataRc, buf)
	buf = buf[:n]
	return string(buf), nil
}

// GetBlobLineCount gets line count of the blob
func (b *Blob) GetBlobLineCount() (int, error) {
	reader, err := b.DataAsync()
	if err != nil {
		return 0, err
	}
	defer reader.Close()
	buf := make([]byte, 32*1024)
	count := 1
	lineSep := []byte{'\n'}

	c, err := reader.Read(buf)
	if c == 0 && err == io.EOF {
		return 0, nil
	}
	for {
		count += bytes.Count(buf[:c], lineSep)
		switch {
		case err == io.EOF:
			return count, nil
		case err != nil:
			return count, err
		}
		c, err = reader.Read(buf)
	}
}

// GetBlobContentBase64 Reads the content of the blob with a base64 encode and returns the encoded string
func (b *Blob) GetBlobContentBase64() (string, error) {
	dataRc, err := b.DataAsync()
	if err != nil {
		return "", err
	}
	defer dataRc.Close()

	pr, pw := io.Pipe()
	encoder := base64.NewEncoder(base64.StdEncoding, pw)

	go func() {
		_, err := io.Copy(encoder, dataRc)
		_ = encoder.Close()

		if err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
	}()

	out, err := io.ReadAll(pr)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// GuessContentType guesses the content type of the blob.
func (b *Blob) GuessContentType() (typesniffer.SniffedType, error) {
	r, err := b.DataAsync()
	if err != nil {
		return typesniffer.SniffedType{}, err
	}
	defer r.Close()

	return typesniffer.DetectContentTypeFromReader(r)
}
