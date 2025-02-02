// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package abi

import (
	"std/internal/goarch"
	"unsafe"
)

// RegArgs is a struct that has space for each argument
// and return value register on the current architecture.
//
// Assembly code knows the layout of the first two fields
// of RegArgs.
//
// RegArgs also contains additional space to hold pointers
// when it may not be safe to keep them only in the integer
// register space otherwise.
type RegArgs struct {
	// Values in these slots should be precisely the bit-by-bit
	// representation of how they would appear in a register.
	//
	// This means that on big endian arches, integer values should
	// be in the top bits of the slot. Floats are usually just
	// directly represented, but some architectures treat narrow
	// width floating point values specially (e.g. they're promoted
	// first, or they need to be NaN-boxed).
	Ints   [IntArgRegs]uintptr  // untyped integer registers
	Floats [FloatArgRegs]uint64 // untyped float registers

	// Fields above this point are known to assembly.

	// Ptrs is a space that duplicates Ints but with pointer type,
	// used to make pointers passed or returned  in registers
	// visible to the GC by making the type unsafe.Pointer.
	Ptrs [IntArgRegs]unsafe.Pointer

	// ReturnIsPtr is a bitmap that indicates which registers
	// contain or will contain pointers on the return path from
	// a reflectcall. The i'th bit indicates whether the i'th
	// register contains or will contain a valid Go pointer.
	ReturnIsPtr IntArgRegBitmap
}

func (r *RegArgs) Dump() {
	print("Ints:")
	for _, x := range r.Ints {
		print(" ", x)
	}
	println()
	print("Floats:")
	for _, x := range r.Floats {
		print(" ", x)
	}
	println()
	print("Ptrs:")
	for _, x := range r.Ptrs {
		print(" ", x)
	}
	println()
}

// IntRegArgAddr returns a pointer inside of r.Ints[reg] that is appropriately
// offset for an argument of size argSize.
//
// argSize must be non-zero, fit in a register, and a power-of-two.
//
// This method is a helper for dealing with the endianness of different CPU
// architectures, since sub-word-sized arguments in big endian architectures
// need to be "aligned" to the upper edge of the register to be interpreted
// by the CPU correctly.
func (r *RegArgs) IntRegArgAddr(reg int, argSize uintptr) unsafe.Pointer {
	if argSize > goarch.PtrSize || argSize == 0 || argSize&(argSize-1) != 0 {
		panic("invalid argSize")
	}
	offset := uintptr(0)
	if goarch.BigEndian {
		offset = goarch.PtrSize - argSize
	}
	return unsafe.Pointer(uintptr(unsafe.Pointer(&r.Ints[reg])) + offset)
}

// IntArgRegBitmap is a bitmap large enough to hold one bit per
// integer argument/return register.
type IntArgRegBitmap [(IntArgRegs + 7) / 8]uint8

// Set sets the i'th bit of the bitmap to 1.
func (b *IntArgRegBitmap) Set(i int) {
	b[i/8] |= uint8(1) << (i % 8)
}

// Get returns whether the i'th bit of the bitmap is set.
//
// nosplit because it's called in extremely sensitive contexts, like
// on the reflectcall return path.
//
//go:nosplit
func (b *IntArgRegBitmap) Get(i int) bool {
	return b[i/8]&(uint8(1)<<(i%8)) != 0
}

// FuncPC* intrinsics.
//
// CAREFUL: In programs with plugins, FuncPC* can return different values
// for the same function (because there are actually multiple copies of
// the same function in the address space). To be safe, don't use the
// results of this function in any == expression. It is only safe to
// use the result as an address at which to start executing code.

// FuncPCABI0 returns the entry PC of the function f, which must be a
// direct reference of a function defined as ABI0. Otherwise it is a
// compile-time error.
//
// Implemented as a compile intrinsic.
// FuncPCABI0 应该是指函数的 ABI（Application Binary Interface）版本 0。
// note 在计算机科学中，ABI定义了如何在二进制级别上与编译器、操作系统和硬件交互。
//
// 在函数调用期间，编译器必须遵循特定的ABI规范，以确保正确地传递参数、返回值以及处理内存等问题。
// 不同的ABI可能会使用不同的寄存器、堆栈布局和对齐方式来实现这些目标。
//
// 因此，FuncPCABI0 可能代表与某个函数相关的ABI版本号或ABI名称，其具体含义取决于上下文。

// FuncPCABI0 返回函数 f 的入口 PC，它必须是定义为 ABI0 的函数的直接引用。否则就是编译时错误。作为编译内在函数实现
func FuncPCABI0(f any) uintptr

// FuncPCABIInternal returns the entry PC of the function f. If f is a
// direct reference of a function, it must be defined as ABIInternal.
// Otherwise it is a compile-time error. If f is not a direct reference
// of a defined function, it assumes that f is a func value. Otherwise
// the behavior is undefined.
//
// Implemented as a compile intrinsic.
func FuncPCABIInternal(f any) uintptr
