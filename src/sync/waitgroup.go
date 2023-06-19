// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import (
	"std/internal/race"
	"std/sync/atomic"
	"std/unsafe"
)

// A WaitGroup waits for a collection of goroutines to finish.
// The main goroutine calls Add to set the number of
// goroutines to wait for. Then each of the goroutines
// runs and calls Done when finished. At the same time,
// Wait can be used to block until all goroutines have finished.
//
// A WaitGroup must not be copied after first use.
//
// In the terminology of the Go memory model, a call to Done
// “synchronizes before” the return of any Wait call that it unblocks.
type WaitGroup struct {
	noCopy noCopy

	// 64-bit value: high 32 bits are counter, low 32 bits are waiter count.
	// 64-bit atomic operations require 64-bit alignment, but 32-bit
	// compilers only guarantee that 64-bit fields are 32-bit aligned.
	// For this reason on 32 bit architectures we need to check in state()
	// if state1 is aligned or not, and dynamically "swap" the field order if
	// needed.
	// 64位值：高32位为计数器，低32位为等待者计数。note 64 位原子操作需要 64 位对齐，但是 32 位编译器只保证 64 位字段是 32 位对齐的。
	//出于这个原因，在 32 位架构上，我们需要检查 state() 是否对齐 state1，并在需要时动态“交换”字段顺序。
	state1 uint64
	state2 uint32
}

// state returns pointers to the state and sema fields stored within wg.state*.
// 字节对齐相关，先不管
func (wg *WaitGroup) state() (statep *uint64, semap *uint32) {
	if unsafe.Alignof(wg.state1) == 8 || uintptr(unsafe.Pointer(&wg.state1))%8 == 0 {
		// state1 is 64-bit aligned: nothing to do.
		return &wg.state1, &wg.state2
	} else {
		// state1 is 32-bit aligned but not 64-bit aligned: this means that
		// (&state1)+4 is 64-bit aligned.
		state := (*[3]uint32)(unsafe.Pointer(&wg.state1))
		return (*uint64)(unsafe.Pointer(&state[1])), &state[0]
	}
}

// Add adds delta, which may be negative, to the WaitGroup counter.
// If the counter becomes zero, all goroutines blocked on Wait are released.
// If the counter goes negative, Add panics.
//
// Note that calls with a positive delta that occur when the counter is zero
// must happen before a Wait. Calls with a negative delta, or calls with a
// positive delta that start when the counter is greater than zero, may happen
// at any time.
// Typically this means the calls to Add should execute before the statement
// creating the goroutine or other event to be waited for.
// If a WaitGroup is reused to wait for several independent sets of events,
// new Add calls must happen after all previous Wait calls have returned.
// See the WaitGroup example.
// 添加向 WaitGroup 计数器添加可能为负的增量。如果计数器变为零，则释放所有阻塞在 Wait 上的 goroutine。
// 如果计数器变为负数，则 Add 恐慌。请注意，当计数器为零时发生的具有正增量的调用必须发生在 Wait 之前。
// 具有负增量的调用或在计数器大于零时开始的具有正增量的调用可能随时发生。
// note 通常这意味着对 Add 的调用应该在创建 goroutine 或其他要等待的事件的语句之前执行。
// 如果重复使用 WaitGroup 来等待多个独立的事件集，则必须在所有先前的 Wait 调用返回后发生新的 Add 调用。
func (wg *WaitGroup) Add(delta int) {
	statep, semap := wg.state()
	if race.Enabled {
		_ = *statep // trigger nil deref early
		if delta < 0 {
			// Synchronize decrements with Wait.
			race.ReleaseMerge(unsafe.Pointer(wg))
		}
		race.Disable()
		defer race.Enable()
	}
	state := atomic.AddUint64(statep, uint64(delta)<<32) // note 高32位为计数器，低32位为等待者计数；这里是加计数器，当然，delta也可能是负值
	v := int32(state >> 32)                              // 加完的计数器值
	w := uint32(state)
	if race.Enabled && delta > 0 && v == int32(delta) {
		// The first increment must be synchronized with Wait.
		// Need to model this as a read, because there can be
		// several concurrent wg.counter transitions from 0.
		race.Read(unsafe.Pointer(semap))
	}
	if v < 0 { // 计数器为负直接panic
		panic("sync: negative WaitGroup counter")
	}
	if w != 0 && delta > 0 && v == int32(delta) { // note 并发调用Add和Wait，所以出现了有等待者，但是计数器却等于delta的情况
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	if v > 0 || w == 0 { // note 加完之后，计数器大于0,则不用唤醒；等待者为0，也不用唤醒，直接返回
		return
	}
	// note 下面是需要唤醒的场景
	// This goroutine has set counter to 0 when waiters > 0.
	// Now there can't be concurrent mutations of state:
	// - Adds must not happen concurrently with Wait,
	// - Wait does not increment waiters if it sees counter == 0.
	// Still do a cheap sanity check to detect WaitGroup misuse.
	// 当 waiters > 0 时，这个 goroutine 将计数器设置为 0。
	//现在不能有状态的并发突变：
	//- Add 不能与 Wait 同时发生，
	//- 如果 Wait 看到计数器 == 0，它不会增加 waiters .
	// 仍然做一个廉价的健全性检查来检测 WaitGroup 滥用。
	if *statep != state { // note add和wait并发调用导致statep!=state
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	// note 将等待数置为0，并一一唤醒等待者
	// Reset waiters count to 0.
	*statep = 0
	for ; w != 0; w-- { // todo 唤醒阻塞等待者，但这里用w来循环，我是没看懂，而且为啥传的是semap值
		runtime_Semrelease(semap, false, 0)
	}
}

// Done decrements the WaitGroup counter by one.
func (wg *WaitGroup) Done() {
	wg.Add(-1)
}

// Wait blocks until the WaitGroup counter is zero.
func (wg *WaitGroup) Wait() {
	statep, semap := wg.state()
	if race.Enabled {
		_ = *statep // trigger nil deref early
		race.Disable()
	}
	for {
		state := atomic.LoadUint64(statep)
		v := int32(state >> 32) // 计数器值
		w := uint32(state)
		if v == 0 { // note 判断计数器为0，则不用等待
			// Counter is 0, no need to wait.
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(wg))
			}
			return
		}
		// Increment waiters count.
		if atomic.CompareAndSwapUint64(statep, state, state+1) { // note 将等待者+1
			if race.Enabled && w == 0 {
				// Wait must be synchronized with the first Add.
				// Need to model this is as a write to race with the read in Add.
				// As a consequence, can do the write only for the first waiter,
				// otherwise concurrent Waits will race with each other.
				race.Write(unsafe.Pointer(semap))
			}
			runtime_Semacquire(semap) // note 阻塞等待，直到被唤醒
			if *statep != 0 {
				panic("sync: WaitGroup is reused before previous Wait has returned")
			}
			if race.Enabled {
				race.Enable()
				race.Acquire(unsafe.Pointer(wg))
			}
			return
		}
	}
}
