// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package process

import (
	"context"
)

// Context is a wrapper around context.Context and contains the current pid for this context
type Context struct {
	context.Context
	pid IDType
}

// GetPID returns the PID for this context
func (c *Context) GetPID() IDType {
	return c.pid
}

// GetParent returns the parent process context (if any)
func (c *Context) GetParent() *Context {
	return GetContext(c.Context)
}

// Value is part of the interface for context.Context. We mostly defer to the internal context - but we return this in response to the ProcessContextKey
func (c *Context) Value(key interface{}) interface{} {
	if key == ProcessContextKey {
		return c
	}
	return c.Context.Value(key)
}

// ProcessContextKey is the key under which process contexts are stored
var ProcessContextKey interface{} = "process-context"

// GetContext will return a process context if one exists
func GetContext(ctx context.Context) *Context {
	if pCtx, ok := ctx.(*Context); ok {
		return pCtx
	}
	pCtxInterface := ctx.Value(ProcessContextKey)
	if pCtxInterface == nil {
		return nil
	}
	if pCtx, ok := pCtxInterface.(*Context); ok {
		return pCtx
	}
	return nil
}

// GetPID returns the PID for this context
func GetPID(ctx context.Context) IDType {
	pCtx := GetContext(ctx)
	if pCtx == nil {
		return ""
	}
	return pCtx.GetPID()
}

// GetParentPID returns the ParentPID for this context
func GetParentPID(ctx context.Context) IDType {
	var parentPID IDType
	if parentProcess := GetContext(ctx); parentProcess != nil {
		parentPID = parentProcess.GetPID()
	}
	return parentPID
}
