// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package poll

import "sync/atomic"

// fdMutex is a specialized synchronization primitive that manages
// lifetime of an fd and serializes access to Read, Write and Close
// methods on FD.
// fdMutex 是一种专门的同步原语，它管理 fd 的生命周期并序列化对 FD 上的 Read、Write 和 Close 方法的访问。
type fdMutex struct {
	state uint64
	// 等待锁的时候是使用下面两个参数之一去等待的，释放锁的时候也是用下面两个参数
	rsema uint32
	wsema uint32
}

// fdMutex.state is organized as follows:
// 1 bit - whether FD is closed, if set all subsequent lock operations will fail.
// 1 bit - lock for read operations.
// 1 bit - lock for write operations.
// 20 bits - total number of references (read+write+misc).
// 20 bits - number of outstanding read waiters.
// 20 bits - number of outstanding write waiters.
// 这个设计也是蛮吊的，通过一个int64的64个bid就可以记录一大堆的状态信息，直接省空间
const (
	mutexClosed  = 1 << 0
	mutexRLock   = 1 << 1
	mutexWLock   = 1 << 2
	mutexRef     = 1 << 3
	mutexRefMask = (1<<20 - 1) << 3 // 11111111111111111111000   23位，最后3位是保留的，前面的20是用来引用计数的
	mutexRWait   = 1 << 23
	mutexRMask   = (1<<20 - 1) << 23 // 20+23=43位
	mutexWWait   = 1 << 43
	mutexWMask   = (1<<20 - 1) << 43 // 20+43=63位
)

const overflowMsg = "too many concurrent operations on a single file or socket (max 1048575)"

// Read operations must do rwlock(true)/rwunlock(true).
//
// Write operations must do rwlock(false)/rwunlock(false).
//
// Misc operations must do incref/decref.
// Misc operations include functions like setsockopt and setDeadline.
// They need to use incref/decref to ensure that they operate on the
// correct fd in presence of a concurrent close call (otherwise fd can
// be closed under their feet).
//
// Close operations must do increfAndClose/decref.

// incref adds a reference to mu.
// It reports whether mu is available for reading or writing.
func (mu *fdMutex) incref() bool {
	for {
		old := atomic.LoadUint64(&mu.state)
		if old&mutexClosed != 0 {
			return false
		}
		new := old + mutexRef
		if new&mutexRefMask == 0 {
			panic(overflowMsg)
		}
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			return true
		}
	}
}

// 当多个 goroutine 竞争同一个文件描述符时，需要一种方式来保证它们能够按照正确的顺序访问该文件描述符。
// 为了实现对文件描述符的安全访问，Go 语言使用了信号量机制来进行同步。
// note 当一个 goroutine 需要访问文件描述符时，它会先获取该文件描述符的锁，然后执行相应的操作。
// 在文件描述符使用完毕后，该 goroutine 再释放该文件描述符的锁，以便其他 goroutine 能够获取该锁并继续访问该文件描述符。
// 在 increfAndClose 方法中，如果发生了错误导致文件描述符无法正常关闭，就需要将引用计数减一以防止泄露。此时，需要释放该文件描述符的锁，以便其他竞争该文件描述符的 goroutine 能够重新尝试关闭该文件描述符。
// 因此，在 fd_mutex.go 文件的 increfAndClose 方法中调用 runtime_Semrelease 方法是为了释放该文件描述符对应的信号量，并且确保其他 goroutine 能够重新尝试关闭该文件描述符。

// 在 Go 语言中，用来保护文件描述符的同步机制具体是通过使用信号量（Semaphore）实现的。Semaphore 是一种通用的同步机制，它可以对共享资源进行计数并控制并发访问。在 Go 语言中，每个文件描述符都有一个与之对应的信号量，用于保证该文件描述符的正确访问。
//Semaphore 的主要特点是可以动态地调整其内部计数器的值，并根据该计数器的值来决定是否允许进程或线程继续执行。在 Go 语言中，可以通过系统调用（如 sem_init、sem_wait 和 sem_post 等函数）来创建和操作 Semaphore。
//在 fd_mutex.go 文件中，对于每个文件描述符，都会创建一个 sync.Mutex 类型的互斥锁和一个 sema 类型的信号量。其中，互斥锁用于保护文件描述符的读写操作，而信号量则用于控制 goroutine 对该文件描述符的访问顺序。

// increfAndClose sets the state of mu to closed.
// It returns false if the file was already closed.
// note 主要是操作文件描述符的信号量的state，增加引用计数，清空读写等待位数，并唤醒所有在等待读写的协程；这里没有使用到锁，所有用的都是原子操作
func (mu *fdMutex) increfAndClose() bool {
	for {
		// note state 存储了文件描述符的引用计数和一些标志位信息，低 32 位存储了当前引用计数值，高 32 位存储了一些状态标志位，如是否已经关闭、是否可以读、是否可以写等等
		old := atomic.LoadUint64(&mu.state)
		if old&mutexClosed != 0 { // 末位bid为1，则为已关闭
			return false
		}
		// Mark as closed and acquire a reference.
		new := (old | mutexClosed) + mutexRef // 置为关闭，且引用+1
		if new&mutexRefMask == 0 {
			panic(overflowMsg)
		}
		// Remove all read and write waiters.
		// note &^= 是 Go 语言中的一种位清除（Bit Clear）操作符，用于将左操作数中对应位置上为 1 且右操作数中对应位置上也为 1 的位设置为 0
		// note 清空读写等待位数
		new &^= mutexRMask | mutexWMask
		// note 更新state
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			// Wake all read and write waiters,
			// they will observe closed flag after wakeup.
			// note 循环唤醒所有在等待读写的协程，直到读写统计数量减为0，
			for old&mutexRMask != 0 {
				old -= mutexRWait
				runtime_Semrelease(&mu.rsema)
			}
			for old&mutexWMask != 0 {
				old -= mutexWWait
				runtime_Semrelease(&mu.wsema)
			}
			return true
		}
	}
}

// decref removes a reference from mu.
// It reports whether there is no remaining reference.
func (mu *fdMutex) decref() bool {
	for {
		old := atomic.LoadUint64(&mu.state)
		if old&mutexRefMask == 0 {
			panic("inconsistent poll.fdMutex")
		}
		new := old - mutexRef
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			return new&(mutexClosed|mutexRefMask) == mutexClosed
		}
	}
}

// lock adds a reference to mu and locks mu.
// It reports whether mu is available for reading or writing.
func (mu *fdMutex) rwlock(read bool) bool {
	var mutexBit, mutexWait, mutexMask uint64
	var mutexSema *uint32
	if read {
		mutexBit = mutexRLock
		mutexWait = mutexRWait
		mutexMask = mutexRMask
		mutexSema = &mu.rsema
	} else {
		mutexBit = mutexWLock
		mutexWait = mutexWWait
		mutexMask = mutexWMask
		mutexSema = &mu.wsema
	}
	for { // 死循环，直到成功抢占
		old := atomic.LoadUint64(&mu.state)
		if old&mutexClosed != 0 {
			return false
		}
		var new uint64
		if old&mutexBit == 0 { // 该锁还没有被抢占
			// Lock is free, acquire it.
			new = (old | mutexBit) + mutexRef // 抢占锁，并增加引用计数
			if new&mutexRefMask == 0 {
				panic(overflowMsg)
			}
		} else {
			// Wait for lock.
			new = old + mutexWait // 锁已被抢，增加等待计数
			if new&mutexMask == 0 {
				panic(overflowMsg)
			}
		}
		if atomic.CompareAndSwapUint64(&mu.state, old, new) { // 修改状态；因为是cas，所以并发问题没影响
			if old&mutexBit == 0 { // 该锁还没有被抢占，在上面的new中已经抢占锁，并增加了引用计数；直接返回true
				return true
			}
			runtime_Semacquire(mutexSema) // note 锁已被抢占，这里估计是阻塞等待获取
			// The signaller has subtracted mutexWait.
		}
	}
}

// unlock removes a reference from mu and unlocks mu.
// It reports whether there is no remaining reference.
func (mu *fdMutex) rwunlock(read bool) bool {
	var mutexBit, mutexWait, mutexMask uint64
	var mutexSema *uint32
	if read {
		mutexBit = mutexRLock
		mutexWait = mutexRWait
		mutexMask = mutexRMask
		mutexSema = &mu.rsema
	} else {
		mutexBit = mutexWLock
		mutexWait = mutexWWait
		mutexMask = mutexWMask
		mutexSema = &mu.wsema
	}
	for {
		old := atomic.LoadUint64(&mu.state)
		if old&mutexBit == 0 || old&mutexRefMask == 0 {
			panic("inconsistent poll.fdMutex")
		}
		// Drop lock, drop reference and wake read waiter if present.
		new := (old &^ mutexBit) - mutexRef // 置空抢锁的标志位，并将引用减1
		if old&mutexMask != 0 {             // 有在等待锁的，则将等待数减1
			new -= mutexWait
		}
		if atomic.CompareAndSwapUint64(&mu.state, old, new) {
			if old&mutexMask != 0 { // 这里应该是唤醒在等待锁的，唤醒一位
				runtime_Semrelease(mutexSema)
			}
			return new&(mutexClosed|mutexRefMask) == mutexClosed
		}
	}
}

// Implemented in runtime package.
func runtime_Semacquire(sema *uint32)
func runtime_Semrelease(sema *uint32)

// incref adds a reference to fd.
// It returns an error when fd cannot be used.
func (fd *FD) incref() error {
	if !fd.fdmu.incref() {
		return errClosing(fd.isFile)
	}
	return nil
}

// decref removes a reference from fd.
// It also closes fd when the state of fd is set to closed and there
// is no remaining reference.
func (fd *FD) decref() error {
	if fd.fdmu.decref() {
		return fd.destroy()
	}
	return nil
}

// readLock adds a reference to fd and locks fd for reading.
// It returns an error when fd cannot be used for reading.
func (fd *FD) readLock() error {
	if !fd.fdmu.rwlock(true) {
		return errClosing(fd.isFile)
	}
	return nil
}

// readUnlock removes a reference from fd and unlocks fd for reading.
// It also closes fd when the state of fd is set to closed and there
// is no remaining reference.
func (fd *FD) readUnlock() {
	if fd.fdmu.rwunlock(true) {
		fd.destroy()
	}
}

// writeLock adds a reference to fd and locks fd for writing.
// It returns an error when fd cannot be used for writing.
func (fd *FD) writeLock() error {
	if !fd.fdmu.rwlock(false) {
		return errClosing(fd.isFile)
	}
	return nil
}

// writeUnlock removes a reference from fd and unlocks fd for writing.
// It also closes fd when the state of fd is set to closed and there
// is no remaining reference.
func (fd *FD) writeUnlock() {
	if fd.fdmu.rwunlock(false) {
		fd.destroy()
	}
}
