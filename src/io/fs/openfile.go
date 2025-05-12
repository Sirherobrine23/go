// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fs

import "syscall"

const (
	O_RDONLY int = syscall.O_RDONLY // open the file read-only.
	O_WRONLY int = syscall.O_WRONLY // open the file write-only.
	O_RDWR   int = syscall.O_RDWR   // open the file read-write.
	O_APPEND int = syscall.O_APPEND // append data to the file when writing.
	O_CREATE int = syscall.O_CREAT  // create a new file if none exists.
	O_EXCL   int = syscall.O_EXCL   // used with O_CREATE, file must not exist.
	O_SYNC   int = syscall.O_SYNC   // open for synchronous I/O.
	O_TRUNC  int = syscall.O_TRUNC  // truncate regular writable file when opened.
)

// WriterFile is an interface that combines the File interface with the
// ability to write data and truncate the file.
type WriterFile interface {
	File

	// Write writes len(b) bytes from b to the File.
	// It returns the number of bytes written and an error, if any.
	// Write returns a non-nil error when n != len(b).
	Write([]byte) (int, error)

	// Truncate changes the size of the file.
	// It does not change the I/O offset.
	// If there is an error, it will be of type [*PathError].
	Truncate(size int64) error
}

// WriteSeekFile is the interface that groups the basic WriteFile and
// Seeker interfaces.
type WriteSeekFile interface {
	WriterFile

	Seek(offset int64, whence int) (int64, error)
}

// WriteAtFile is the interface that groups the basic WriterFile and
// the ReadAt and WriteAt methods.
type WriteAtFile interface {
	WriterFile

	// ReadAt reads len(b) bytes from the File starting at byte offset off.
	// It returns the number of bytes read and the error, if any.
	// ReadAt always returns a non-nil error when n < len(b).
	// At end of file, that error is io.EOF.
	ReadAt(b []byte, off int64) (int, error)

	// WriteAt writes len(b) bytes to the File starting at byte offset off.
	// It returns the number of bytes written and an error, if any.
	// WriteAt returns a non-nil error when n != len(b).
	//
	// If file was opened with the O_APPEND flag, WriteAt returns an error.
	WriteAt(b []byte, off int64) (int, error)
}

// OpenFileFS is the interface implemented by a file system
// that provides an OpenFile method that allows opening a file
// with flags and permissions.
type OpenFileFS interface {
	FS

	// OpenFile is the generalized open call; most users will use Open
	// or Create instead. It opens the named file with specified flag
	// ([O_RDONLY] etc.). If the file does not exist, and the [O_CREATE] flag
	// is passed, it is created with mode perm (before umask);
	// the containing directory must exist. If successful,
	// methods on the returned [WriterFile] can be used for I/O.
	// If there is an error, it will be of type *PathError.
	OpenFile(name string, flag int, perm FileMode) (WriterFile, error)
}

func OpenFile(fsys FS, name string, flag int, perm FileMode) (WriterFile, error) {
	sym, ok := fsys.(OpenFileFS)
	if !ok {
		return nil, &PathError{Op: "openfile", Path: name, Err: ErrInvalid}
	}
	return sym.OpenFile(name, flag, perm)
}
