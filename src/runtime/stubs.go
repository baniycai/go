// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"internal/abi"
	"internal/goarch"
	"runtime/internal/math"
	"unsafe"
)

// Should be a built-in for unsafe.Pointer?
//
//go:nosplit
func add(p unsafe.Pointer, x uintptr) unsafe.Pointer { // note 地址值相加
	return unsafe.Pointer(uintptr(p) + x)
}

// getg returns the pointer to the current g.
// The compiler rewrites calls to this function into instructions
// that fetch the g directly (from TLS or from the dedicated register).
// note 在 Go 的运行时库中是一个内置函数，其具体实现是由 Go 语言编译器提供，并且无法像其他普通函数一样进行查看。
// 在代码中，我们只能调用这个函数，而不能查看其具体实现。
// 需要注意的是，func getg() *g 函数的作用是返回当前 Goroutine 的 g 结构体指针，包含了当前 Goroutine 的状态和上下文信息
// 简单理解，就是在编译的时候由编译器提供这个函数的具体实现

// 在 Go 语言中，func getg() *g 函数之所以不是直接实现的，而是通过编译器和运行时库提供实现，主要有以下几个原因：
// func getg() *g 函数的实现涉及到很多底层细节，包括 Goroutine 的调度、栈管理、内存分配等等。这些细节需要由编译器和运行时库共同来实现，才能确保其正确性和性能。
// 在 Go 语言中，Goroutine 是一种轻量级线程，可以并发执行。为了实现 Goroutine 的高效调度和资源管理，Go 语言采用了一种特殊的 Goroutine 调度器。这个调度器需要和编译器、运行时库紧密协作，才能达到最佳的效果。因此，将 func getg() *g 函数的实现交给编译器和运行时库处理，也可以更好地与 Goroutine 调度器进行集成。
// Go 语言的一个重要特点是垃圾回收机制，而 func getg() *g 函数也和垃圾回收机制密切相关。由于垃圾回收机制需要对所有内存进行扫描和标记，因此需要访问所有 Goroutine 中的栈。func getg() *g 函数的实现需要将当前 Goroutine 的 g 结构体指针保存在栈帧中，以便垃圾回收器能够正确地访问该 Goroutine 的栈。这种处理需要编译器和运行时库共同协作，才能实现准确和高效的垃圾回收。
// 因此，为了保证 func getg() *g 函数的正确性和性能，并且与 Goroutine 调度器和垃圾回收机制紧密集成，Go 语言选择通过编译器和运行时库来提供其具体实现。
func getg() *g

// mcall switches from the g to the g0 stack and invokes fn(g),
// where g is the goroutine that made the call.
// mcall saves g's current PC/SP in g->sched so that it can be restored later.
// It is up to fn to arrange for that later execution, typically by recording
// g in a data structure, causing something to call ready(g) later.
// mcall returns to the original goroutine g later, when g has been rescheduled.
// fn must not return at all; typically it ends by calling schedule, to let the m
// run other goroutines.
//
// mcall can only be called from g stacks (not g0, not gsignal).
//
// This must NOT be go:noescape: if fn is a stack-allocated closure,
// fn puts g on a run queue, and g executes before fn returns, the
// closure will be invalidated while it is still executing.
func mcall(fn func(*g))

// systemstack 在系统栈上运行 fn 函数。
// 如果 systemstack 是从操作系统线程 (g0) 栈或信号处理 (gsignal) 栈中调用的，
// systemstack 将直接调用 fn 并返回。
// 否则，systemstack 正在从普通 goroutine 的有限栈中调用。
// 在这种情况下，systemstack 切换到操作系统线程栈，调用 fn，并切换回来。
// 通常使用函数字面量作为参数，以便与调用 system stack 的代码共享输入和输出：
//
//   ... 设置 y ...
//   systemstack(func() {
//     x = bigcall(y)
//   })
//   ... 使用 x ...

// systemstack runs fn on a system stack.
// If systemstack is called from the per-OS-thread (g0) stack, or
// if systemstack is called from the signal handling (gsignal) stack,
// systemstack calls fn directly and returns.
// Otherwise, systemstack is being called from the limited stack
// of an ordinary goroutine. In this case, systemstack switches
// to the per-OS-thread stack, calls fn, and switches back.
// It is common to use a func literal as the argument, in order
// to share inputs and outputs with the code around the call
// to system stack:
//
//	... set up y ...
//	systemstack(func() {
//		x = bigcall(y)
//	})
//	... use x ...
//
//go:noescape
func systemstack(fn func())

var badsystemstackMsg = "fatal: systemstack called from unexpected goroutine"

//go:nosplit
//go:nowritebarrierrec
func badsystemstack() {
	sp := stringStructOf(&badsystemstackMsg)
	write(2, sp.str, int32(sp.len))
}

// memclrNoHeapPointers clears n bytes starting at ptr.
//
// Usually you should use typedmemclr. memclrNoHeapPointers should be
// used only when the caller knows that *ptr contains no heap pointers
// because either:
//
// *ptr is initialized memory and its type is pointer-free, or
//
// *ptr is uninitialized memory (e.g., memory that's being reused
// for a new allocation) and hence contains only "junk".
//
// memclrNoHeapPointers ensures that if ptr is pointer-aligned, and n
// is a multiple of the pointer size, then any pointer-aligned,
// pointer-sized portion is cleared atomically. Despite the function
// name, this is necessary because this function is the underlying
// implementation of typedmemclr and memclrHasPointers. See the doc of
// memmove for more details.
//
// The (CPU-specific) implementations of this function are in memclr_*.s.
//
//go:noescape
func memclrNoHeapPointers(ptr unsafe.Pointer, n uintptr)

//go:linkname reflect_memclrNoHeapPointers reflect.memclrNoHeapPointers
func reflect_memclrNoHeapPointers(ptr unsafe.Pointer, n uintptr) {
	memclrNoHeapPointers(ptr, n)
}

// memmove 确保“from”中的任何指针都以不可分割的方式写入“to”，这样活泼的读取就无法观察到写了一半的指针。
// 这是防止垃圾收集器观察无效指针所必需的，并且不同于非托管语言中的 memmove
// 但是，如果“from”和“to”可能包含指针，则 memmove 只需要这样做，如果“from”、“to”和“n”都是字对齐的，这只能是这种情况.
//

// TODO 这里有点歧义，个人认为，禁止逃逸分析就是完全分配到栈上了!但gpt说不是

// 使用指令 //go:noescape 可以让编译器不对某个函数进行逃逸分析。这样就可以 note 尽可能(注意关键词)地减少逃逸，从而减小内存分配和垃圾回收的负担，提高程序的性能

// 如果你在使用 go:noescape 指令禁止逃逸分析的同时，返回了一个指向函数内部局部变量的指针，则这个指针对应的对象会被分配在栈上而不是堆上。
//需要注意的是，如果这个指针所指向的对象在函数调用结束之后仍然需要被使用，那么这个对象就不能再分配在栈上了，而需要分配在堆上。
//如果编译器进行了逃逸分析，并认为这个对象需要在堆上分配，那么即使使用了 go:noescape 指令，这个对象也会在堆上分配。
//因此，在使用 go:noescape 指令时，需要确保返回值不包含指向函数作用域之外的变量的指针，否则可能会导致内存访问错误或者未定义行为等问题。

// 使用 go:noescape 指令的主要目的是为了优化函数性能，通过减少逃逸的发生来降低内存分配和垃圾回收的负担。
//虽然使用 go:noescape 指令并不保证所有对象都被分配在栈上，但是当编译器无法进行逃逸分析时，
//通过使用该指令可以确保 note 尽可能多地将对象分配在栈上而非堆上。在某些情况下，这可能会对程序性能产生积极的影响。
//此外，使用 go:noescape 指令也有助于加强代码的安全性，因为它显式地告诉了编译器该函数中的指针不应该逃逸到函数作用域之外。
//这样可以避免在函数调用结束后访问已经失效的指针，从而提高了代码的可靠性。
//总之，尽管使用 go:noescape 指令并不能完全消除逃逸的发生，但它仍然可以提供一定的优化和安全性保障，具体是否使用需要根据实际情况进行权衡。

// 以下是一些使用 go:noescape 指令的例子：
//字符串切片操作：如果函数需要对一个字符串进行切片操作并返回切片，通常情况下编译器会将切片分配在堆上，因为切片指向了原始字符串的内存空间。但实际上，这个切片只有在函数调用过程中才会被使用，所以可以使用 go:noescape 指令来禁止逃逸分析，让切片分配在栈上，从而提高程序性能。
//链表操作：某些链表操作可能需要返回链表节点的指针，但这个指针通常只在函数调用过程中被使用，不需要在堆上分配。可以使用 go:noescape 指令来禁止逃逸分析，让链表节点的指针分配在栈上，从而提高程序性能。
//排序算法：排序算法通常需要创建大量的临时对象，这些对象容易导致逃逸。可以使用 go:noescape 指令来禁止逃逸分析，让这些对象分配在栈上，从而减少内存分配和垃圾回收的负担，提高程序性能。

// memmove copies n bytes from "from" to "to".
//
// memmove ensures that any pointer in "from" is written to "to" with
// an indivisible write, so that racy reads cannot observe a
// half-written pointer. This is necessary to prevent the garbage
// collector from observing invalid pointers, and differs from memmove
// in unmanaged languages. However, memmove is only required to do
// this if "from" and "to" may contain pointers, which can only be the
// case if "from", "to", and "n" are all be word-aligned.
//
// Implementations are in memmove_*.s.
//
//go:noescape
func memmove(to, from unsafe.Pointer, n uintptr) // 汇编实现

// Outside assembly calls memmove. Make sure it has ABI wrappers.
//
//go:linkname memmove

//go:linkname reflect_memmove reflect.memmove
func reflect_memmove(to, from unsafe.Pointer, n uintptr) {
	memmove(to, from, n)
}

// exported value for testing
const hashLoad = float32(loadFactorNum) / float32(loadFactorDen)

//go:nosplit
func fastrand() uint32 {
	mp := getg().m
	// Implement wyrand: https://github.com/wangyi-fudan/wyhash
	// Only the platform that math.Mul64 can be lowered
	// by the compiler should be in this list.
	if goarch.IsAmd64|goarch.IsArm64|goarch.IsPpc64|
		goarch.IsPpc64le|goarch.IsMips64|goarch.IsMips64le|
		goarch.IsS390x|goarch.IsRiscv64 == 1 {
		mp.fastrand += 0xa0761d6478bd642f
		hi, lo := math.Mul64(mp.fastrand, mp.fastrand^0xe7037ed1a0b428db)
		return uint32(hi ^ lo)
	}

	// Implement xorshift64+: 2 32-bit xorshift sequences added together.
	// Shift triplet [17,7,16] was calculated as indicated in Marsaglia's
	// Xorshift paper: https://www.jstatsoft.org/article/view/v008i14/xorshift.pdf
	// This generator passes the SmallCrush suite, part of TestU01 framework:
	// http://simul.iro.umontreal.ca/testu01/tu01.html
	t := (*[2]uint32)(unsafe.Pointer(&mp.fastrand))
	s1, s0 := t[0], t[1]
	s1 ^= s1 << 17
	s1 = s1 ^ s0 ^ s1>>7 ^ s0>>16
	t[0], t[1] = s0, s1
	return s0 + s1
}

//go:nosplit
func fastrandn(n uint32) uint32 {
	// This is similar to fastrand() % n, but faster.
	// See https://lemire.me/blog/2016/06/27/a-fast-alternative-to-the-modulo-reduction/
	return uint32(uint64(fastrand()) * uint64(n) >> 32)
}

func fastrand64() uint64 {
	mp := getg().m
	// Implement wyrand: https://github.com/wangyi-fudan/wyhash
	// Only the platform that math.Mul64 can be lowered
	// by the compiler should be in this list.
	if goarch.IsAmd64|goarch.IsArm64|goarch.IsPpc64|
		goarch.IsPpc64le|goarch.IsMips64|goarch.IsMips64le|
		goarch.IsS390x|goarch.IsRiscv64 == 1 {
		mp.fastrand += 0xa0761d6478bd642f
		hi, lo := math.Mul64(mp.fastrand, mp.fastrand^0xe7037ed1a0b428db)
		return hi ^ lo
	}

	// Implement xorshift64+: 2 32-bit xorshift sequences added together.
	// Xorshift paper: https://www.jstatsoft.org/article/view/v008i14/xorshift.pdf
	// This generator passes the SmallCrush suite, part of TestU01 framework:
	// http://simul.iro.umontreal.ca/testu01/tu01.html
	t := (*[2]uint32)(unsafe.Pointer(&mp.fastrand))
	s1, s0 := t[0], t[1]
	s1 ^= s1 << 17
	s1 = s1 ^ s0 ^ s1>>7 ^ s0>>16
	r := uint64(s0 + s1)

	s0, s1 = s1, s0
	s1 ^= s1 << 17
	s1 = s1 ^ s0 ^ s1>>7 ^ s0>>16
	r += uint64(s0+s1) << 32

	t[0], t[1] = s0, s1
	return r
}

func fastrandu() uint {
	if goarch.PtrSize == 4 {
		return uint(fastrand())
	}
	return uint(fastrand64())
}

//go:linkname sync_fastrandn sync.fastrandn
func sync_fastrandn(n uint32) uint32 { return fastrandn(n) }

//go:linkname net_fastrandu net.fastrandu
func net_fastrandu() uint { return fastrandu() }

//go:linkname os_fastrand os.fastrand
func os_fastrand() uint32 { return fastrand() }

// in internal/bytealg/equal_*.s
//
//go:noescape
func memequal(a, b unsafe.Pointer, size uintptr) bool

// noescape hides a pointer from escape analysis.  noescape is
// the identity function but escape analysis doesn't think the
// output depends on the input.  noescape is inlined and currently
// compiles down to zero instructions.
// USE CAREFULLY!
//
//go:nosplit
func noescape(p unsafe.Pointer) unsafe.Pointer {
	x := uintptr(p)
	return unsafe.Pointer(x ^ 0)
}

// Not all cgocallback frames are actually cgocallback,
// so not all have these arguments. Mark them uintptr so that the GC
// does not misinterpret memory when the arguments are not present.
// cgocallback is not called from Go, only from crosscall2.
// This in turn calls cgocallbackg, which is where we'll find
// pointer-declared arguments.
func cgocallback(fn, frame, ctxt uintptr)

func gogo(buf *gobuf)

func asminit()
func setg(gg *g)
func breakpoint()

// reflectcall calls fn with arguments described by stackArgs, stackArgsSize,
// frameSize, and regArgs.
//
// Arguments passed on the stack and space for return values passed on the stack
// must be laid out at the space pointed to by stackArgs (with total length
// stackArgsSize) according to the ABI.
//
// stackRetOffset must be some value <= stackArgsSize that indicates the
// offset within stackArgs where the return value space begins.
//
// frameSize is the total size of the argument frame at stackArgs and must
// therefore be >= stackArgsSize. It must include additional space for spilling
// register arguments for stack growth and preemption.
//
// TODO(mknyszek): Once we don't need the additional spill space, remove frameSize,
// since frameSize will be redundant with stackArgsSize.
//
// Arguments passed in registers must be laid out in regArgs according to the ABI.
// regArgs will hold any return values passed in registers after the call.
//
// reflectcall copies stack arguments from stackArgs to the goroutine stack, and
// then copies back stackArgsSize-stackRetOffset bytes back to the return space
// in stackArgs once fn has completed. It also "unspills" argument registers from
// regArgs before calling fn, and spills them back into regArgs immediately
// following the call to fn. If there are results being returned on the stack,
// the caller should pass the argument frame type as stackArgsType so that
// reflectcall can execute appropriate write barriers during the copy.
//
// reflectcall expects regArgs.ReturnIsPtr to be populated indicating which
// registers on the return path will contain Go pointers. It will then store
// these pointers in regArgs.Ptrs such that they are visible to the GC.
//
// Package reflect passes a frame type. In package runtime, there is only
// one call that copies results back, in callbackWrap in syscall_windows.go, and it
// does NOT pass a frame type, meaning there are no write barriers invoked. See that
// call site for justification.
//
// Package reflect accesses this symbol through a linkname.
//
// Arguments passed through to reflectcall do not escape. The type is used
// only in a very limited callee of reflectcall, the stackArgs are copied, and
// regArgs is only used in the reflectcall frame.
//
//go:noescape
func reflectcall(stackArgsType *_type, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)

func procyield(cycles uint32)

type neverCallThisFunction struct{}

// goexit is the return stub at the top of every goroutine call stack.
// Each goroutine stack is constructed as if goexit called the
// goroutine's entry point function, so that when the entry point
// function returns, it will return to goexit, which will call goexit1
// to perform the actual exit.
//
// This function must never be called directly. Call goexit1 instead.
// gentraceback assumes that goexit terminates the stack. A direct
// call on the stack will cause gentraceback to stop walking the stack
// prematurely and if there is leftover state it may panic.
func goexit(neverCallThisFunction)

// publicationBarrier performs a store/store barrier (a "publication"
// or "export" barrier). Some form of synchronization is required
// between initializing an object and making that object accessible to
// another processor. Without synchronization, the initialization
// writes and the "publication" write may be reordered, allowing the
// other processor to follow the pointer and observe an uninitialized
// object. In general, higher-level synchronization should be used,
// such as locking or an atomic pointer write. publicationBarrier is
// for when those aren't an option, such as in the implementation of
// the memory manager.
//
// There's no corresponding barrier for the read side because the read
// side naturally has a data dependency order. All architectures that
// Go supports or seems likely to ever support automatically enforce
// data dependency ordering.
func publicationBarrier()

// getcallerpc returns the program counter (PC) of its caller's caller.
// getcallersp returns the stack pointer (SP) of its caller's caller.
// The implementation may be a compiler intrinsic; there is not
// necessarily code implementing this on every platform.
//
// For example:
//
//	func f(arg1, arg2, arg3 int) {
//		pc := getcallerpc()
//		sp := getcallersp()
//	}
//
// These two lines find the PC and SP immediately following
// the call to f (where f will return).
//
// The call to getcallerpc and getcallersp must be done in the
// frame being asked about.
//
// The result of getcallersp is correct at the time of the return,
// but it may be invalidated by any subsequent call to a function
// that might relocate the stack in order to grow or shrink it.
// A general rule is that the result of getcallersp should be used
// immediately and can only be passed to nosplit functions.

// 在某些平台上，getcallerpc() 和 getcallersp() 可能被编译器内部实现为内置函数。因此，不是所有平台都会有相应的代码实现，这里应该是没有具体实现的
// getcallerpc() 函数返回其调用者的调用者的程序计数器（PC）。换句话说，它能够获取当前函数的调用者的返回地址。
// getcallersp() 函数返回其调用者的调用者的栈指针（SP）。换句话说，它能够获取当前函数的调用者的栈指针位置。

//go:noescape
func getcallerpc() uintptr

//go:noescape
func getcallersp() uintptr // implemented as an intrinsic on all platforms

// getclosureptr returns the pointer to the current closure.
// getclosureptr can only be used in an assignment statement
// at the entry of a function. Moreover, go:nosplit directive
// must be specified at the declaration of caller function,
// so that the function prolog does not clobber the closure register.
// for example:
//
//	//go:nosplit
//	func f(arg1, arg2, arg3 int) {
//		dx := getclosureptr()
//	}
//
// The compiler rewrites calls to this function into instructions that fetch the
// pointer from a well-known register (DX on x86 architecture, etc.) directly.
func getclosureptr() uintptr

// 应该是在asm_arm64.s中映射到
//
//go:noescape
func asmcgocall(fn, arg unsafe.Pointer) int32

func morestack()
func morestack_noctxt()
func rt0_go()

// return0 is a stub used to return 0 from deferproc.
// It is called at the very end of deferproc to signal
// the calling Go function that it should not jump
// to deferreturn.
// in asm_*.s
func return0()

// in asm_*.s
// not called directly; definitions here supply type information for traceback.
// These must have the same signature (arg pointer map) as reflectcall.
func call16(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call32(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call64(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call128(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call256(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call512(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call1024(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call2048(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call4096(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call8192(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call16384(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call32768(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call65536(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call131072(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call262144(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call524288(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call1048576(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call2097152(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call4194304(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call8388608(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call16777216(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call33554432(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call67108864(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call134217728(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call268435456(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call536870912(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)
func call1073741824(typ, fn, stackArgs unsafe.Pointer, stackArgsSize, stackRetOffset, frameSize uint32, regArgs *abi.RegArgs)

func systemstack_switch()

// alignUp rounds n up to a multiple of a. a must be a power of 2.
// tag 算法
// 使用 &^ 运算符实现了将数字 n 向上舍入到指定对齐大小 a 的倍数的功能。
// 具体而言，我们执行的是 (n + a - 1) &^ (a - 1) 的操作，
// 即先将 n 加上 a - 1，得到向上舍入后最接近 n 的 a 的倍数，
// 然后再用 a - 1 进行按位取反，note 得到比 n 大的最小的 a 的倍数
// note &^=先对后面^,再和前面&，实现的效果：如果第二个的指定位为1，则第一个的指定位为0；如果第二个为0，则第一个不变
func alignUp(n, a uintptr) uintptr {
	return (n + a - 1) &^ (a - 1)
}

// alignDown rounds n down to a multiple of a. a must be a power of 2.
func alignDown(n, a uintptr) uintptr {
	return n &^ (a - 1)
}

// divRoundUp returns ceil(n / a).
func divRoundUp(n, a uintptr) uintptr {

	// a is generally a power of two. This will get inlined and
	// the compiler will optimize the division.

	// tag 算法 note 内联展开（inline expansion）是指编译器在编译过程中，将被调用的函数或方法的代码直接嵌入到调用其它地方的函数或方法的代码中，从而避免了函数调用的开销。
	//被内联展开的函数或方法会在编译时替换成它的实际代码，而不是在运行时进行函数调用的开销。
	//
	//内联展开可以提高程序的性能，因为函数调用本身需要额外的开销，如参数传递、栈帧的创建和销毁等。
	//而内联展开则直接将被调用的函数的代码插入到调用的位置，避免了这些开销。在一些情况下，特别是对于性能关键的代码段，手动进行内联展开是一种常见的优化手段。
	//
	//在 Golang 中，如果一个函数的代码体积比较小，且该函数经常被调用，编译器会自动将其内联展开。
	//通过使用 go build -gcflags=-m 命令可以查看编译器是否对某个函数进行内联展开。

	// note 除法向上取整，+(a-1)的目的是为了补偿n/a的余数与a之间的差距，从而实现向上取整；如果+的是a，那在n/a刚好整除的时候就会出问题了
	// 因为a刚好是2的整数次幂，所以这里的除法操作能够被编译器优化成移位操作，从而提高代码的性能
	return (n + a - 1) / a
}

// checkASM reports whether assembly runtime checks have passed.
func checkASM() bool

func memequal_varlen(a, b unsafe.Pointer) bool

// bool2int returns 0 if x is false or 1 if x is true.
func bool2int(x bool) int {
	// Avoid branches. In the SSA compiler, this compiles to
	// exactly what you would want it to.
	return int(uint8(*(*uint8)(unsafe.Pointer(&x))))
}

// abort crashes the runtime in situations where even throw might not
// work. In general it should do something a debugger will recognize
// (e.g., an INT3 on x86). A crash in abort is recognized by the
// signal handler, which will attempt to tear down the runtime
// immediately.
func abort()

// Called from compiled code; declared for vet; do NOT call from Go.
func gcWriteBarrier()
func duffzero()
func duffcopy()

// Called from linker-generated .initarray; declared for go vet; do NOT call from Go.
func addmoduledata()

// Injected by the signal handler for panicking signals.
// Initializes any registers that have fixed meaning at calls but
// are scratch in bodies and calls sigpanic.
// On many platforms it just jumps to sigpanic.
func sigpanic0()

// intArgRegs is used by the various register assignment
// algorithm implementations in the runtime. These include:.
// - Finalizers (mfinal.go)
// - Windows callbacks (syscall_windows.go)
//
// Both are stripped-down versions of the algorithm since they
// only have to deal with a subset of cases (finalizers only
// take a pointer or interface argument, Go Windows callbacks
// don't support floating point).
//
// It should be modified with care and are generally only
// modified when testing this package.
//
// It should never be set higher than its internal/abi
// constant counterparts, because the system relies on a
// structure that is at least large enough to hold the
// registers the system supports.
//
// Protected by finlock.
var intArgRegs = abi.IntArgRegs
