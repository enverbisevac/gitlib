// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"errors"
)

// Common Errors forming the base of our error system
//
// Many Errors returned by Gitea can be tested against these errors
// using errors.Is.
var (
	ErrInvalidArgument  = errors.New("invalid argument")
	ErrPermissionDenied = errors.New("permission denied")
	ErrAlreadyExist     = errors.New("resource already exists")
	ErrNotExist         = errors.New("resource does not exist")
)
