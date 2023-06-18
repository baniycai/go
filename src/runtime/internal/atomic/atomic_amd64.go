// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package atomic

import "unsafe"

// Export some functions via linkname to assembly in sync/atomic.
//
//go:linkname Load
//go:linkname Loadp
//go:linkname Load64

// 使用go:nosplit和go:noinline指令可以优化一些特定场景下的代码性能。
//
//对于go:nosplit(不要自己新开一个栈)指令，当需要非常快速的函数执行时，避免在栈上分配内存和进行额外的函数调用开销可以提高程序运行速度。
//例如，在编写嵌入式系统或者需要实现高性能网络协议栈时，使用该指令可以帮助开发人员满足性能需求。
//
//对于go:noinline指令，通常在需要动态调用的函数或递归函数中使用。例如，在构建泛型库时，由于无法预先知道具体调用的类型，因此需要动态生成代码。
//而使用该指令可以确保这个过程的正确性，并且不会因为内联函数导致错误的代码生成。此外，在编写基准测试或者进行性能调优时，使用该指令还可以更精确地测量函数的性能。
// 编译器应该会自己判断是否需要内联，但为了防止它判断错误，有时候需要加上指令来强制阻止它内联
//
//需要注意的是，使用这两个指令都需要谨慎，因为它们可能会导致未定义的行为，尤其是在使用go:nosplit时。只有在确保代码正确性的前提下，才应该考虑使用这两个指令来进行性能优化。

// note 是的，go:nosplit指令和go:inline指令都可以让被调用函数共用调用者的栈帧。
//
//在使用go:nosplit指令时，编译器会将被调用函数的参数和局部变量等数据存储在调用者的栈帧中，而不会为被调用函数单独分配一个新的栈帧。
//这样做的好处是 note 避免了栈帧分配和销毁的开销，从而提高了程序的执行效率。但是，这也需要开发人员自己来确保程序正确性，避免产生未定义行为和安全问题。
//
//在使用go:inline指令时，编译器会将被调用函数的代码直接嵌入到调用它的代码中，从而避免了函数调用的开销。
//note 这样做的好处是使程序更加高效，但也可能会导致代码膨胀、可读性降低以及增加代码维护的难度。
//
//因此，虽然go:nosplit指令和go:inline指令都可以让被调用函数共用调用者的栈帧，但是它们的实现方式和使用场景是不同的。需要根据具体的应用场景和优化需求选择合适的指令。

//go:nosplit
//go:noinline
func Load(ptr *uint32) uint32 {
	return *ptr
}

//go:nosplit
//go:noinline
func Loadp(ptr unsafe.Pointer) unsafe.Pointer {
	return *(*unsafe.Pointer)(ptr)
}

//go:nosplit
//go:noinline
func Load64(ptr *uint64) uint64 {
	return *ptr
}

//go:nosplit
//go:noinline
func LoadAcq(ptr *uint32) uint32 {
	return *ptr
}

//go:nosplit
//go:noinline
func LoadAcq64(ptr *uint64) uint64 {
	return *ptr
}

//go:nosplit
//go:noinline
func LoadAcquintptr(ptr *uintptr) uintptr {
	return *ptr
}

//go:noescape
func Xadd(ptr *uint32, delta int32) uint32

//go:noescape
func Xadd64(ptr *uint64, delta int64) uint64

//go:noescape
func Xadduintptr(ptr *uintptr, delta uintptr) uintptr

//go:noescape
func Xchg(ptr *uint32, new uint32) uint32

//go:noescape
func Xchg64(ptr *uint64, new uint64) uint64

//go:noescape
func Xchguintptr(ptr *uintptr, new uintptr) uintptr

//go:nosplit
//go:noinline
func Load8(ptr *uint8) uint8 {
	return *ptr
}

//go:noescape
func And8(ptr *uint8, val uint8)

//go:noescape
func Or8(ptr *uint8, val uint8)

//go:noescape
func And(ptr *uint32, val uint32)

//go:noescape
func Or(ptr *uint32, val uint32)

// NOTE: Do not add atomicxor8 (XOR is not idempotent).

//go:noescape
func Cas64(ptr *uint64, old, new uint64) bool

//go:noescape
func CasRel(ptr *uint32, old, new uint32) bool

//go:noescape
func Store(ptr *uint32, val uint32)

//go:noescape
func Store8(ptr *uint8, val uint8)

//go:noescape
func Store64(ptr *uint64, val uint64)

//go:noescape
func StoreRel(ptr *uint32, val uint32)

//go:noescape
func StoreRel64(ptr *uint64, val uint64)

//go:noescape
func StoreReluintptr(ptr *uintptr, val uintptr)

// StorepNoWB performs *ptr = val atomically and without a write
// barrier.
//
// NO go:noescape annotation; see atomic_pointer.go.
func StorepNoWB(ptr unsafe.Pointer, val unsafe.Pointer)
