// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package unsafe contains operations that step around the type safety of Go programs.

Packages that import unsafe may be non-portable and are not protected by the
Go 1 compatibility guidelines.
*/
package unsafe

// ArbitraryType is here for the purposes of documentation only and is not actually
// part of the unsafe package. It represents the type of an arbitrary Go expression.
type ArbitraryType int

// IntegerType is here for the purposes of documentation only and is not actually
// part of the unsafe package. It represents any arbitrary integer type.
type IntegerType int

// Pointer represents a pointer to an arbitrary type. There are four special operations
// available for type Pointer that are not available for other types:
//   - A pointer value of any type can be converted to a Pointer.
//   - A Pointer can be converted to a pointer value of any type.
//   - A uintptr can be converted to a Pointer.
//   - A Pointer can be converted to a uintptr.
//
// Pointer therefore allows a program to defeat the type system and read and write
// arbitrary memory. It should be used with extreme care.
//
// The following patterns involving Pointer are valid.
// Code not using these patterns is likely to be invalid today
// or to become invalid in the future.
// Even the valid patterns below come with important caveats.
//
// Running "go vet" can help find uses of Pointer that do not conform to these patterns,
// but silence from "go vet" is not a guarantee that the code is valid.
//
// (1) Conversion of a *T1 to Pointer to *T2.
//
// Provided that T2 is no larger than T1 and that the two share an equivalent
// memory layout, this conversion allows reinterpreting data of one type as
// data of another type. An example is the implementation of
// math.Float64bits:
//
//	func Float64bits(f float64) uint64 {
//		return *(*uint64)(unsafe.Pointer(&f))
//	}
//
// (2) Conversion of a Pointer to a uintptr (but not back to Pointer).
//
// Converting a Pointer to a uintptr produces the memory address of the value
// pointed at, as an integer. The usual use for such a uintptr is to print it.
//
// Conversion of a uintptr back to Pointer is not valid in general.
//
// A uintptr is an integer, not a reference.
// Converting a Pointer to a uintptr creates an integer value
// with no pointer semantics.
// Even if a uintptr holds the address of some object,
// the garbage collector will not update that uintptr's value
// if the object moves, nor will that uintptr keep the object
// from being reclaimed.
//
// The remaining patterns enumerate the only valid conversions
// from uintptr to Pointer.
//
// (3) Conversion of a Pointer to a uintptr and back, with arithmetic.
//
// If p points into an allocated object, it can be advanced through the object
// by conversion to uintptr, addition of an offset, and conversion back to Pointer.
//
//	p = unsafe.Pointer(uintptr(p) + offset)
//
// The most common use of this pattern is to access fields in a struct
// or elements of an array:
//
//	// equivalent to f := unsafe.Pointer(&s.f)
//	f := unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + unsafe.Offsetof(s.f))
//
//	// equivalent to e := unsafe.Pointer(&x[i])
//	e := unsafe.Pointer(uintptr(unsafe.Pointer(&x[0])) + i*unsafe.Sizeof(x[0]))
//
// It is valid both to add and to subtract offsets from a pointer in this way.
// It is also valid to use &^ to round pointers, usually for alignment.
// In all cases, the result must continue to point into the original allocated object.
//
// Unlike in C, it is not valid to advance a pointer just beyond the end of
// its original allocation:
//
//	// INVALID: end points outside allocated space.
//	var s thing
//	end = unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + unsafe.Sizeof(s))
//
//	// INVALID: end points outside allocated space.
//	b := make([]byte, n)
//	end = unsafe.Pointer(uintptr(unsafe.Pointer(&b[0])) + uintptr(n))
//
// Note that both conversions must appear in the same expression, with only
// the intervening arithmetic between them:
//
//	// INVALID: uintptr cannot be stored in variable
//	// before conversion back to Pointer.
//	u := uintptr(p)
//	p = unsafe.Pointer(u + offset)
//
// Note that the pointer must point into an allocated object, so it may not be nil.
//
//	// INVALID: conversion of nil pointer
//	u := unsafe.Pointer(nil)
//	p := unsafe.Pointer(uintptr(u) + offset)
//
// (4) Conversion of a Pointer to a uintptr when calling syscall.Syscall.
//
// The Syscall functions in package syscall pass their uintptr arguments directly
// to the operating system, which then may, depending on the details of the call,
// reinterpret some of them as pointers.
// That is, the system call implementation is implicitly converting certain arguments
// back from uintptr to pointer.
//
// If a pointer argument must be converted to uintptr for use as an argument,
// that conversion must appear in the call expression itself:
//
//	syscall.Syscall(SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(p)), uintptr(n))
//
// The compiler handles a Pointer converted to a uintptr in the argument list of
// a call to a function implemented in assembly by arranging that the referenced
// allocated object, if any, is retained and not moved until the call completes,
// even though from the types alone it would appear that the object is no longer
// needed during the call.
//
// For the compiler to recognize this pattern,
// the conversion must appear in the argument list:
//
//	// INVALID: uintptr cannot be stored in variable
//	// before implicit conversion back to Pointer during system call.
//	u := uintptr(unsafe.Pointer(p))
//	syscall.Syscall(SYS_READ, uintptr(fd), u, uintptr(n))
//
// (5) Conversion of the result of reflect.Value.Pointer or reflect.Value.UnsafeAddr
// from uintptr to Pointer.
//
// Package reflect's Value methods named Pointer and UnsafeAddr return type uintptr
// instead of unsafe.Pointer to keep callers from changing the result to an arbitrary
// type without first importing "unsafe". However, this means that the result is
// fragile and must be converted to Pointer immediately after making the call,
// in the same expression:
//
//	p := (*int)(unsafe.Pointer(reflect.ValueOf(new(int)).Pointer()))
//
// As in the cases above, it is invalid to store the result before the conversion:
//
//	// INVALID: uintptr cannot be stored in variable
//	// before conversion back to Pointer.
//	u := reflect.ValueOf(new(int)).Pointer()
//	p := (*int)(unsafe.Pointer(u))
//
// (6) Conversion of a reflect.SliceHeader or reflect.StringHeader Data field to or from Pointer.
//
// As in the previous case, the reflect data structures SliceHeader and StringHeader
// declare the field Data as a uintptr to keep callers from changing the result to
// an arbitrary type without first importing "unsafe". However, this means that
// SliceHeader and StringHeader are only valid when interpreting the content
// of an actual slice or string value.
//
//	var s string
//	hdr := (*reflect.StringHeader)(unsafe.Pointer(&s)) // case 1
//	hdr.Data = uintptr(unsafe.Pointer(p))              // case 6 (this case)
//	hdr.Len = n
//
// In this usage hdr.Data is really an alternate way to refer to the underlying
// pointer in the string header, not a uintptr variable itself.
//
// In general, reflect.SliceHeader and reflect.StringHeader should be used
// only as *reflect.SliceHeader and *reflect.StringHeader pointing at actual
// slices or strings, never as plain structs.
// A program should not declare or allocate variables of these struct types.
//
//	// INVALID: a directly-declared header will not hold Data as a reference.
//	var hdr reflect.StringHeader
//	hdr.Data = uintptr(unsafe.Pointer(p))
//	hdr.Len = n
//	s := *(*string)(unsafe.Pointer(&hdr)) // p possibly already lost

// 在 Golang 中，unsafe.Pointer 是一种特殊的指针类型，note 它可以被用来转换任何指针类型，并且可以用于指针运算和内存操作等。
//
//对于不同类型之间的指针转化，Golang 的设计者们倾向于保持类型的静态检查和类型安全。 note 因此，在进行不同类型之间的指针转换时，必须显式地使用 unsafe.Pointer 进行类型转换，以确保程序的类型安全性。
//这样做可以防止在某些情况下出现类型错误或内存访问问题，从而提高代码的可靠性和安全性。
//
// note 在 Golang 中，每个指针类型都有一个固定的大小和对应的数据类型。这样可以确保编译器在对内存进行访问时能够正确地将其解释为相应的类型，从而提高程序的类型安全性和稳定性。
//但是，在某些情况下，我们需要将指针从一种类型转换为另一种类型。例如，将一个 *int 类型的指针转换为 *float64 类型的指针。
//在这种情况下，如果我们直接进行强制类型转换，则可能会破坏程序的类型安全性，导致访问非法内存或出现其他错误。
//note 因此，可以使用 unsafe.Pointer 类型来作为中转，它可以将任何类型的指针转换为通用的 void* 指针，从而实现不同类型之间的指针转换。

// 使用 unsafe.Pointer 进行指针转换时，需要注意保证转换后的类型与原始类型之间的内存布局是相同的。否则，可能会导致指针指向错误的内存位置或者读写非法内存等问题。
//
// note 对于将 *int 类型的指针转换为 *float64 类型的指针，可以按照以下步骤进行转换：
//将 *int 类型的指针转换为 uintptr 类型的整数值，可以使用 uintptr(unsafe.Pointer(ptr))。
//将上一步得到的 uintptr 整数值转换为 *float64 类型的指针，可以使用 (*float64)(unsafe.Pointer(uintptrValue))。
//需要注意的是，这种转换方式比较危险，因为它可能会违反 Golang 的类型安全机制。如果你不确定自己的代码是否正确，请尽量避免使用 unsafe.Pointer 进行指针转换。

// note 具体来说，使用 unsafe.Pointer 进行指针转换时，会将指针所指向的内存地址视为一个无类型的连续字节序列。
//这些字节序列的解释方式取决于转换后的类型。例如，当将 *int 类型的指针转换为 *float64 类型的指针时，Golang 会假设这些字节序列表示了一个 float64 类型的值，
//从而返回一个正确的 *float64 类型的指针。

// 在 Golang 中，*int 和 uintptr 之间的转换是通过使用 unsafe.Pointer 实现的。在进行指针转换时，unsafe.Pointer 提供了一种通用的方式来将任何指针类型转换为 void* 类型的指针，从而实现不同类型指针之间的转换。
//
//当你需要将一个 *int 类型的指针转换为 uintptr 类型时，可以使用 uintptr(unsafe.Pointer(ptr)) 将指针先转换为 unsafe.Pointer 类型，然后再将其强制转换为 uintptr 类型。
//
//note 这里的关键点是，unsafe.Pointer 可以将一个指针转换为一个通用的空指针，即 void* 类型的指针，并且这个空指针可以被看作一个无类型的连续字节序列。
//因此，在将 *int 类型的指针转换为 uintptr 类型的整数值时，unsafe.Pointer 实际上是将指针所指向的内存地址视为一个无类型的整数值，并将该值转换为 uintptr 类型的整数值。
//
//虽然 *int 和 uintptr 的内存布局是不同的，但是在这种转换中，unsafe.Pointer 使得我们可以将它们视为相同的字节序列，并把它们相互转换。
//同时，需要注意的是，使用 unsafe.Pointer 进行指针转换可能会违反 Golang 的类型安全机制。因此，在进行指针转换时，必须格外小心，并确保对转换后的指针进行正确的类型断言和内存操作。

// 在 Golang 中，*int 和 *float64 指针类型所指向的内存地址的字节序列通常是不同的，因为它们对应着不同的数据类型。
//因此，如果直接将 *int 类型的指针强制转换为 *float64 类型的指针，可能会导致读写错误的内存位置或者出现其他的问题。
//
//在使用 unsafe.Pointer 进行指针转换时，需要确保转换后的类型与原始类型之间的内存布局是相同的。否则，可能会产生不可预期的行为。
//在将 *int 类型的指针转换为 *float64 类型的指针时，可以借助 unsafe 包中的 uintptr 类型来实现。
//
//具体来说，可以先将 *int 类型的指针转换为 uintptr 类型的整数值，然后再将该整数值转换为 *float64 类型的指针。这样做的目的是将指针转换为一个无类型的整数值，然后再将其转换回另一个指针类型。

type Pointer *ArbitraryType // note 不同类型的指针之间的转化都会使用Pointer作为中转，貌似是告诉编译器不要管我这个转化的安全性问题，我能carry住！

// Sizeof takes an expression x of any type and returns the size in bytes
// of a hypothetical variable v as if v was declared via var v = x.
// The size does not include any memory possibly referenced by x.
// For instance, if x is a slice, Sizeof returns the size of the slice
// descriptor, not the size of the memory referenced by the slice.
// For a struct, the size includes any padding introduced by field alignment.
// The return value of Sizeof is a Go constant if the type of the argument x
// does not have variable size.
// (A type has variable size if it is a type parameter or if it is an array
// or struct type with elements of variable size).
func Sizeof(x ArbitraryType) uintptr

// Offsetof returns the offset within the struct of the field represented by x,
// which must be of the form structValue.field. In other words, it returns the
// number of bytes between the start of the struct and the start of the field.
// The return value of Offsetof is a Go constant if the type of the argument x
// does not have variable size.
// (See the description of [Sizeof] for a definition of variable sized types.)
func Offsetof(x ArbitraryType) uintptr

// Alignof takes an expression x of any type and returns the required alignment
// of a hypothetical variable v as if v was declared via var v = x.
// It is the largest value m such that the address of v is always zero mod m.
// It is the same as the value returned by reflect.TypeOf(x).Align().
// As a special case, if a variable s is of struct type and f is a field
// within that struct, then Alignof(s.f) will return the required alignment
// of a field of that type within a struct. This case is the same as the
// value returned by reflect.TypeOf(s.f).FieldAlign().
// The return value of Alignof is a Go constant if the type of the argument
// does not have variable size.
// (See the description of [Sizeof] for a definition of variable sized types.)

// 好的，我们可以通过一个例子来演示 `unsafe.Alignof()` 方法的使用。
//假设有如下的一个结构体类型：
//type Person struct {
//    name string
//    age  int
//}
//那么我们可以使用 `unsafe.Alignof()` 方法来获取该结构体类型在内存中的对齐方式：
//package main
//import (
//    "fmt"
//    "unsafe"
//)
//type Person struct {
//    name string
//    age  int
//}
//func main() {
//    fmt.Println(unsafe.Alignof(Person{}))
//}
//输出结果为：
//8
//可以看到，`Person` 结构体类型的对齐方式为 8 字节。
//note 这是因为在默认情况下，`string` 类型在 64 位架构上需要 8 字节的内存空间，而 `int` 类型也需要 8 字节的内存空间，所以 `Person` 结构体类型的对齐方式就是 8 字节。
//再举一个例子，假设我们要手动分配一段内存空间来存储一个 `int` 类型的变量，我们可以使用以下方式：
//package main
//import (
//    "fmt"
//    "unsafe"
//)
//func main() {
//    var p *int = (*int)(unsafe.Pointer(uintptr(0x100)))
//    fmt.Println(*p)
//}
//
//这里我们首先将一个无符号整数转换为指针类型，然后使用 `unsafe.Pointer()` 函数将其转换为 `unsafe.Pointer` 类型的指针，
//最后将其转换为 `*int` 类型的指针。这样我们就可以通过 `p` 指针来读写内存地址为 `0x100` 的整数了。
//但是需要注意，使用 `unsafe.Pointer()` 函数和强制类型转换来操作指针是非常危险的，
//因为它们可以绕过 Go 语言的类型安全检查机制，容易导致程序出现未定义行为或者崩溃。

// 在计算机内存中，不同的数据类型需要占用不同数量的字节。例如，一个 int 类型通常需要 4 个字节，而一个 double 类型通常需要 8 个字节。
//同时，计算机内存中也有一个基本的读写单位，即字节（byte），也就是最小的可寻址内存单元。
//
//根据计算机硬件的特点，访问内存通常需要按照一定的规则进行，否则会导致性能问题或者出错。
//note 其中之一就是内存对齐（alignment），即确保数据类型所占用的内存地址是某个值的倍数。
//这个值被称为对齐边界（alignment boundary）或者对齐因子（alignment factor），通常是该类型所需的最小内存边界的大小。
//
//note 对齐的主要作用是提高了内存读写的效率。当使用一个未对齐的变量时，CPU 需要进行多次读/写操作才能从内存中取出或存储该变量的值；
//note 而当使用一个已对齐的变量时，CPU 只需要进行一次读/写操作就可以完成该操作。
//此外，在某些架构下，未对齐的内存操作还有可能导致程序崩溃或者产生不正确的结果。
//
//因此，在开发高性能的计算机程序时，对齐是一个非常重要的优化技术。
//note 在 Go 语言中，编译器会自动对变量进行内存对齐，但是在某些特殊情况下，需要手动控制变量的对齐方式,这时候就可以使用 unsafe.Alignof() 方法来获取变量的对齐值，并使用其他的 unsafe 包函数来操作指针和内存。
//但需要注意，使用这些函数需要小心谨慎，因为它们容易导致编程错误或者安全漏洞

// Alignof 采用任何类型的表达式 x 并返回假设变量 v 所需的对齐方式，就好像 v 是通过 var v = x 声明的一样。
// 它是使 v 的地址始终为零模 m 的最大值 m。它与 reflect.TypeOf(x).Align() 返回的值相同。
// 作为一种特殊情况，如果变量 s 是结构类型并且 f 是该结构中的一个字段，则 Alignof(s.f) 将返回结构中该类型字段的所需对齐方式。
// 这种情况与 reflect.TypeOf(s.f).FieldAlign() 返回的值相同。如果参数的类型没有可变大小，Alignof 的返回值是一个 Go 常量。
func Alignof(x ArbitraryType) uintptr

// The function Add adds len to ptr and returns the updated pointer
// Pointer(uintptr(ptr) + uintptr(len)).
// The len argument must be of integer type or an untyped constant.
// A constant len argument must be representable by a value of type int;
// if it is an untyped constant it is given type int.
// The rules for valid uses of Pointer still apply.
func Add(ptr Pointer, len IntegerType) Pointer

// The function Slice returns a slice whose underlying array starts at ptr
// and whose length and capacity are len.
// Slice(ptr, len) is equivalent to
//
//	(*[len]ArbitraryType)(unsafe.Pointer(ptr))[:]
//
// except that, as a special case, if ptr is nil and len is zero,
// Slice returns nil.
//
// The len argument must be of integer type or an untyped constant.
// A constant len argument must be non-negative and representable by a value of type int;
// if it is an untyped constant it is given type int.
// At run time, if len is negative, or if ptr is nil and len is not zero,
// a run-time panic occurs.
func Slice(ptr *ArbitraryType, len IntegerType) []ArbitraryType
