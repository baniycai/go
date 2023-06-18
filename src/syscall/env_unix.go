// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix || (js && wasm) || plan9

// Unix environment variables.

package syscall

import (
	"runtime"
	"sync"
)

var (
	// envOnce guards initialization by copyenv, which populates env.
	envOnce sync.Once

	// envLock guards env and envs.
	envLock sync.RWMutex

	// env maps from an environment variable to its first occurrence in envs.
	env map[string]int // 存放的是key:index的格式，index是其在envs中的idx

	// envs is provided by the runtime. elements are expected to
	// be of the form "key=value". An empty string means deleted
	// (or a duplicate to be ignored).

	// 运行时提供的环境变量，存放的是key=val的格式
	envs []string = runtime_envs()
)

func runtime_envs() []string // in package runtime

// setenv_c and unsetenv_c are provided by the runtime but are no-ops
// if cgo isn't loaded.
func setenv_c(k, v string)
func unsetenv_c(k string)

func copyenv() {
	env = make(map[string]int)
	for i, s := range envs { // 遍历运行时提供的环境变量[]string
		for j := 0; j < len(s); j++ {
			if s[j] == '=' {
				key := s[:j]
				if _, ok := env[key]; !ok { // 切割得到map[key]index
					env[key] = i // first mention of key
				} else {
					// Clear duplicate keys. This permits Unsetenv to
					// safely delete only the first item without
					// worrying about unshadowing a later one,
					// which might be a security problem.
					envs[i] = "" // 存在重复的key，则将val置空
				}
				break
			}
		}
	}
}

func Unsetenv(key string) error {
	envOnce.Do(copyenv)

	envLock.Lock()
	defer envLock.Unlock()

	if i, ok := env[key]; ok {
		envs[i] = ""
		delete(env, key)
	}
	unsetenv_c(key)
	return nil
}

func Getenv(key string) (value string, found bool) {
	envOnce.Do(copyenv)
	if len(key) == 0 {
		return "", false
	}

	envLock.RLock()
	defer envLock.RUnlock()

	i, ok := env[key] // 找索引
	if !ok {
		return "", false
	}
	s := envs[i] // 找key=val串
	for i := 0; i < len(s); i++ {
		if s[i] == '=' {
			return s[i+1:], true // 切割得到val
		}
	}
	return "", false
}

func Setenv(key, value string) error {
	envOnce.Do(copyenv)
	if len(key) == 0 {
		return EINVAL
	}
	for i := 0; i < len(key); i++ {
		if key[i] == '=' || key[i] == 0 {
			return EINVAL
		}
	}
	// On Plan 9, null is used as a separator, eg in $path.
	if runtime.GOOS != "plan9" {
		for i := 0; i < len(value); i++ {
			if value[i] == 0 {
				return EINVAL
			}
		}
	}

	envLock.Lock()
	defer envLock.Unlock()

	i, ok := env[key]
	kv := key + "=" + value
	if ok {
		envs[i] = kv
	} else {
		i = len(envs)
		envs = append(envs, kv)
	}
	env[key] = i
	setenv_c(key, value)
	return nil
}

func Clearenv() {
	envOnce.Do(copyenv) // prevent copyenv in Getenv/Setenv

	envLock.Lock()
	defer envLock.Unlock()

	for k := range env {
		unsetenv_c(k)
	}
	env = make(map[string]int)
	envs = []string{}
}

func Environ() []string {
	envOnce.Do(copyenv)
	envLock.RLock()
	defer envLock.RUnlock()
	a := make([]string, 0, len(envs))
	for _, env := range envs {
		if env != "" {
			a = append(a, env)
		}
	}
	return a
}
