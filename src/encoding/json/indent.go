// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import (
	"std/bytes"
)

// Compact appends to dst the JSON-encoded src with
// insignificant space characters elided.
func Compact(dst *bytes.Buffer, src []byte) error {
	return compact(dst, src, false)
}

func compact(dst *bytes.Buffer, src []byte, escape bool) error {
	origLen := dst.Len()
	scan := newScanner()
	defer freeScanner(scan)
	start := 0
	for i, c := range src {
		if escape && (c == '<' || c == '>' || c == '&') {
			if start < i {
				dst.Write(src[start:i])
			}
			dst.WriteString(`\u00`)
			dst.WriteByte(hex[c>>4])
			dst.WriteByte(hex[c&0xF])
			start = i + 1
		}
		// Convert U+2028 and U+2029 (E2 80 A8 and E2 80 A9).
		if escape && c == 0xE2 && i+2 < len(src) && src[i+1] == 0x80 && src[i+2]&^1 == 0xA8 {
			if start < i {
				dst.Write(src[start:i])
			}
			dst.WriteString(`\u202`)
			dst.WriteByte(hex[src[i+2]&0xF])
			start = i + 3
		}
		v := scan.step(scan, c)
		if v >= scanSkipSpace {
			if v == scanError {
				break
			}
			if start < i {
				dst.Write(src[start:i])
			}
			start = i + 1
		}
	}
	if scan.eof() == scanError {
		dst.Truncate(origLen)
		return scan.err
	}
	if start < len(src) {
		dst.Write(src[start:])
	}
	return nil
}

// NOTE 对于每个json子元素，需要另起一行，加上前缀和缩进。尼玛,怎么prefix是在缩进之前的...
func newline(dst *bytes.Buffer, prefix, indent string, depth int) {
	dst.WriteByte('\n')
	dst.WriteString(prefix)
	for i := 0; i < depth; i++ {
		dst.WriteString(indent)
	}
}

// Indent appends to dst an indented form of the JSON-encoded src.
// Each element in a JSON object or array begins on a new,
// indented line beginning with prefix followed by one or more
// copies of indent according to the indentation nesting.
// The data appended to dst does not begin with the prefix nor
// any indentation, to make it easier to embed inside other formatted JSON data.
// Although leading space characters (space, tab, carriage return, newline)
// at the beginning of src are dropped, trailing space characters
// at the end of src are preserved and copied to dst.
// For example, if src has no trailing spaces, neither will dst;
// if src ends in a trailing newline, so will dst.

// 还是有点小东西的，对已经marshal好的json再进行操作，这里也是使用状态机来决定要在哪里加上prefix这些信息，蛮叼的
// 详细注释在👇🏻，反正大概原理就是逐个读取src，再结合状态机的状态来使用写入dst或者加上一些prefix啥的
// 这个状态机也蛮牛逼的，根据当前的byte来决定下一个状态是什么，在stateBeginValueOrEmpty、stateBeginValue、stateBeginStringOrEmpty等等方法之间跳来跳去，每个方法都会改变scan.step的值来实现跳到下一个状态
// 想了解更细致的过程的话，debug一下就知道了
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	origLen := dst.Len()
	scan := newScanner() // NOTE 看到蛮多这种操作的，基本都是取pool缓存，再加个defer来放回缓存。取出来和放回去时都要先重置一下字段
	defer freeScanner(scan)
	needIndent := false
	depth := 0 // 嵌套深度，以此来决定indent的次数
	for _, c := range src {
		scan.bytes++
		v := scan.step(scan, c)
		if v == scanSkipSpace {
			continue
		}
		if v == scanError { // 不符合json格式，直接break
			break
		}
		// NOTE 虽然这里把needIndent置为false，导致只有第一个key会加上newline，但是第二个key在下面的case ','也会帮它加上newline的哈
		if needIndent && v != scanEndObject && v != scanEndArray {
			needIndent = false
			depth++                             // needIndent(前面是[或{) 且非]或},则说明是key的开始，需要嵌套，将深度+1
			newline(dst, prefix, indent, depth) // key的开头，补上prefix和indent
		}

		// Emit semantically uninteresting bytes
		// (in particular, punctuation in strings) unmodified.
		if v == scanContinue { // 常规的小垃圾字符，如"、key和val这些，直接写，无需其它操作
			dst.WriteByte(c)
			continue
		}

		// Add spacing around real punctuation.
		// 特殊字符处理
		switch c {
		case '{', '[':
			// delay indent so that empty object and array are formatted as {} and [].
			needIndent = true // 为后面第一个key的缩进做准备
			dst.WriteByte(c)

		case ',': // value结尾，为下一个元素的缩进做准备
			dst.WriteByte(c)
			newline(dst, prefix, indent, depth)

		case ':':
			dst.WriteByte(c)
			dst.WriteByte(' ') // :跟val之间加个空格

		case '}', ']':
			if needIndent { // 处理空对象或空数组的场景
				// suppress indent in empty object/array
				needIndent = false
			} else { // 结束，嵌套减一，但是结束符也需要加上前缀
				depth--
				newline(dst, prefix, indent, depth)
			}
			dst.WriteByte(c)

		default:
			dst.WriteByte(c)
		}
	}
	if scan.eof() == scanError {
		dst.Truncate(origLen)
		return scan.err
	}
	return nil
}
