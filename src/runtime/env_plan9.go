// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// 在Go语言中，plan9相关的文件主要是为了支持分布式操作系统Plan 9的一些特性。
//
//Plan 9是由贝尔实验室开发的分布式操作系统，其旨在提供一种基于网络的计算环境，使得用户可以远程协作、共享资源并进行分布式处理。Go语言最初也是由贝尔实验室开发的，因此在设计时就考虑了对Plan 9的支持。
//
//在Go语言中，Plan 9相关的文件主要包括：
//
//plan9.h：定义了Plan 9的系统调用接口；
//syscall_plan9.go：实现了Plan 9的系统调用接口；
//net_plan9.go：实现了Plan 9下的网络套接字接口；
//os_plan9.go：实现了Plan 9下的文件和进程管理接口。
//通过这些文件，Go语言可以直接调用Plan 9的系统调用接口和其他相关功能，从而支持分布式计算和网络编程等应用。

package runtime

import "unsafe"

const (
	// Plan 9 environment device
	envDir = "/env/"
	// size of buffer to read from a directory
	dirBufSize = 4096
	// size of buffer to read an environment variable (may grow)
	envBufSize = 128
	// offset of the name field in a 9P directory entry - see syscall.UnmarshalDir()
	nameOffset = 39
)

// Goenvs caches the Plan 9 environment variables at start of execution into
// string array envs, to supply the initial contents for os.Environ.
// Subsequent calls to os.Setenv will change this cache, without writing back
// to the (possibly shared) Plan 9 environment, so that Setenv and Getenv
// conform to the same Posix semantics as on other operating systems.
// For Plan 9 shared environment semantics, instead of Getenv(key) and
// Setenv(key, value), one can use os.ReadFile("/env/" + key) and
// os.WriteFile("/env/" + key, value, 0666) respectively.
//
//go:nosplit
func goenvs() {
	buf := make([]byte, envBufSize)
	copy(buf, envDir)
	dirfd := open(&buf[0], _OREAD, 0)
	if dirfd < 0 {
		return
	}
	defer closefd(dirfd)
	dofiles(dirfd, func(name []byte) {
		name = append(name, 0)
		buf = buf[:len(envDir)]
		copy(buf, envDir)
		buf = append(buf, name...)
		fd := open(&buf[0], _OREAD, 0)
		if fd < 0 {
			return
		}
		defer closefd(fd)
		n := len(buf)
		r := 0
		for {
			r = int(pread(fd, unsafe.Pointer(&buf[0]), int32(n), 0))
			if r < n {
				break
			}
			n = int(seek(fd, 0, 2)) + 1
			if len(buf) < n {
				buf = make([]byte, n)
			}
		}
		if r <= 0 {
			r = 0
		} else if buf[r-1] == 0 {
			r--
		}
		name[len(name)-1] = '='
		env := make([]byte, len(name)+r)
		copy(env, name)
		copy(env[len(name):], buf[:r])
		envs = append(envs, string(env))
	})
}

// Dofiles reads the directory opened with file descriptor fd, applying function f
// to each filename in it.
//
//go:nosplit
func dofiles(dirfd int32, f func([]byte)) {
	dirbuf := new([dirBufSize]byte)

	var off int64 = 0
	for {
		n := pread(dirfd, unsafe.Pointer(&dirbuf[0]), int32(dirBufSize), off)
		if n <= 0 {
			return
		}
		for b := dirbuf[:n]; len(b) > 0; {
			var name []byte
			name, b = gdirname(b)
			if name == nil {
				return
			}
			f(name)
		}
		off += int64(n)
	}
}

// Gdirname returns the first filename from a buffer of directory entries,
// and a slice containing the remaining directory entries.
// If the buffer doesn't start with a valid directory entry, the returned name is nil.
//
//go:nosplit
func gdirname(buf []byte) (name []byte, rest []byte) {
	if 2+nameOffset+2 > len(buf) {
		return
	}
	entryLen, buf := gbit16(buf)
	if entryLen > len(buf) {
		return
	}
	n, b := gbit16(buf[nameOffset:])
	if n > len(b) {
		return
	}
	name = b[:n]
	rest = buf[entryLen:]
	return
}

// Gbit16 reads a 16-bit little-endian binary number from b and returns it
// with the remaining slice of b.
//
//go:nosplit
func gbit16(b []byte) (int, []byte) {
	return int(b[0]) | int(b[1])<<8, b[2:]
}
