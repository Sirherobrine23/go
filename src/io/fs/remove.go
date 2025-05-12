// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

// RemoveFS is the interface implemented that supporter
// to remove Folder or File.
type RemoveFS interface {
	FS

	// Remove removes the named file or (empty) directory.
	// If there is an error, it will be of type [*PathError].
	Remove(name string) error

	// RemoveAll removes path and any children it contains.
	// It removes everything it can but returns the first error
	// it encounters. If the path does not exist, RemoveAll
	// returns nil (no error).
	// If there is an error, it will be of type [*PathError].
	RemoveAll(name string) error
}

func Remove(fsys FS, name string) error {
	sym, ok := fsys.(RemoveFS)
	if !ok {
		return &PathError{Op: "remove", Path: name, Err: ErrInvalid}
	}
	return sym.Remove(name)
}

func RemoveAll(fsys FS, name string) error {
	sym, ok := fsys.(RemoveFS)
	if !ok {
		return &PathError{Op: "RemoveAll", Path: name, Err: ErrInvalid}
	}
	return sym.RemoveAll(name)
}
