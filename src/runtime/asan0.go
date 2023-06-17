// Copyright 2021 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !asan

// Dummy ASan support API, used when not built with -asan.

// Dummy ASan（AddressSanitizer）是一个针对C/C++语言编写程序的内存错误检测工具。它可以帮助开发人员在测试和调试应用程序时发现内存错误，如使用未初始化的变量、缓冲区溢出等问题。Dummy ASan会在编译时将一些特殊的代码注入到应用程序中，以便在程序运行时检测内存访问错误，并在检测到错误时输出相关的信息。
//
//与传统的内存调试工具相比，Dummy ASan具有更低的性能开销和更高的检测精度。它可以检测到一些常见的内存错误类型，如堆栈缓冲区溢出、堆内存使用后释放、指针越界等。同时，在某些情况下，它还可以检测到不可预测的行为，如使用已经被释放的内存。
//
//需要注意的是，Dummy ASan并不能保证完全消除所有内存错误，因此仍然需要开发人员进行充分的测试和调试。

package runtime

import (
	"unsafe"
)

const asanenabled = false

// Because asanenabled is false, none of these functions should be called.

func asanread(addr unsafe.Pointer, sz uintptr)            { throw("asan") }
func asanwrite(addr unsafe.Pointer, sz uintptr)           { throw("asan") }
func asanunpoison(addr unsafe.Pointer, sz uintptr)        { throw("asan") }
func asanpoison(addr unsafe.Pointer, sz uintptr)          { throw("asan") }
func asanregisterglobals(addr unsafe.Pointer, sz uintptr) { throw("asan") }
