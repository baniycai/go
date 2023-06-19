// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package textproto

// 本质上就是个map啦，为了标准化或者说定制化，包装了一层type MIMEHeader
// 然后还有一个CanonicalMIMEHeaderKey()来标准化key，其它就是增删改查了

// MIME（Multipurpose Internet Mail Extensions）是一种Internet标准，note 用于在电子邮件、Web页面和其他应用程序之间传输多媒体数据。
// MIME-style通常指的是MIME类型（MIME type），也称为媒体类型（media type），note 它是一种用来表示数据类型的标识符。
//
// note MIME类型通过在HTTP协议头部中指定，告诉浏览器如何处理服务器返回的响应数据。
// 例如，在Web页面中，如果服务器返回的数据类型是text/html，浏览器就会将数据解释为HTML文档并正确显示。
// 同样，如果返回的是image/png或image/jpeg类型的数据，浏览器就会将其视为图片并将其显示在页面上。
//
// note 除了在HTTP协议中使用，MIME类型还被广泛应用于电子邮件、FTP等应用程序中，以确保数据在不同平台和应用程序之间能够正确地传输和解释。
// 下面是一些常见的MIME类型及其对应的文件类型：
//
// text/html：HTML文档
// text/plain：纯文本文件
// application/json：JSON数据
// application/xml：XML文档
// image/jpeg：JPEG格式的图像文件
// image/png：PNG格式的图像文件
// audio/mpeg：MP3音频文件
// video/mp4：MP4视频文件
// application/pdf：PDF文档
// A MIMEHeader represents a MIME-style header mapping
// keys to sets of values.
type MIMEHeader map[string][]string

// Add adds the key, value pair to the header.
// It appends to any existing values associated with key.
func (h MIMEHeader) Add(key, value string) {
	key = CanonicalMIMEHeaderKey(key) // 标准化
	h[key] = append(h[key], value)
}

// Set sets the header entries associated with key to
// the single element value. It replaces any existing
// values associated with key.
// add是追加，set是新增或替换
func (h MIMEHeader) Set(key, value string) {
	h[CanonicalMIMEHeaderKey(key)] = []string{value}
}

// Get gets the first value associated with the given key.
// It is case insensitive; CanonicalMIMEHeaderKey is used
// to canonicalize the provided key.
// If there are no values associated with the key, Get returns "".
// To use non-canonical keys, access the map directly.
func (h MIMEHeader) Get(key string) string {
	if h == nil {
		return ""
	}
	v := h[CanonicalMIMEHeaderKey(key)]
	if len(v) == 0 {
		return ""
	}
	return v[0]
}

// Values returns all values associated with the given key.
// It is case insensitive; CanonicalMIMEHeaderKey is
// used to canonicalize the provided key. To use non-canonical
// keys, access the map directly.
// The returned slice is not a copy.
func (h MIMEHeader) Values(key string) []string {
	if h == nil {
		return nil
	}
	return h[CanonicalMIMEHeaderKey(key)]
}

// Del deletes the values associated with key.
func (h MIMEHeader) Del(key string) {
	delete(h, CanonicalMIMEHeaderKey(key))
}
