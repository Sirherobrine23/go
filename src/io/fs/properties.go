// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import "time"

// Properties is an interface for file systems that provide
// extended file properties manipulation capabilities, such as
// changing file modes, ownership, and timestamps.
//
// Implementations of this interface allow for more fine-grained
// control over file attributes beyond basic read and write operations.
type PropertiesFS interface {
	FS

	// Chmod changes the mode of the named file to mode.
	// If the file is a symbolic link, it changes the mode of the link's target.
	// If there is an error, it will be of type *PathError.
	Chmod(name string, mode FileMode) error

	// Chown changes the numeric uid and gid of the named file.
	// If the file is a symbolic link, it changes the uid and gid of the link's target.
	// A uid or gid of -1 means to not change that value.
	// If there is an error, it will be of type [*PathError].
	Chown(name string, uid, gid int) error

	// Chtimes changes the access and modification times of the named
	// file, similar to the Unix utime() or utimes() functions.
	// A zero [time.Time] value will leave the corresponding file time unchanged.
	Chtimes(name string, atime time.Time, mtime time.Time) error
}

func Chmod(fsys FS, name string, mode FileMode) error {
	sym, ok := fsys.(PropertiesFS)
	if !ok {
		return &PathError{Op: "chmod", Path: name, Err: ErrInvalid}
	}
	return sym.Chmod(name, mode)
}

func Chown(fsys FS, name string, uid, gid int) error {
	sym, ok := fsys.(PropertiesFS)
	if !ok {
		return &PathError{Op: "chown", Path: name, Err: ErrInvalid}
	}
	return sym.Chown(name, uid, gid)
}

func Chtimes(fsys FS, name string, atime time.Time, mtime time.Time) error {
	sym, ok := fsys.(PropertiesFS)
	if !ok {
		return &PathError{Op: "chtimes", Path: name, Err: ErrInvalid}
	}
	return sym.Chtimes(name, atime, mtime)
}
