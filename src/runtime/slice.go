// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"std/internal/abi"
	"std/internal/goarch"
	"std/runtime/internal/math"
	"std/runtime/internal/sys"
	"std/unsafe"
)

type slice struct {
	array unsafe.Pointer
	len   int
	cap   int
}

// A notInHeapSlice is a slice backed by go:notinheap memory.
type notInHeapSlice struct {
	array *notInHeap
	len   int
	cap   int
}

func panicmakeslicelen() {
	panic(errorString("makeslice: len out of range"))
}

func panicmakeslicecap() {
	panic(errorString("makeslice: cap out of range"))
}

// makeslicecopy allocates a slice of "tolen" elements of type "et",
// then copies "fromlen" elements of type "et" into that new allocation from "from".
func makeslicecopy(et *_type, tolen int, fromlen int, from unsafe.Pointer) unsafe.Pointer {
	var tomem, copymem uintptr
	if uintptr(tolen) > uintptr(fromlen) {
		var overflow bool
		tomem, overflow = math.MulUintptr(et.size, uintptr(tolen))
		if overflow || tomem > maxAlloc || tolen < 0 {
			panicmakeslicelen()
		}
		copymem = et.size * uintptr(fromlen)
	} else {
		// fromlen is a known good length providing and equal or greater than tolen,
		// thereby making tolen a good slice length too as from and to slices have the
		// same element width.
		tomem = et.size * uintptr(tolen)
		copymem = tomem
	}

	var to unsafe.Pointer
	if et.ptrdata == 0 {
		to = mallocgc(tomem, nil, false)
		if copymem < tomem {
			memclrNoHeapPointers(add(to, copymem), tomem-copymem)
		}
	} else {
		// Note: can't use rawmem (which avoids zeroing of memory), because then GC can scan uninitialized memory.
		to = mallocgc(tomem, et, true)
		if copymem > 0 && writeBarrier.enabled {
			// Only shade the pointers in old.array since we know the destination slice to
			// only contains nil pointers because it has been cleared during alloc.
			bulkBarrierPreWriteSrcOnly(uintptr(to), uintptr(from), copymem)
		}
	}

	if raceenabled {
		callerpc := getcallerpc()
		pc := abi.FuncPCABIInternal(makeslicecopy)
		racereadrangepc(from, copymem, callerpc, pc)
	}
	if msanenabled {
		msanread(from, copymem)
	}
	if asanenabled {
		asanread(from, copymem)
	}

	memmove(to, from, copymem)

	return to
}

func makeslice(et *_type, len, cap int) unsafe.Pointer {
	mem, overflow := math.MulUintptr(et.size, uintptr(cap)) // note 每个元素的大小*cap得到需要的总空间大小
	if overflow || mem > maxAlloc || len < 0 || len > cap {
		// NOTE: Produce a 'len out of range' error instead of a
		// 'cap out of range' error when someone does make([]T, bignumber).
		// 'cap out of range' is true too, but since the cap is only being
		// supplied implicitly, saying len is clearer.
		// See golang.org/issue/4085.
		mem, overflow := math.MulUintptr(et.size, uintptr(len)) // note cap需要的空间太大了，则用len*size来搞
		if overflow || mem > maxAlloc || len < 0 {
			panicmakeslicelen()
		}
		panicmakeslicecap()
	}

	return mallocgc(mem, et, true)
}

func makeslice64(et *_type, len64, cap64 int64) unsafe.Pointer {
	len := int(len64)
	if int64(len) != len64 {
		panicmakeslicelen()
	}

	cap := int(cap64)
	if int64(cap) != cap64 {
		panicmakeslicecap()
	}

	return makeslice(et, len, cap)
}

// This is a wrapper over runtime/internal/math.MulUintptr,
// so the compiler can recognize and treat it as an intrinsic.
func mulUintptr(a, b uintptr) (uintptr, bool) {
	return math.MulUintptr(a, b)
}

// Keep this code in sync with cmd/compile/internal/walk/builtin.go:walkUnsafeSlice
func unsafeslice(et *_type, ptr unsafe.Pointer, len int) {
	if len < 0 {
		panicunsafeslicelen()
	}

	mem, overflow := math.MulUintptr(et.size, uintptr(len))
	if overflow || mem > -uintptr(ptr) {
		if ptr == nil {
			panicunsafeslicenilptr()
		}
		panicunsafeslicelen()
	}
}

// Keep this code in sync with cmd/compile/internal/walk/builtin.go:walkUnsafeSlice
func unsafeslice64(et *_type, ptr unsafe.Pointer, len64 int64) {
	len := int(len64)
	if int64(len) != len64 {
		panicunsafeslicelen()
	}
	unsafeslice(et, ptr, len)
}

func unsafeslicecheckptr(et *_type, ptr unsafe.Pointer, len64 int64) {
	unsafeslice64(et, ptr, len64)

	// Check that underlying array doesn't straddle multiple heap objects.
	// unsafeslice64 has already checked for overflow.
	if checkptrStraddles(ptr, uintptr(len64)*et.size) {
		throw("checkptr: unsafe.Slice result straddles multiple allocations")
	}
}

func panicunsafeslicelen() {
	panic(errorString("unsafe.Slice: len out of range"))
}

func panicunsafeslicenilptr() {
	panic(errorString("unsafe.Slice: ptr is nil and len is not zero"))
}

// growslice handles slice growth during append.
// It is passed the slice element type, the old slice, and the desired new minimum capacity,
// and it returns a new slice with at least that capacity, with the old data
// copied into it.
// The new slice's length is set to the old slice's length,
// NOT to the new requested capacity.
// This is for codegen convenience. The old slice's length is used immediately
// to calculate where to write new values during an append.
// TODO: When the old backend is gone, reconsider this decision.
// The SSA backend might prefer the new length or to return only ptr/cap and save stack space.
func growslice(et *_type, old slice, cap int) slice {
	if raceenabled {
		callerpc := getcallerpc()
		racereadrangepc(old.array, uintptr(old.len*int(et.size)), callerpc, abi.FuncPCABIInternal(growslice))
	}
	if msanenabled {
		msanread(old.array, uintptr(old.len*int(et.size)))
	}
	if asanenabled {
		asanread(old.array, uintptr(old.len*int(et.size)))
	}

	if cap < old.cap {
		panic(errorString("growslice: cap out of range"))
	}

	if et.size == 0 {
		// append should not create a slice with nil pointer but non-zero len.
		// We assume that append doesn't need to preserve old.array in this case.
		return slice{unsafe.Pointer(&zerobase), old.len, cap}
	}

	newcap := old.cap
	doublecap := newcap + newcap
	if cap > doublecap { // note  requirecap>doublecap，=requirecap
		newcap = cap
	} else { //  note requirecap<doublecap
		const threshold = 256
		if old.cap < threshold { // note cap没到达256这个阈值，=doublecap
			newcap = doublecap
		} else {
			// Check 0 < newcap to detect overflow
			// and prevent an infinite loop.

			// 如果你采用每次增加原大小的50%的方式来扩容，当切片容量比较小的时候，这种方式可以快速扩大容量；但当容量比较大的时候，仍然按照每次增加50%的方式，可能会造成内存分配过多，导致内存浪费。
			//
			//另外，如果采用固定的增长率，可能会出现以下两种情况：
			//
			//当容量比较小时，增长率不足以快速扩容。
			//当容量比较大时，增长率过低，可能使得切片无法满足需求，导致内存浪费。
			//因此，需要一种平滑过渡的扩容策略，通过动态调整增长率，使得切片在容量不断增大的情况下，能够实现高效、稳定的内存分配。
			//具体而言，逐步降低增长率能够使得增长速度慢慢减缓，同时保证内存分配的效率和使用的空间，避免了内存的浪费。
			//
			//需要注意的是，平滑过渡的扩容策略并不是唯一的选择，具体的扩容策略可以根据实际应用场景的需求进行调整。

			// note 这个增加原值的25%和一个固定的阈值256的75%的计算方法，是为了让新容量的增长率逐渐减小，从而使得增长过程更加平滑。
			//
			//具体来说，当切片容量比较小时，增长率接近50%，newcap会快速增长；随着newcap的增大，增长率会逐渐减小，newcap增长趋于缓慢，最终达到每次增加25%的稳定状态。
			//如果采用固定的增长率，可能会造成在容量较小时内存分配过多，导致内存浪费；而在容量较大时增长率过低，则可能会导致切片无法满足需求。
			//
			//具体地，(newcap + 3*threshold) / 4 表示将newcap的大小增加原有大小的25%和一个固定的阈值(3*threshold)的大小之和，然后再除以4，从而获得新的容量大小。
			//其中，阈值threshold 的设定是为了确保在切片容量较小时，增长率能够达到50%，使得容量可以快速增长；而随着容量的增大，增长率会逐渐减小，直到最终稳定在25%左右，防止出现内存浪费。
			// note 按我理解，应该是一开始newcap比较小，显得threshold比较大，占比比较高，75%+25%cap实现一个类似50%的效果，后面newcap数量上来了，threshold的大小就几乎可以忽略了，基本就是25%cap增长了
			// 没有直接25%cap，可能去怕一开始cap太小了，25%增长比较慢，所以加个75%threshold来帮一下；不过它这里是用个for循环的，我雀氏有点震惊
			for 0 < newcap && newcap < cap { // note 至少要比cap大
				// Transition from growing 2x for small slices
				// to growing 1.25x for large slices. This formula
				// gives a smooth-ish transition between the two.
				newcap += (newcap + 3*threshold) / 4
			}
			// Set newcap to the requested cap when
			// the newcap calculation overflowed.
			if newcap <= 0 {
				newcap = cap
			}
		}
	}

	var overflow bool
	// lenmem->old.len,newlenmem->调用该函数欲申请的cap,capmem->实际计算出来要申请的cap
	var lenmem, newlenmem, capmem uintptr
	// Specialize for common values of et.size.
	// For 1 we don't need any division/multiplication.
	// For goarch.PtrSize, compiler will optimize division/multiplication into a shift by a constant.
	// For powers of 2, use a variable shift.
	// 专用于 et.size 的公共值。
	// 对于 1，我们不需要任何除法/乘法。
	// 对于 goarch.PtrSize，编译器会将除法/乘法优化为一个常量移位。
	// 对于 2 的幂，使用变量 shift。
	switch {
	case et.size == 1:
		lenmem = uintptr(old.len)
		newlenmem = uintptr(cap)
		capmem = roundupsize(uintptr(newcap)) // 需要分配的内存块大小
		overflow = uintptr(newcap) > maxAlloc
		newcap = int(capmem)
	case et.size == goarch.PtrSize:
		lenmem = uintptr(old.len) * goarch.PtrSize
		newlenmem = uintptr(cap) * goarch.PtrSize
		capmem = roundupsize(uintptr(newcap) * goarch.PtrSize)
		overflow = uintptr(newcap) > maxAlloc/goarch.PtrSize
		newcap = int(capmem / goarch.PtrSize)
	case isPowerOfTwo(et.size):
		var shift uintptr
		if goarch.PtrSize == 8 {
			// Mask shift for better code generation.
			shift = uintptr(sys.Ctz64(uint64(et.size))) & 63
		} else {
			shift = uintptr(sys.Ctz32(uint32(et.size))) & 31
		}
		lenmem = uintptr(old.len) << shift
		newlenmem = uintptr(cap) << shift
		capmem = roundupsize(uintptr(newcap) << shift)
		overflow = uintptr(newcap) > (maxAlloc >> shift)
		newcap = int(capmem >> shift)
	default:
		lenmem = uintptr(old.len) * et.size
		newlenmem = uintptr(cap) * et.size
		capmem, overflow = math.MulUintptr(et.size, uintptr(newcap))
		capmem = roundupsize(capmem)
		newcap = int(capmem / et.size)
	}

	// The check of overflow in addition to capmem > maxAlloc is needed
	// to prevent an overflow which can be used to trigger a segfault
	// on 32bit architectures with this example program:
	//
	// type T [1<<27 + 1]int64
	//
	// var d T
	// var s []T
	//
	// func main() {
	//   s = append(s, d, d, d, d)
	//   print(len(s), "\n")
	// }
	if overflow || capmem > maxAlloc {
		panic(errorString("growslice: cap out of range"))
	}
	// note 分配内存(这里返回的p应该是分配的内存的字段)，并copy旧slice的array，包装一个新的slice返回
	var p unsafe.Pointer
	if et.ptrdata == 0 {
		p = mallocgc(capmem, nil, false)
		// The append() that calls growslice is going to overwrite from old.len to cap (which will be the new length).
		// Only clear the part that will not be overwritten.
		memclrNoHeapPointers(add(p, newlenmem), capmem-newlenmem)
	} else {
		// Note: can't use rawmem (which avoids zeroing of memory), because then GC can scan uninitialized memory.
		p = mallocgc(capmem, et, true)
		if lenmem > 0 && writeBarrier.enabled {
			// Only shade the pointers in old.array since we know the destination slice p
			// only contains nil pointers because it has been cleared during alloc.
			bulkBarrierPreWriteSrcOnly(uintptr(p), uintptr(old.array), lenmem-et.size+et.ptrdata)
		}
	}
	memmove(p, old.array, lenmem)

	return slice{p, old.len, newcap}
}

// tag 算法，力扣常见算法，嘿嘿
func isPowerOfTwo(x uintptr) bool {
	return x&(x-1) == 0
}

// slicecopy is used to copy from a string or slice of pointerless elements into a slice.
func slicecopy(toPtr unsafe.Pointer, toLen int, fromPtr unsafe.Pointer, fromLen int, width uintptr) int {
	if fromLen == 0 || toLen == 0 {
		return 0
	}

	n := fromLen
	if toLen < n {
		n = toLen
	}

	if width == 0 {
		return n
	}

	size := uintptr(n) * width
	if raceenabled {
		callerpc := getcallerpc()
		pc := abi.FuncPCABIInternal(slicecopy)
		racereadrangepc(fromPtr, size, callerpc, pc)
		racewriterangepc(toPtr, size, callerpc, pc)
	}
	if msanenabled {
		msanread(fromPtr, size)
		msanwrite(toPtr, size)
	}
	if asanenabled {
		asanread(fromPtr, size)
		asanwrite(toPtr, size)
	}

	if size == 1 { // common case worth about 2x to do here
		// TODO: is this still worth it with new memmove impl?
		*(*byte)(toPtr) = *(*byte)(fromPtr) // known to be a byte pointer
	} else {
		memmove(toPtr, fromPtr, size)
	}
	return n
}
