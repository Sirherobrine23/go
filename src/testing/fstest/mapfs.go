// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fstest

import (
	"io"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"
)

// A MapFS is a simple in-memory file system for use in tests,
// represented as a map from path names (arguments to Open)
// to information about the files, directories, or symbolic links they represent.
//
// The map need not include parent directories for files contained
// in the map; those will be synthesized if needed.
// But a directory can still be included by setting the [MapFile.Mode]'s [fs.ModeDir] bit;
// this may be necessary for detailed control over the directory's [fs.FileInfo]
// or to create an empty directory.
//
// File system operations read directly from the map,
// so that the file system can be changed by editing the map as needed.
// An implication is that file system operations must not run concurrently
// with changes to the map, which would be a race.
// Another implication is that opening or reading a directory requires
// iterating over the entire map, so a MapFS should typically be used with not more
// than a few hundred entries or directory reads.
type MapFS map[string]*MapFile

// A MapFile describes a single file in a [MapFS].
type MapFile struct {
	Data     []byte      // file content or symlink destination
	Mode     fs.FileMode // fs.FileInfo.Mode
	ModTime  time.Time   // fs.FileInfo.ModTime
	Sys      any         // fs.FileInfo.Sys
	UID, GID int
}

var _ fs.FS = MapFS(nil)
var _ fs.ReadLinkFS = MapFS(nil)
var _ fs.File = (*openMapFile)(nil)

// Open opens the named file after following any symbolic links.
func (fsys MapFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	realName, ok := fsys.resolveSymlinks(name)
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	file := fsys[realName]
	if file != nil && file.Mode&fs.ModeDir == 0 {
		// Ordinary file
		return &openMapFile{name, mapFileInfo{path.Base(name), file}, 0}, nil
	}

	// Directory, possibly synthesized.
	// Note that file can be nil here: the map need not contain explicit parent directories for all its files.
	// But file can also be non-nil, in case the user wants to set metadata for the directory explicitly.
	// Either way, we need to construct the list of children of this directory.
	var list []mapFileInfo
	var need = make(map[string]bool)
	if realName == "." {
		for fname, f := range fsys {
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
					list = append(list, mapFileInfo{fname, f})
				}
			} else {
				need[fname[:i]] = true
			}
		}
	} else {
		prefix := realName + "/"
		for fname, f := range fsys {
			if strings.HasPrefix(fname, prefix) {
				felem := fname[len(prefix):]
				i := strings.Index(felem, "/")
				if i < 0 {
					list = append(list, mapFileInfo{felem, f})
				} else {
					need[fname[len(prefix):len(prefix)+i]] = true
				}
			}
		}
		// If the directory name is not in the map,
		// and there are no children of the name in the map,
		// then the directory is treated as not existing.
		if file == nil && list == nil && len(need) == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
	}
	for _, fi := range list {
		delete(need, fi.name)
	}
	for name := range need {
		list = append(list, mapFileInfo{name, &MapFile{Mode: fs.ModeDir | 0555}})
	}
	slices.SortFunc(list, func(a, b mapFileInfo) int {
		return strings.Compare(a.name, b.name)
	})

	if file == nil {
		file = &MapFile{Mode: fs.ModeDir | 0555}
	}
	var elem string
	if name == "." {
		elem = "."
	} else {
		elem = name[strings.LastIndex(name, "/")+1:]
	}
	return &mapDir{name, mapFileInfo{elem, file}, list, 0}, nil
}

func (fsys MapFS) OpenFile(name string, flag int, perm fs.FileMode) (fs.WriterFile, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	realName, ok := fsys.resolveSymlinks(name)
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}

	file := fsys[realName]
	if file != nil && file.Mode&fs.ModeDir == 0 {
		// Ordinary file
		return &openMapFile{name, mapFileInfo{path.Base(name), file}, 0}, nil
	} else if flag&fs.O_CREATE > 0 && perm&fs.ModeDir == 0 {
		file = &MapFile{[]byte{}, perm, time.Now(), nil, 0, 0}
		fsys[realName] = file
		return &openMapFile{name, mapFileInfo{path.Base(name), file}, 0}, nil
	}

	// Directory, possibly synthesized.
	// Note that file can be nil here: the map need not contain explicit parent directories for all its files.
	// But file can also be non-nil, in case the user wants to set metadata for the directory explicitly.
	// Either way, we need to construct the list of children of this directory.
	var list []mapFileInfo
	var need = make(map[string]bool)
	if realName == "." {
		for fname, f := range fsys {
			i := strings.Index(fname, "/")
			if i < 0 {
				if fname != "." {
					list = append(list, mapFileInfo{fname, f})
				}
			} else {
				need[fname[:i]] = true
			}
		}
	} else {
		prefix := realName + "/"
		for fname, f := range fsys {
			if strings.HasPrefix(fname, prefix) {
				felem := fname[len(prefix):]
				i := strings.Index(felem, "/")
				if i < 0 {
					list = append(list, mapFileInfo{felem, f})
				} else {
					need[fname[len(prefix):len(prefix)+i]] = true
				}
			}
		}
		// If the directory name is not in the map,
		// and there are no children of the name in the map,
		// then the directory is treated as not existing.
		if file == nil && list == nil && len(need) == 0 {
			return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
		}
	}
	for _, fi := range list {
		delete(need, fi.name)
	}
	for name := range need {
		list = append(list, mapFileInfo{name, &MapFile{Mode: fs.ModeDir | 0555}})
	}
	slices.SortFunc(list, func(a, b mapFileInfo) int {
		return strings.Compare(a.name, b.name)
	})

	if file == nil {
		file = &MapFile{Mode: fs.ModeDir | 0555}
	}
	var elem string
	if name == "." {
		elem = "."
	} else {
		elem = name[strings.LastIndex(name, "/")+1:]
	}
	return &mapDir{name, mapFileInfo{elem, file}, list, 0}, nil
}

func (fsys MapFS) resolveSymlinks(name string) (_ string, ok bool) {
	// Fast path: if a symlink is in the map, resolve it.
	if file := fsys[name]; file != nil && file.Mode.Type() == fs.ModeSymlink {
		target := string(file.Data)
		if path.IsAbs(target) {
			return "", false
		}
		return fsys.resolveSymlinks(path.Join(path.Dir(name), target))
	}

	// Check if each parent directory (starting at root) is a symlink.
	for i := 0; i < len(name); {
		j := strings.Index(name[i:], "/")
		var dir string
		if j < 0 {
			dir = name
			i = len(name)
		} else {
			dir = name[:i+j]
			i += j
		}
		if file := fsys[dir]; file != nil && file.Mode.Type() == fs.ModeSymlink {
			target := string(file.Data)
			if path.IsAbs(target) {
				return "", false
			}
			return fsys.resolveSymlinks(path.Join(path.Dir(dir), target) + name[i:])
		}
		i += len("/")
	}
	return name, fs.ValidPath(name)
}

// ReadLink returns the destination of the named symbolic link.
func (fsys MapFS) ReadLink(name string) (string, error) {
	info, err := fsys.lstat(name)
	if err != nil {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: err}
	}
	if info.f.Mode.Type() != fs.ModeSymlink {
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrInvalid}
	}
	return string(info.f.Data), nil
}

// Lstat returns a FileInfo describing the named file.
// If the file is a symbolic link, the returned FileInfo describes the symbolic link.
// Lstat makes no attempt to follow the link.
func (fsys MapFS) Lstat(name string) (fs.FileInfo, error) {
	info, err := fsys.lstat(name)
	if err != nil {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: err}
	}
	return info, nil
}

func (fsys MapFS) lstat(name string) (*mapFileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, fs.ErrNotExist
	}
	realDir, ok := fsys.resolveSymlinks(path.Dir(name))
	if !ok {
		return nil, fs.ErrNotExist
	}
	elem := path.Base(name)
	realName := path.Join(realDir, elem)

	file := fsys[realName]
	if file != nil {
		return &mapFileInfo{elem, file}, nil
	}

	if realName == "." {
		return &mapFileInfo{elem, &MapFile{Mode: fs.ModeDir | 0555}}, nil
	}
	// Maybe a directory.
	prefix := realName + "/"
	for fname := range fsys {
		if strings.HasPrefix(fname, prefix) {
			return &mapFileInfo{elem, &MapFile{Mode: fs.ModeDir | 0555}}, nil
		}
	}
	// If the directory name is not in the map,
	// and there are no children of the name in the map,
	// then the directory is treated as not existing.
	return nil, fs.ErrNotExist
}

// fsOnly is a wrapper that hides all but the fs.FS methods,
// to avoid an infinite recursion when implementing special
// methods in terms of helpers that would use them.
// (In general, implementing these methods using the package fs helpers
// is redundant and unnecessary, but having the methods may make
// MapFS exercise more code paths when used in tests.)
type fsOnly struct{ fs.FS }

func (fsys MapFS) ReadFile(name string) ([]byte, error) {
	return fs.ReadFile(fsOnly{fsys}, name)
}

func (fsys MapFS) Stat(name string) (fs.FileInfo, error) {
	return fs.Stat(fsOnly{fsys}, name)
}

func (fsys MapFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(fsOnly{fsys}, name)
}

func (fsys MapFS) Glob(pattern string) ([]string, error) {
	return fs.Glob(fsOnly{fsys}, pattern)
}

func (fsys MapFS) Mkdir(name string, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return fs.ErrNotExist
	}

	realDir, ok := fsys.resolveSymlinks(path.Dir(name))
	if !ok {
		return fs.ErrNotExist
	} else if _, ok := fsys[realDir]; !ok {
		return fs.ErrNotExist
	}

	elem := path.Base(name)
	realName := path.Join(realDir, elem)
	fsys[realName] = &MapFile{
		Data:    nil,
		Mode:    fs.ModeDir | 0777,
		ModTime: time.Now(),
		Sys:     nil,
	}

	return nil
}

func (fsys MapFS) MkdirAll(name string, perm fs.FileMode) error {
	if !fs.ValidPath(name) {
		return fs.ErrNotExist
	}

	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := fsys.Stat(name)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrNotExist}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.

	// Extract the parent folder from path by first removing any trailing
	// path separator and then scanning backward until finding a path
	// separator or reaching the beginning of the string.
	i := len(name) - 1
	for i >= 0 && name[i] == '/' {
		i--
	}
	for i >= 0 && name[i] != '/' {
		i--
	}
	if i < 0 {
		i = 0
	}

	// If there is a parent directory, and it is not the volume name,
	// recurse to ensure parent directory exists.
	if parent := name[:i]; len(parent) > len(name) {
		err = fsys.MkdirAll(parent, perm)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = fsys.Mkdir(name, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := fsys.Lstat(name)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}
	return nil
}

func (fsys MapFS) Remove(name string) error {
	if !fs.ValidPath(name) {
		return fs.ErrNotExist
	}

	realDir, ok := fsys.resolveSymlinks(name)
	if !ok {
		return fs.ErrNotExist
	}
	info, err := fsys.Stat(realDir)
	if err != nil {
		return err
	} else if info.IsDir() {
		entrys, err := fsys.ReadDir(realDir)
		if err != nil {
			return err
		} else if len(entrys) > 0 {
			return &fs.PathError{Op: "remove", Path: realDir, Err: fs.ErrInvalid}
		}
	}
	delete(fsys, realDir)
	return nil
}

func (fsys MapFS) RemoveAll(name string) error {
	if name == "." || !fs.ValidPath(name) {
		return &fs.PathError{Op: "RemoveAll", Path: name, Err: fs.ErrInvalid}
	}

	err := &fs.PathError{Op: "RemoveAll", Path: name, Err: fs.ErrNotExist}
	for pathName := range fsys {
		if strings.HasPrefix(pathName, name) {
			err = nil
			delete(fsys, name)
		}
	}

	return err
}

func (fsys MapFS) Symlink(oldname, newname string) error { return fsys.Link(oldname, newname) }
func (fsys MapFS) Link(oldname, newname string) error {
	if !fs.ValidPath(oldname) {
		return &fs.PathError{Op: "link", Path: oldname, Err: fs.ErrInvalid}
	} else if !fs.ValidPath(newname) {
		return &fs.PathError{Op: "link", Path: newname, Err: fs.ErrInvalid}
	}

	oldname, ok := fsys.resolveSymlinks(oldname)
	if !ok {
		return &fs.PathError{Op: "link", Path: newname, Err: fs.ErrNotExist}
	}

	if _, ok := fsys[oldname]; !ok {
		return &fs.PathError{Op: "link", Path: oldname, Err: fs.ErrNotExist}
	} else if _, ok := fsys[newname]; !ok {
		return &fs.PathError{Op: "link", Path: newname, Err: fs.ErrInvalid}
	}

	fsys[newname] = &MapFile{
		Data:    []byte(oldname),
		Mode:    fs.ModeSymlink,
		ModTime: fsys[oldname].ModTime,
		Sys:     fsys[oldname].Sys,
	}
	return nil
}

func (fsys MapFS) Chmod(name string, mode fs.FileMode) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chmod", Path: name, Err: fs.ErrInvalid}
	}

	name, ok := fsys.resolveSymlinks(name)
	if !ok || fsys[name] == nil {
		return &fs.PathError{Op: "chmod", Path: name, Err: fs.ErrNotExist}
	}

	fsys[name].Mode = (fsys[name].Mode.Perm() & fsys[name].Mode) | mode.Perm()
	return nil
}

func (fsys MapFS) Chown(name string, uid, gid int) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chown", Path: name, Err: fs.ErrInvalid}
	}
	name, ok := fsys.resolveSymlinks(name)
	if !ok || fsys[name] == nil {
		return &fs.PathError{Op: "chown", Path: name, Err: fs.ErrNotExist}
	}
	if uid >= 0 {
		fsys[name].UID = uid
	}
	if gid >= 0 {
		fsys[name].GID = gid
	}
	return nil
}

func (fsys MapFS) Chtimes(name string, _ time.Time, mtime time.Time) error {
	if !fs.ValidPath(name) {
		return &fs.PathError{Op: "chtime", Path: name, Err: fs.ErrInvalid}
	}
	name, ok := fsys.resolveSymlinks(name)
	if !ok || fsys[name] == nil {
		return &fs.PathError{Op: "chtime", Path: name, Err: fs.ErrNotExist}
	}
	fsys[name].ModTime = mtime
	return nil
}

type noSub struct {
	MapFS
}

func (noSub) Sub() {} // not the fs.SubFS signature

func (fsys MapFS) Sub(dir string) (fs.FS, error) {
	return fs.Sub(noSub{fsys}, dir)
}

// A mapFileInfo implements fs.FileInfo and fs.DirEntry for a given map file.
type mapFileInfo struct {
	name string
	f    *MapFile
}

func (i *mapFileInfo) Name() string               { return path.Base(i.name) }
func (i *mapFileInfo) Size() int64                { return int64(len(i.f.Data)) }
func (i *mapFileInfo) Mode() fs.FileMode          { return i.f.Mode }
func (i *mapFileInfo) Type() fs.FileMode          { return i.f.Mode.Type() }
func (i *mapFileInfo) ModTime() time.Time         { return i.f.ModTime }
func (i *mapFileInfo) IsDir() bool                { return i.f.Mode&fs.ModeDir != 0 }
func (i *mapFileInfo) Sys() any                   { return i.f.Sys }
func (i *mapFileInfo) Info() (fs.FileInfo, error) { return i, nil }

func (i *mapFileInfo) String() string {
	return fs.FormatFileInfo(i)
}

// An openMapFile is a regular (non-directory) fs.File open for reading.
type openMapFile struct {
	path string
	mapFileInfo
	offset int64
}

func (f *openMapFile) Stat() (fs.FileInfo, error) { return &f.mapFileInfo, nil }

func (f *openMapFile) Close() error { return nil }

func (f *openMapFile) Read(b []byte) (int, error) {
	if f.offset >= int64(len(f.f.Data)) {
		return 0, io.EOF
	}
	if f.offset < 0 {
		return 0, &fs.PathError{Op: "read", Path: f.path, Err: fs.ErrInvalid}
	}
	n := copy(b, f.f.Data[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *openMapFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		// offset += 0
	case 1:
		offset += f.offset
	case 2:
		offset += int64(len(f.f.Data))
	}
	if offset < 0 || offset > int64(len(f.f.Data)) {
		return 0, &fs.PathError{Op: "seek", Path: f.path, Err: fs.ErrInvalid}
	}
	f.offset = offset
	return offset, nil
}

func (f *openMapFile) ReadAt(b []byte, offset int64) (int, error) {
	if offset < 0 || offset > int64(len(f.f.Data)) {
		return 0, &fs.PathError{Op: "read", Path: f.path, Err: fs.ErrInvalid}
	}
	n := copy(b, f.f.Data[offset:])
	if n < len(b) {
		return n, io.EOF
	}
	return n, nil
}

func (f *openMapFile) WriteAt(b []byte, offset int64) (int, error) {
	if offset < 0 || offset < int64(len(f.f.Data)) {
		return 0, &fs.PathError{Op: "write", Path: f.path, Err: fs.ErrInvalid}
	}

	at, before := f.f.Data[offset:], f.f.Data[:offset]
	switch {
	case len(before) > len(b):
		before = before[len(b):]
	default:
		before = before[:0]
	}
	f.f.Data = append(at, append(b, before...)...)
	return len(b), nil
}

func (f *openMapFile) Write(b []byte) (int, error) {
	at, before := f.f.Data[f.offset:], f.f.Data[:f.offset]
	switch {
	case len(before) > len(b):
		before = before[len(b):]
	default:
		before = before[:0]
	}
	f.f.Data = append(at, append(b, before...)...)
	f.offset += int64(len(b))
	return len(b), nil
}

func (f *openMapFile) Truncate(size int64) error {
	if int64(len(f.f.Data)) > size {
		f.f.Data = f.f.Data[:size]
		return nil
	}
	f.f.Data = append(f.f.Data, make([]byte, size-int64(len(f.f.Data)))...)
	return nil
}

// A mapDir is a directory fs.File (so also an fs.ReadDirFile) open for reading.
type mapDir struct {
	path string
	mapFileInfo
	entry  []mapFileInfo
	offset int
}

func (d *mapDir) Stat() (fs.FileInfo, error) { return &d.mapFileInfo, nil }
func (d *mapDir) Close() error               { return nil }
func (d *mapDir) Read(b []byte) (int, error) {
	return 0, &fs.PathError{Op: "read", Path: d.path, Err: fs.ErrInvalid}
}

func (mapDir) Write([]byte) (int, error) { return 0, io.EOF }
func (mapDir) Truncate(size int64) error { return io.EOF }

func (d *mapDir) ReadDir(count int) ([]fs.DirEntry, error) {
	n := len(d.entry) - d.offset
	if n == 0 && count > 0 {
		return nil, io.EOF
	}
	if count > 0 && n > count {
		n = count
	}
	list := make([]fs.DirEntry, n)
	for i := range list {
		list[i] = &d.entry[d.offset+i]
	}
	d.offset += n
	return list, nil
}
