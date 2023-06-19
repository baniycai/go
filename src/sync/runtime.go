// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sync

import "unsafe"

// defined in package runtime

// Semacquire waits until *s > 0 and then atomically decrements it.
// It is intended as a simple sleep primitive for use by the synchronization
// library and should not be used directly.
// `runtime_Semacquire` 是 Go 语言中 `sync.Mutex` 互斥锁的实现函数之一，它属于 `unsafe` 包。
// 在 Go 语言中，`sync.Mutex` 是一种常见的同步原语，用于保护代码块或者变量的读写操作。
// 当某一协程获取了一个 `sync.Mutex` 锁时，其他协程将会被阻塞，直到这个协程释放锁为止。
// `runtime_Semacquire` 函数的作用就是在获取 `sync.Mutex` 锁时进行信号量的等待和获取，并且保证获取锁的操作是原子性的。
// 具体来说，当一个协程要获取 `sync.Mutex` 锁时，它会调用 `runtime_Semacquire` 函数，
// note 该函数会负责判断当前是否有其他协程已经获取了锁，如果有，则该协程会进入睡眠状态，等待锁的释放；
// 如果没有，则该协程会获取锁并继续执行后续的代码。
// 需要注意的是，由于 `runtime_Semacquire` 函数属于 `unsafe` 包，因此使用它时需要特别小心，避免产生潜在的安全风险。
func runtime_Semacquire(s *uint32)

// SemacquireMutex is like Semacquire, but for profiling contended Mutexes.
// If lifo is true, queue waiter at the head of wait queue.
// skipframes is the number of frames to omit during tracing, counting from
// runtime_SemacquireMutex's caller.
func runtime_SemacquireMutex(s *uint32, lifo bool, skipframes int)

// runtime_Semrelease 方法是 Go 语言运行时库中的一个函数，note 用于释放一个信号量。
//该函数在操作系统线程和调度器之间进行交互，帮助实现协程调度和同步等功能。
// 在 Go 语言中，信号量被广泛用于并发编程中的同步和互斥。在某些情况下，当一个 goroutine 需要访问共享资源时，它需要获取一个信号量。
//如果这个信号量已经被其他 goroutine 持有，则当前 goroutine 会进入阻塞状态，直到信号量被释放为止。
// note runtime_Semrelease 方法就是用于释放这种信号量的。
// note 它使用了一些底层的操作系统原语来实现信号量的正确释放，包括内存屏障和原子操作等。
//在信号量被释放后，其他的 goroutine 可以继续获取它，并且当前的 goroutine 可以继续执行它的任务。

// runtime_Semrelease 方法的具体实现是由 Go 语言运行时库提供的，而这个库是用 Go 语言编写的。因此，我们无法在 Go 语言中直接查看 runtime_Semrelease 的底层实现细节。
//不过，我们可以通过查看 Go 语言运行时库的源代码来了解 runtime_Semrelease 方法的大致实现方式。在 Go 语言源代码库的 src/runtime/sema.go 文件中，可以找到 runtime_Semrelease 方法的定义和注释。
// code......
//从上述代码片段可以看出，runtime_Semrelease 方法会根据传入的信号量指针 s 和一个布尔值 handoff 来执行不同的操作。
//note 如果 handoff 为 false，则会通过原子操作将信号量的值加1，从而释放该信号量。
//note 如果 handoff 为 true，则会调用 semrelease1 方法来进一步处理信号量的释放。
//semrelease1 是内部的函数，它包含了更多底层的实现细节，比如内存屏障、原子操作和调度等。

// Semrelease atomically increments *s and notifies a waiting goroutine
// if one is blocked in Semacquire.
// It is intended as a simple wakeup primitive for use by the synchronization
// library and should not be used directly.
// If handoff is true, pass count directly to the first waiter.
// skipframes is the number of frames to omit during tracing, counting from
// runtime_Semrelease's caller.
// Semrelease 自动递增 *s 并通知等待的 goroutine 如果一个 goroutine 在 Semacquire 中被阻塞。
// 它旨在作为同步库使用的简单唤醒原语，不应直接使用。如果 handoff 为真，则将计数直接传递给第一个等待者。
// skipframes 是跟踪期间要忽略的帧数，从 runtime_Semrelease 的调用者开始计算。
func runtime_Semrelease(s *uint32, handoff bool, skipframes int)

// See runtime/sema.go for documentation.
func runtime_notifyListAdd(l *notifyList) uint32

// See runtime/sema.go for documentation.
func runtime_notifyListWait(l *notifyList, t uint32)

// See runtime/sema.go for documentation.
func runtime_notifyListNotifyAll(l *notifyList)

// See runtime/sema.go for documentation.
func runtime_notifyListNotifyOne(l *notifyList)

// Ensure that sync and runtime agree on size of notifyList.
func runtime_notifyListCheck(size uintptr)
func init() {
	var n notifyList
	runtime_notifyListCheck(unsafe.Sizeof(n))
}

// Active spinning runtime support.
// runtime_canSpin reports whether spinning makes sense at the moment.
func runtime_canSpin(i int) bool

// runtime_doSpin does active spinning.
func runtime_doSpin()

func runtime_nanotime() int64
