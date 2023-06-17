// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package errors

import (
	"std/internal/reflectlite"
)

// Unwrap returns the result of calling the Unwrap method on err, if err's
// type contains an Unwrap method returning error.
// Otherwise, Unwrap returns nil.
func Unwrap(err error) error {
	u, ok := err.(interface {
		Unwrap() error
	})
	if !ok {
		return nil
	}
	return u.Unwrap()
}

// Is reports whether any error in err's chain matches target.
//
// The chain consists of err itself followed by the sequence of errors obtained by
// repeatedly calling Unwrap.
//
// An error is considered to match a target if it is equal to that target or if
// it implements a method Is(error) bool such that Is(target) returns true.
//
// An error type might provide an Is method so it can be treated as equivalent
// to an existing error. For example, if MyError defines
//
//	func (m MyError) Is(target error) bool { return target == fs.ErrExist }
//
// then Is(MyError{}, fs.ErrExist) returns true. See syscall.Errno.Is for
// an example in the standard library. An Is method should only shallowly
// compare err and the target and not call Unwrap on either.

// 类型断言是 Go 语言中常用的操作之一，它用于判断一个接口值是否是某个具体类型的值。
//
// 但是，在 Go 语言中，还可以使用类型断言来判断一个接口值是否实现了特定的接口。这种情况下的类型断言被称为“接口断言”。
//
// 接口断言的语法与普通的类型断言类似，只不过需要将断言的类型指定为一个接口类型，并且该接口类型定义了一些必须要实现的方法。这样，当我们对一个接口值进行接口断言时，如果该接口值实现了目标接口中定义的所有必要方法，那么该接口值就可以被认为是实现了目标接口。
//
// 在 Go 语言中，接口断言通常用于检查错误是否包含某个特定的错误类型，或者在编写通用代码时，对接口值进行限制，使其必须实现一些特定的方法。

// err.(interface{ Is(error) bool }) 是 Go 语言类型断言的语法，用于判断一个接口值是否实现了特定的接口。
//

//在这个例子中，我们希望判断 err 是否实现了一个具有 Is(error) bool 方法的接口。
//
//注意，interface{} 表示空接口类型，它可以表示任意类型的值。
//因此，在这里，我们定义了一个匿名的空接口类型，并在其中声明了一个 Is 方法，这个方法接受一个 error 类型的参数并返回一个 bool 类型的值。这样，我们就可以将实现了这个接口的类型和 err 进行比较了。

// note 用来比较两个err是否是同种类型的；如果err实现了Is接口，则直接调用Is来判相等，否则如果实现了Unwrap接口，则调用Unwrap来解包装
// note Unwrap主要用在包装err从而添加err信息属性的场景，比如struct{err,msg}，这时候需要通过Unwrap来解包装得到真正的err
// 通过for来不断解包装，直到拿到真正的err
func Is(err, target error) bool {
	if target == nil {
		return err == target
	}

	isComparable := reflectlite.TypeOf(target).Comparable()
	for {
		if isComparable && err == target {
			return true
		}
		if x, ok := err.(interface{ Is(error) bool }); ok && x.Is(target) {
			return true
		}
		// TODO: consider supporting target.Is(err). This would allow
		// user-definable predicates, but also may allow for coping with sloppy
		// APIs, thereby making it easier to get away with them.
		if err = Unwrap(err); err == nil {
			return false
		}
	}
}

// As finds the first error in err's chain that matches target, and if one is found, sets
// target to that error value and returns true. Otherwise, it returns false.
//
// The chain consists of err itself followed by the sequence of errors obtained by
// repeatedly calling Unwrap.
//
// An error matches target if the error's concrete value is assignable to the value
// pointed to by target, or if the error has a method As(interface{}) bool such that
// As(target) returns true. In the latter case, the As method is responsible for
// setting target.
//
// An error type might provide an As method so it can be treated as if it were a
// different error type.
//
// As panics if target is not a non-nil pointer to either a type that implements
// error, or to any interface type.

// 不断将err解包，直到其类型满足要求后，将其值赋给target，或者是其实现了As接口，则直接调用As方法，然后返回
func As(err error, target any) bool {
	if target == nil {
		panic("errors: target cannot be nil")
	}
	val := reflectlite.ValueOf(target)
	typ := val.Type()
	if typ.Kind() != reflectlite.Ptr || val.IsNil() {
		panic("errors: target must be a non-nil pointer")
	}
	targetType := typ.Elem()
	// 必须实现error接口
	if targetType.Kind() != reflectlite.Interface && !targetType.Implements(errorType) {
		panic("errors: *target must be interface or implement error")
	}
	for err != nil {
		if reflectlite.TypeOf(err).AssignableTo(targetType) { // 能赋值则将err赋值给targetType
			val.Elem().Set(reflectlite.ValueOf(err))
			return true
		}
		if x, ok := err.(interface{ As(any) bool }); ok && x.As(target) { // 实现了As接口则直接调用
			return true
		}
		err = Unwrap(err) // 解包装，继续往下判断
	}
	return false
}

var errorType = reflectlite.TypeOf((*error)(nil)).Elem()
