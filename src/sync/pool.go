// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"runtime"
	"std/internal/race"
	"sync/atomic"
	"unsafe"
)

// A Pool is a set of temporary objects that may be individually saved and
// retrieved.
//
// Any item stored in the Pool may be removed automatically at any time without
// notification. If the Pool holds the only reference when this happens, the
// item might be deallocated.
//
// A Pool is safe for use by multiple goroutines simultaneously.
//
// Pool's purpose is to cache allocated but unused items for later reuse,
// relieving pressure on the garbage collector. That is, it makes it easy to
// build efficient, thread-safe free lists. However, it is not suitable for all
// free lists.
//
// An appropriate use of a Pool is to manage a group of temporary items
// silently shared among and potentially reused by concurrent independent
// clients of a package. Pool provides a way to amortize allocation overhead
// across many clients.
//
// An example of good use of a Pool is in the fmt package, which maintains a
// dynamically-sized store of temporary output buffers. The store scales under
// load (when many goroutines are actively printing) and shrinks when
// quiescent.
//
// On the other hand, a free list maintained as part of a short-lived object is
// not a suitable use for a Pool, since the overhead does not amortize well in
// that scenario. It is more efficient to have such objects implement their own
// free list.
//
// A Pool must not be copied after first use.
//
// In the terminology of the Go memory model, a call to Put(x) “synchronizes before”
// a call to Get returning that same value x.
// Similarly, a call to New returning x “synchronizes before”
// a call to Get returning that same value x.
type Pool struct {
	noCopy noCopy

	local     unsafe.Pointer // local fixed-size per-P pool, actual type is [P]poolLocal  指针，指向poolLocal数组的第一个元素
	localSize uintptr        // size of the local array			地址值，该地址存储的是当前Pool中存储的本地池的大小

	victim     unsafe.Pointer // local from previous cycle
	victimSize uintptr        // size of victims array

	// New optionally specifies a function to generate
	// a value when Get would otherwise return nil.
	// It may not be changed concurrently with calls to Get.
	New func() any
}

// Local per-P Pool appendix.
type poolLocalInternal struct {
	// 放any的时候优先放在private中，private不为nil则放到shared这个链表中
	// 取的时候也是优先从private取，取不到再走shared链表
	private any       // Can be used only by the respective P.
	shared  poolChain // Local P can pushHead/popHead; any P can popTail.
}

// 这个结构体是在Go语言标准库中的sync包中定义的，它实现了一个goroutine池。
// 具体来说，它是每个P（处理器）的本地goroutine池的私有部分。
//
// 这个结构体的最重要的成员是poolLocalInternal，它是一个包含链表头和尾指针以及其他一些状态信息的内部结构体。
// 链表用于存储可重用的goroutine，每当需要新的goroutine时，都会从这个链表中取出一个。
//
// 由于这个结构体是每个P的本地结构体，因此它可以避免多个P之间的竞争，并且可以提高性能。
// 为了更好地利用现代CPU的缓存机制，在该结构体中添加了一个名为pad的数组。这个数组的目的是填充字节对齐的空间，防止不必要的伪共享。
//
// 总的来说，这个结构体是Go语言中实现goroutine池的关键，它通过避免竞争和优化缓存来提高程序的性能。
type poolLocal struct {
	poolLocalInternal

	// Prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	pad [128 - unsafe.Sizeof(poolLocalInternal{})%128]byte
}

// 在现代CPU中，缓存是一种重要的性能优化技术。缓存通常以线或块的形式组织，每个线或块包含许多相邻的内存位置。
//当一个线或块中的某个位置发生变化时，整个线或块将被复制到CPU缓存中。这可以大大提高代码的执行速度，因为从缓存中读取数据比从主存中读取数据要快得多。
//不幸的是，当多个线程同时更新同一个缓存线（如不同的CPU核心更新同一行内存）时，缓存就会失效，因为每个线程都需要复制自己的版本。
//这导致了所谓的“伪共享”，即多个线程因为竞争同一个缓存行而导致性能下降。
//为了防止伪共享，编程人员通常使用填充字节来     确保结构体中的不同变量之间具有足够的间距     ，这样它们就不会在同一个缓存线上。
//在这个特定的例子中，结构体中添加了一个名为pad的数组来填充字节，以确保poolLocalInternal类型的实例之间具有足够的间隔。
//这样，即使多个goroutine同时访问本地池，它们也可以在不影响彼此的情况下更新自己的缓存行。

// 当几个线程同时访问同一个缓存行时，就可能导致伪共享问题。
// 例如，假设有两个线程同时更新下面的结构体中的x和y变量：
//
//struct {
//  int x;
//  int y;
//} myStruct;
//如果这两个线程运行在不同的CPU核心上，并且它们同时更新了相同的结构体实例，那么它们将会竞争同一个缓存行。这意味着每个线程都需要复制自己的版本到它们各自的CPU缓存中，这会导致缓存行失效并影响性能。
//
//为了解决这个问题，可以使用填充字节来增加变量之间的间距，如下所示：
//
//struct {
//  int x;
//  char padding[60]; // 添加填充字节
//  int y;
//  char padding2[60]; // 再次添加填充字节
//} myStruct;
//通过添加填充字节，x和y之间的距离被增加了，这样它们就不会共享同一个缓存行，从而避免了伪共享问题。
// 可以把填充字节理解为一种减少失效缓存行数量的技术。填充字节增加了变量之间的距离，从而使它们不会共享同一个缓存行，这样就可以避免伪共享问题

// 这个函数使用了一种名为 "FastRand" 的算法来生成伪随机数。
// FastRand 算法是一种基于汇编代码的快速随机数生成器，它能在不使用锁或其他同步机制的情况下产生高质量的随机数。
// 由于 FastRand 算法并非真正意义上的随机数生成器，因此它的性质与真正的随机数生成器略有不同，但在大多数情况下都能满足需求。
// 因此，在需要简单而快速地生成随机数的场景中，可以使用 fastrandn 函数来代替更加复杂的随机数生成器
// from runtime
func fastrandn(n uint32) uint32

var poolRaceHash [128]uint64

// 是一个用于获取 x 对象关联的 race 检查资源的函数，它会返回一个 uintptr 类型的结果。
// 在此例中，它会返回一个 uintptr，该值是 x 对象在池中的地址加上一个偏移量，以便与其它的竞争检查资源区分开来.todo
// poolRaceAddr returns an address to use as the synchronization point
// for race detector logic. We don't use the actual pointer stored in x
// directly, for fear of conflicting with other synchronization on that address.
// Instead, we hash the pointer to get an index into poolRaceHash.
// See discussion on golang.org/cl/31589.
func poolRaceAddr(x any) unsafe.Pointer {
	ptr := uintptr((*[2]unsafe.Pointer)(unsafe.Pointer(&x))[1])
	h := uint32((uint64(uint32(ptr)) * 0x85ebca6b) >> 16)
	return unsafe.Pointer(&poolRaceHash[h%uint32(len(poolRaceHash))])
}

// NOTE 总的来说，就是维护了一个链表吧，put的时候放到链表中，get的时候从链表中拿走
// 还有一个重要的点，链表中的缓存是以线程为单位的(即每个线程一份)，所以如果一个协程被调度到另一个线程了，那可能就拿不到前面put的东西了???
// 通常会通过调用Runtime_procPin()来绑定协程到线程上
// 还有一个点，每份缓存主要有两个部分，一个是private，一个是链表，put的时候先放到private，private！=nil再放链表，get的时候是先从private拿
// 当然，还有很多骚操作，比如协程独占线程，然后还有一堆优化逻辑，没太看懂(细看)

// Put adds x to the pool.
func (p *Pool) Put(x any) {
	if x == nil {
		return
	}
	if race.Enabled {
		if fastrandn(4) == 0 {
			// Randomly drop x on floor.
			return
		}
		race.ReleaseMerge(poolRaceAddr(x))
		race.Disable()
	}
	l, _ := p.pin()
	if l.private == nil {
		l.private = x
	} else {
		l.shared.pushHead(x)
	}
	runtime_procUnpin()
	if race.Enabled {
		race.Enable()
	}
}

// Get selects an arbitrary item from the Pool, removes it from the
// Pool, and returns it to the caller.
// Get may choose to ignore the pool and treat it as empty.
// Callers should not assume any relation between values passed to Put and
// the values returned by Get.
//
// If Get would otherwise return nil and p.New is non-nil, Get returns
// the result of calling p.New.
func (p *Pool) Get() any {
	// 这段代码是用来禁用 Go 语言程序中的竞争检测工具（Race Detector）的。
	//在 Go 语言标准库中，有一个叫做 race 的包，其中包含了一些用于对应用程序进行竞争检测的函数和变量。
	//这段代码中的 if race.Enabled 检查当前是否启用了竞争检测，如果启用了，则调用 race.Disable() 函数来禁用竞争检测。
	//禁用竞争检测通常是为了提高程序的性能，在生产环境中可能会经常使用。
	//但需要注意的是，禁用竞争检测可能会让程序中的并发问题难以被发现和调试，因此在开发和测试过程中仍然应该启用竞争检测来保证代码的正确性。
	if race.Enabled {
		race.Disable()
	}
	// 调用 pin() 方法获取当前协程关联的本地页（local shard）和池的 ID 号。todo
	l, pid := p.pin()
	x := l.private // 从本地页中取出私有资源，并置空以便下次使用
	l.private = nil
	if x == nil {
		// 若本地页中的私有资源为空，则先尝试从共享资源队列中弹出一个资源。
		//如果共享资源队列也为空，则调用 getSlow 方法创建一个新的资源。
		//这里的目的是利用局部性原理（temporal locality）提高资源的重用率。
		// Try to pop the head of the local shard. We prefer
		// the head over the tail for temporal locality of
		// reuse.
		x, _ = l.shared.popHead()
		if x == nil {
			x = p.getSlow(pid)
		}
	}
	runtime_procUnpin() // 将当前协程从本地页上解除关联
	if race.Enabled {
		race.Enable()
		if x != nil {
			race.Acquire(poolRaceAddr(x)) // 使用 poolRaceAddr(x) 函数获取其内存地址，并调用 Acquire 函数告诉竞争检测工具这段代码正在访问这个资源
		}
	}
	if x == nil && p.New != nil {
		x = p.New()
	}
	return x
}

func (p *Pool) getSlow(pid int) any {
	// See the comment in pin regarding ordering of the loads.
	size := runtime_LoadAcquintptr(&p.localSize) // load-acquire
	locals := p.local                            // load-consume
	// Try to steal one element from other procs.
	for i := 0; i < int(size); i++ {
		l := indexLocal(locals, (pid+i+1)%int(size))
		if x, _ := l.shared.popTail(); x != nil {
			return x
		}
	}

	// Try the victim cache. We do this after attempting to steal
	// from all primary caches because we want objects in the
	// victim cache to age out if at all possible.
	size = atomic.LoadUintptr(&p.victimSize)
	if uintptr(pid) >= size {
		return nil
	}
	locals = p.victim
	l := indexLocal(locals, pid)
	if x := l.private; x != nil {
		l.private = nil
		return x
	}
	for i := 0; i < int(size); i++ {
		l := indexLocal(locals, (pid+i)%int(size))
		if x, _ := l.shared.popTail(); x != nil {
			return x
		}
	}

	// Mark the victim cache as empty for future gets don't bother
	// with it.
	atomic.StoreUintptr(&p.victimSize, 0)

	return nil
}

// 这段代码是Go语言标准库中的sync.Pool（同步池）的实现中的一个方法，用于从一个Pool结构体中获取一个poolLocal对象并将其和当前的goroutine ID绑定。
//具体来说，该方法执行以下操作：
// 执行runtime_procPin()函数，该函数会将当前goroutine与某个P (Processor) 绑定，防止在后续的运行过程中被抢占。
// 使用p.localSize字段的原子加载( runtime_LoadAcquintptr )操作读取当前Pool中存储的本地池的大小。
// 使用p.local字段的普通加载操作读取当前Pool中存储的本地池的指针。
// 如果当前goroutine ID小于p.localSize，则使用indexLocal函数从本地池中获取一个poolLocal对象，并返回该对象及当前goroutine ID。
// 否则，调用pinSlow函数，该函数会动态扩展本地池并返回一个新的poolLocal对象。
// 在Go语言中，同步池（sync.Pool）是一种可以存放临时对象的对象池，它可以帮助开发人员在不频繁创建和销毁对象的情况下，提高内存的利用率和程序的性能。
// 在使用同步池时，开发人员需要避免在多个goroutine之间共享临时对象，因为同步池并不能保证对象的唯一性。

// pin pins the current goroutine to P, disables preemption and
// returns poolLocal pool for the P and the P's id.
// Caller must call runtime_procUnpin() when done with the pool.
func (p *Pool) pin() (*poolLocal, int) {
	pid := runtime_procPin() // 将协程绑定到线程上，应该是独占的。这里的pid应该是线程id，从0开始编号，所以可以在poolLocal数组上进行索引
	// In pinSlow we store to local and then to localSize, here we load in opposite order.
	// Since we've disabled preemption, GC cannot happen in between.
	// Thus here we must observe local at least as large localSize.
	// We can observe a newer/larger local, it is fine (we must observe its zero-initialized-ness).
	s := runtime_LoadAcquintptr(&p.localSize) // load-acquire   加载poolLocal数量
	l := p.local                              // load-consume
	if uintptr(pid) < s {                     // 在poolLocal索引内则直接用
		return indexLocal(l, pid), pid
	}
	return p.pinSlow()
}

// 可以认为是初始化p叭？
func (p *Pool) pinSlow() (*poolLocal, int) {
	// Retry under the mutex.
	// Can not lock the mutex while pinned.
	runtime_procUnpin()
	allPoolsMu.Lock()
	defer allPoolsMu.Unlock()
	pid := runtime_procPin()
	// poolCleanup won't be called while we are pinned.
	s := p.localSize
	l := p.local
	if uintptr(pid) < s {
		return indexLocal(l, pid), pid
	}
	if p.local == nil {
		allPools = append(allPools, p)
	}
	// If GOMAXPROCS changes between GCs, we re-allocate the array and lose the old one.
	size := runtime.GOMAXPROCS(0)                            // 应该是当前正在运行的线程数量(M),或者说P的数量？
	local := make([]poolLocal, size)                         // 每个线程一个poolLocal
	atomic.StorePointer(&p.local, unsafe.Pointer(&local[0])) // store-release   local指向第一个pollLocal的地址
	runtime_StoreReluintptr(&p.localSize, uintptr(size))     // store-release   将存放poolLocal大小的变量的地址保存起来
	return &local[pid], pid                                  // 通过pid来索引，说明golang的pid是从0开始算的
}

func poolCleanup() {
	// This function is called with the world stopped, at the beginning of a garbage collection.
	// It must not allocate and probably should not call any runtime functions.

	// Because the world is stopped, no pool user can be in a
	// pinned section (in effect, this has all Ps pinned).

	// Drop victim caches from all pools.
	for _, p := range oldPools {
		p.victim = nil
		p.victimSize = 0
	}

	// Move primary cache to victim cache.
	for _, p := range allPools {
		p.victim = p.local
		p.victimSize = p.localSize
		p.local = nil
		p.localSize = 0
	}

	// The pools with non-empty primary caches now have non-empty
	// victim caches and no pools have primary caches.
	oldPools, allPools = allPools, nil
}

var (
	allPoolsMu Mutex

	// allPools is the set of pools that have non-empty primary
	// caches. Protected by either 1) allPoolsMu and pinning or 2)
	// STW.
	allPools []*Pool

	// oldPools is the set of pools that may have non-empty victim
	// caches. Protected by STW.
	oldPools []*Pool
)

func init() {
	runtime_registerPoolCleanup(poolCleanup)
}

func indexLocal(l unsafe.Pointer, i int) *poolLocal {
	lp := unsafe.Pointer(uintptr(l) + uintptr(i)*unsafe.Sizeof(poolLocal{}))
	return (*poolLocal)(lp)
}

// Implemented in runtime.
func runtime_registerPoolCleanup(cleanup func()) // 注册一个池清理函数，当需要清理池时，该函数会被调用。它可以用于在内存使用量较高时清理池，以避免内存泄漏
// 该函数可能会将当前 goroutine 绑定到 OS 线程上，以确保在此期间禁用抢占机制
func runtime_procPin() int // 用于将当前的 goroutine 固定在其所在的处理器上。如果没有可用的处理器，则该方法会阻塞等待，直到有处理器可用。它可以用于确保某个 goroutine 只在指定的处理器上运行；该函数用于将当前 goroutine（协程）绑定到特定的处理器上。如果调用成功，则返回处理器的 ID
func runtime_procUnpin()   // 用于取消当前 goroutine 在处理器上的固定。如果当前 goroutine 没有固定任何处理器，则该方法不会产生任何影响

// The below are implemented in runtime/internal/atomic and the
// compiler also knows to intrinsify the symbol we linkname into this
// package.

// runtime_LoadAcquintptr 函数是 Go语言中的内部函数，用于原子性地加载指向 uintptr 的指针。
// 在 Go 语言中，uintptr 类型表示一个无符号整数，它足以存储任何指针值。
// 该函数的作用是将指针指向的地址上的值（一个无符号整数）读取出来，并返回该值。
// 在函数定义中，参数 ptr 是一个 uintptr 类型的指针的指针，
// 在函数调用时，可以将一个 uintptr 类型的变量的地址传递给 ptr，函数会将该变量的值原子性地赋值给返回值并返回。
// 需要注意的是，该函数是 Go语言运行时库中的一个内部函数，不推荐在普通的应用程序中使用
//
//go:linkname runtime_LoadAcquintptr runtime/internal/atomic.LoadAcquintptr
func runtime_LoadAcquintptr(ptr *uintptr) uintptr

//go:linkname runtime_StoreReluintptr runtime/internal/atomic.StoreReluintptr
func runtime_StoreReluintptr(ptr *uintptr, val uintptr) uintptr
