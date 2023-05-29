// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !race
// +build !race

package race

import (
	"unsafe"
)

const Enabled = false

func Acquire(addr unsafe.Pointer) {
}

func Release(addr unsafe.Pointer) {
}

// ReleaseMerge race.ReleaseMerge(x) 是一个用于在使用 race 检查的环境下释放关联到指定对象的全部竞争检查资源的函数。
// 它有一个参数，即要释放的对象的地址，返回值为无。
//
// race.ReleaseMerge 函数的作用是告诉 race 工具，与该对象相关联的竞争检查已经结束，可以将与之相关的竞争检查资源释放掉了。
// 这里需要注意的是，race.ReleaseMerge 函数并不是真正地释放资源，它只是向 race 工具发送一个信号，告诉它可以将关联到该对象的竞争检查资源合并到其他资源中。
// 实际上，race 工具会在程序的运行期间跟踪所有线程对内存的读写操作，并通过这些信息来检测程序中的数据竞争问题。
// 在 race 工具发现某些内存区域存在竞争问题时，会记录下来这些区域的相关信息，以便在后续的分析和报告中使用。
// 而 release 和 merge 操作则是 race 工具用来优化数据结构和提高性能的一种手段
func ReleaseMerge(addr unsafe.Pointer) {
}

func Disable() {
}

func Enable() {
}

func Read(addr unsafe.Pointer) {
}

func Write(addr unsafe.Pointer) {
}

func ReadRange(addr unsafe.Pointer, len int) {
}

func WriteRange(addr unsafe.Pointer, len int) {
}

func Errors() int { return 0 }
