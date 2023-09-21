// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"crypto/aes"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAESGCM(t *testing.T) {
	t.Parallel()

	key := make([]byte, aes.BlockSize)
	_, err := rand.Read(key)
	assert.NoError(t, err)

	plaintext := []byte("this will be encrypted")

	ciphertext, err := AESGCMEncrypt(key, plaintext)
	assert.NoError(t, err)

	decrypted, err := AESGCMDecrypt(key, ciphertext)
	assert.NoError(t, err)

	assert.Equal(t, plaintext, decrypted)
}
