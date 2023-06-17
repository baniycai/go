// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Malloc small size classes.
//
// See malloc.go for overview.
// See also mksizeclasses.go for how we decide what size classes to use.

package runtime

// Returns size of the memory block that mallocgc will allocate if you ask for the size.
func roundupsize(size uintptr) uintptr { // 申请的内存应该以byte为单位的
	if size < _MaxSmallSize {
		if size <= smallSizeMax-8 {
			// 在 roundupsize 函数中，对 smallSizeDiv 进行向上取整是为了保证计算出来的索引值能够覆盖到所有可能的内存大小范围。
			//
			//具体来说，smallSizeDiv 是一个常量，它表示小内存块的尺寸倍数。在实际使用中，小内存块的实际大小可能略微大于 smallSizeMax / smallSizeDiv，所以我们需要将请求的内存大小除以 smallSizeDiv 并向上取整，来得到一个可以覆盖所有小内存块的索引值。这样做可以确保计算出来的索引下标一定落在 size_to_class8 数组的合法范围内，并且对应的内存块大小也足够满足请求的内存大小。
			//
			//需要注意的是，在 Go 的运行时系统中，smallSizeDiv 的值通常为 8（即 smallSizeDiv = 8），因此这里的向上取整操作实际上等价于将请求的内存大小除以 8 并向上取整。
			return uintptr(class_to_size[size_to_class8[divRoundUp(size, smallSizeDiv)]])
		} else {
			return uintptr(class_to_size[size_to_class128[divRoundUp(size-smallSizeMax, largeSizeDiv)]])
		}
	}
	if size+_PageSize < size {
		return size
	}
	return alignUp(size, _PageSize)
}
