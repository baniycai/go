// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package metrics

import (
	_ "runtime" // depends on the runtime via a linkname'd function
	"unsafe"
)

// Sample captures a single metric sample.
// note 想要抽样的指标名称；初始化好Name后，将Sample传入Read(m []Sample)，它会帮你填充Value的值
type Sample struct {
	// Name is the name of the metric sampled.
	//
	// It must correspond to a name in one of the metric descriptions
	// returned by All.
	Name string

	// Value is the value of the metric sample.
	Value Value
}

// Implemented in the runtime.
func runtime_readMetrics(unsafe.Pointer, int, int)

// Read populates each Value field in the given slice of metric samples.
//
// Desired metrics should be present in the slice with the appropriate name.
// The user of this API is encouraged to re-use the same slice between calls for
// efficiency, but is not required to do so.
//
// Note that re-use has some caveats. Notably, Values should not be read or
// manipulated while a Read with that value is outstanding; that is a data race.
// This property includes pointer-typed Values (for example, Float64Histogram)
// whose underlying storage will be reused by Read when possible. To safely use
// such values in a concurrent setting, all data must be deep-copied.
//
// It is safe to execute multiple Read calls concurrently, but their arguments
// must share no underlying memory. When in doubt, create a new []Sample from
// scratch, which is always safe, though may be inefficient.
//
// Sample values with names not appearing in All will have their Value populated
// as KindBad to indicate that the name is unknown.
// note 读取填充给定的度量样本切片中的每个值字段。所需指标应以适当的名称出现在切片中。
// note 鼓励此 API 的用户在调用之间重复使用相同的切片以提高效率，但不要求这样做。
// 请注意，重用有一些注意事项。值得注意的是，当具有该值的读取未完成时，不应读取或操作该值；那是一场数据竞赛。
// 此属性包括指针类型的值（例如，Float64Histogram），其底层存储将在可能时由 Read 重用。
// note 为了在并发设置中安全地使用这些值，必须深度复制所有数据。同时执行多个 Read 调用是安全的，但它们的参数必须不共享底层内存。
// 如有疑问，请从头开始创建一个新的 []Sample，这始终是安全的，但可能效率低下。名称未出现在 All 中的示例值会将其值填充为 KindBad 以指示名称未知。
func Read(m []Sample) {
	runtime_readMetrics(unsafe.Pointer(&m[0]), len(m), cap(m))
}
