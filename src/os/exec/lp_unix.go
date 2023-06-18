// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package exec

import (
	"errors"
	"internal/godebug"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is the error resulting if a path search failed to find an executable file.
var ErrNotFound = errors.New("executable file not found in $PATH")

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
		return nil
	}
	return fs.ErrPermission
}

// LookPath searches for an executable named file in the
// directories named by the PATH environment variable.
// If file contains a slash, it is tried directly and the PATH is not consulted.
// Otherwise, on success, the result is an absolute path.
//
// In older versions of Go, LookPath could return a path relative to the current directory.
// As of Go 1.19, LookPath will instead return that path along with an error satisfying
// errors.Is(err, ErrDot). See the package documentation for more details.
// note LookPath在由PATH环境变量命名的目录中搜索名为file的可执行文件。如果file包含斜杠，则直接尝试搜索，不会查询PATH。否则，成功后的结果是绝对路径。
// 在较旧版本的Go中，LookPath可能返回相对于当前目录的路径。从Go 1.19开始，LookPath将返回该路径和满足errors.Is(err,ErrDot)错误的错误。
func LookPath(file string) (string, error) {
	// NOTE(rsc): I wish we could use the Plan 9 behavior here
	// (only bypass the path if file begins with / or ./ or ../)
	// but that would not match all the Unix shells.

	if strings.Contains(file, "/") { // note file包含/，直接看当前系统是否存在该可执行文件
		err := findExecutable(file)
		if err == nil {
			return file, nil
		}
		return "", &Error{file, err}
	}
	// macOS格式：/Users/chb/.docker/bin:/Users/chb/.orbstack/bin:/opt/homebrew/bin:/opt/homebrew/sbin:
	path := os.Getenv("PATH") // note file不含/，则查看PATH环境变量，看是否存在dir+file的可执行文件
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		path := filepath.Join(dir, file)
		if err := findExecutable(path); err == nil {
			if !filepath.IsAbs(path) && godebug.Get("execerrdot") != "0" {
				return path, &Error{file, ErrDot}
			}
			return path, nil
		}
	}
	return "", &Error{file, ErrNotFound}
}
