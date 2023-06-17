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

// race.Errors() 方法的作用是检测当前程序是否存在数据竞争，并返回当前程序中存在的数据竞争数量。
// 当在编译 Go 程序时使用 -race 参数时，编译器会自动生成带有数据竞争检测功能的二进制文件。
// 此时，如果程序中存在数据竞争，运行时就会检测到并打印相关信息。race.Errors() 方法可以用于在程序运行期间主动检测数据竞争情况
// 如果有数据竞争，返回值大于 0；否则，返回值等于 0。
//
// 需要注意的是，在使用 race 包进行数据竞争检测时，需要满足以下两个条件：
//
// Go 程序必须以 -race 参数来进行编译，才能启用数据竞争检测功能；
// 在程序中使用了并发编程的模式，才会存在数据竞争的可能性。如果程序只包含顺序执行的代码，则不存在数据竞争问题。
func Errors() int { return 0 }
