// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build unix

package net

import (
	"context"
	"internal/bytealg"
	"sync"
	"syscall"
)

var onceReadProtocols sync.Once

// readProtocols loads contents of /etc/protocols into protocols map
// for quick access.
// NOTE 读取linux系统的/etc/protocols文件,构造一个[protocol]protocolVal映射写入protocols，将[protocolAlias]protocolVal也写入protocols
func readProtocols() {
	// 该文件包含的是Internet (IP) protocols，内容示例:
	// ip	0	IP		# internet protocol, pseudo protocol number
	//hopopt	0	HOPOPT		# IPv6 Hop-by-Hop Option [RFC1883]
	//icmp	1	ICMP		# internet control message protocol
	//igmp	2	IGMP		# Internet Group Management
	//ggp	3	GGP		# gateway-gateway protocol
	//ipencap	4	IP-ENCAP	# IP encapsulated in IP (officially ``IP'')
	//st	5	ST		# ST datagram mode
	//tcp	6	TCP		# transmission control protocol
	//egp	8	EGP		# exterior gateway protocol
	//igp	9	IGP		# any private interior gateway (Cisco)
	//pup	12	PUP		# PARC universal packet protocol
	//udp	17	UDP		# user datagram protocol
	//hmp	20	HMP		# host monitoring protocol
	//xns-idp	22	XNS-IDP		# Xerox NS IDP
	file, err := open("/etc/protocols")
	if err != nil {
		return
	}
	defer file.close()

	for line, ok := file.readLine(); ok; line, ok = file.readLine() {
		// tcp    6   TCP    # transmission control protocol
		if i := bytealg.IndexByteString(line, '#'); i >= 0 {
			line = line[0:i]
		}
		f := getFields(line)
		if len(f) < 2 {
			continue
		}
		if proto, _, ok := dtoi(f[1]); ok {
			if _, ok := protocols[f[0]]; !ok {
				protocols[f[0]] = proto
			}
			for _, alias := range f[2:] {
				if _, ok := protocols[alias]; !ok {
					protocols[alias] = proto
				}
			}
		}
	}
}

// lookupProtocol looks up IP protocol name in /etc/protocols and
// returns correspondent protocol number.
// NOTE 找出该协议在/etc/protocols文件中的对应protocol number
func lookupProtocol(_ context.Context, name string) (int, error) {
	onceReadProtocols.Do(readProtocols)
	return lookupProtocolMap(name)
}

func (r *Resolver) lookupHost(ctx context.Context, host string) (addrs []string, err error) {
	order := systemConf().hostLookupOrder(r, host)
	if !r.preferGo() && order == hostLookupCgo {
		if addrs, err, ok := cgoLookupHost(ctx, host); ok {
			return addrs, err
		}
		// cgo not available (or netgo); fall back to Go's DNS resolver
		order = hostLookupFilesDNS
	}
	return r.goLookupHostOrder(ctx, host, order)
}

func (r *Resolver) lookupIP(ctx context.Context, network, host string) (addrs []IPAddr, err error) {
	if r.preferGo() {
		return r.goLookupIP(ctx, network, host)
	}
	order := systemConf().hostLookupOrder(r, host)
	if order == hostLookupCgo {
		if addrs, err, ok := cgoLookupIP(ctx, network, host); ok {
			return addrs, err
		}
		// cgo not available (or netgo); fall back to Go's DNS resolver
		order = hostLookupFilesDNS
	}
	ips, _, err := r.goLookupIPCNAMEOrder(ctx, network, host, order)
	return ips, err
}

func (r *Resolver) lookupPort(ctx context.Context, network, service string) (int, error) {
	if !r.preferGo() && systemConf().canUseCgo() {
		if port, err, ok := cgoLookupPort(ctx, network, service); ok {
			if err != nil {
				// Issue 18213: if cgo fails, first check to see whether we
				// have the answer baked-in to the net package.
				if port, err := goLookupPort(network, service); err == nil {
					return port, nil
				}
			}
			return port, err
		}
	}
	return goLookupPort(network, service)
}

func (r *Resolver) lookupCNAME(ctx context.Context, name string) (string, error) {
	if !r.preferGo() && systemConf().canUseCgo() {
		if cname, err, ok := cgoLookupCNAME(ctx, name); ok {
			return cname, err
		}
	}
	return r.goLookupCNAME(ctx, name)
}

func (r *Resolver) lookupSRV(ctx context.Context, service, proto, name string) (string, []*SRV, error) {
	return r.goLookupSRV(ctx, service, proto, name)
}

func (r *Resolver) lookupMX(ctx context.Context, name string) ([]*MX, error) {
	return r.goLookupMX(ctx, name)
}

func (r *Resolver) lookupNS(ctx context.Context, name string) ([]*NS, error) {
	return r.goLookupNS(ctx, name)
}

func (r *Resolver) lookupTXT(ctx context.Context, name string) ([]string, error) {
	return r.goLookupTXT(ctx, name)
}

func (r *Resolver) lookupAddr(ctx context.Context, addr string) ([]string, error) {
	if !r.preferGo() && systemConf().canUseCgo() {
		if ptrs, err, ok := cgoLookupPTR(ctx, addr); ok {
			return ptrs, err
		}
	}
	return r.goLookupPTR(ctx, addr)
}

// concurrentThreadsLimit returns the number of threads we permit to
// run concurrently doing DNS lookups via cgo. A DNS lookup may use a
// file descriptor so we limit this to less than the number of
// permitted open files. On some systems, notably Darwin, if
// getaddrinfo is unable to open a file descriptor it simply returns
// EAI_NONAME rather than a useful error. Limiting the number of
// concurrent getaddrinfo calls to less than the permitted number of
// file descriptors makes that error less likely. We don't bother to
// apply the same limit to DNS lookups run directly from Go, because
// there we will return a meaningful "too many open files" error.
func concurrentThreadsLimit() int {
	var rlim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim); err != nil {
		return 500
	}
	r := int(rlim.Cur)
	if r > 500 {
		r = 500
	} else if r > 30 {
		r -= 30
	}
	return r
}
