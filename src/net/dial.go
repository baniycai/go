// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package net

import (
	"context"
	"std/internal/nettrace"
	"syscall"
	"time"
)

// defaultTCPKeepAlive is a default constant value for TCPKeepAlive times
// See golang.org/issue/31510
const (
	defaultTCPKeepAlive = 15 * time.Second
)

// A Dialer contains options for connecting to an address.
//
// The zero value for each field is equivalent to dialing
// without that option. Dialing with the zero value of Dialer
// is therefore equivalent to just calling the Dial function.
//
// It is safe to call Dialer's methods concurrently.
// NOTE 连接到一个地址时的相关配置选项吧
type Dialer struct {
	// Timeout is the maximum amount of time a dial will wait for
	// a connect to complete. If Deadline is also set, it may fail
	// earlier.
	//
	// The default is no timeout.
	//
	// When using TCP and dialing a host name with multiple IP
	// addresses, the timeout may be divided between them.
	//
	// With or without a timeout, the operating system may impose
	// its own earlier timeout. For instance, TCP timeouts are
	// often around 3 minutes.
	Timeout time.Duration

	// Deadline is the absolute point in time after which dials
	// will fail. If Timeout is set, it may fail earlier.
	// Zero means no deadline, or dependent on the operating system
	// as with the Timeout option.
	Deadline time.Time

	// LocalAddr is the local address to use when dialing an
	// address. The address must be of a compatible type for the
	// network being dialed.
	// If nil, a local address is automatically chosen.
	LocalAddr Addr

	// DualStack previously enabled RFC 6555 Fast Fallback
	// support, also known as "Happy Eyeballs", in which IPv4 is
	// tried soon if IPv6 appears to be misconfigured and
	// hanging.
	//
	// Deprecated: Fast Fallback is enabled by default. To
	// disable, set FallbackDelay to a negative value.
	DualStack bool

	// FallbackDelay specifies the length of time to wait before
	// spawning a RFC 6555 Fast Fallback connection. That is, this
	// is the amount of time to wait for IPv6 to succeed before
	// assuming that IPv6 is misconfigured and falling back to
	// IPv4.
	//
	// If zero, a default delay of 300ms is used.
	// A negative value disables Fast Fallback support.
	FallbackDelay time.Duration

	// KeepAlive specifies the interval between keep-alive
	// probes for an active network connection.
	// If zero, keep-alive probes are sent with a default value
	// (currently 15 seconds), if supported by the protocol and operating
	// system. Network protocols or operating systems that do
	// not support keep-alives ignore this field.
	// If negative, keep-alive probes are disabled.
	KeepAlive time.Duration

	// Resolver optionally specifies an alternate resolver to use.
	Resolver *Resolver

	// Cancel is an optional channel whose closure indicates that
	// the dial should be canceled. Not all types of dials support
	// cancellation.
	//
	// Deprecated: Use DialContext instead.
	Cancel <-chan struct{}

	// If Control is not nil, it is called after creating the network
	// connection but before actually dialing.
	//
	// Network and address parameters passed to Control method are not
	// necessarily the ones passed to Dial. For example, passing "tcp" to Dial
	// will cause the Control function to be called with "tcp4" or "tcp6".
	Control func(network, address string, c syscall.RawConn) error
}

func (d *Dialer) dualStack() bool { return d.FallbackDelay >= 0 }

func minNonzeroTime(a, b time.Time) time.Time {
	if a.IsZero() {
		return b
	}
	if b.IsZero() || a.Before(b) {
		return a
	}
	return b
}

// deadline returns the earliest of:
//   - now+Timeout
//   - d.Deadline
//   - the context's deadline
//
// Or zero, if none of Timeout, Deadline, or context's deadline is set.
// 从now+Timeout、d.Deadline、context's deadline中选择一个最短的，即最先到期的吧
func (d *Dialer) deadline(ctx context.Context, now time.Time) (earliest time.Time) {
	if d.Timeout != 0 { // including negative, for historical reasons
		earliest = now.Add(d.Timeout)
	}
	if d, ok := ctx.Deadline(); ok {
		earliest = minNonzeroTime(earliest, d)
	}
	return minNonzeroTime(earliest, d.Deadline)
}

func (d *Dialer) resolver() *Resolver {
	if d.Resolver != nil {
		return d.Resolver
	}
	return DefaultResolver
}

// partialDeadline returns the deadline to use for a single address,
// when multiple addresses are pending.
// 当有一堆地址需要顺序发起请求时，进行deadline的分配，addrsRemaining是还剩下几个地址要发送请求
func partialDeadline(now, deadline time.Time, addrsRemaining int) (time.Time, error) {
	if deadline.IsZero() {
		return deadline, nil
	}
	timeRemaining := deadline.Sub(now)
	if timeRemaining <= 0 {
		return time.Time{}, errTimeout
	}
	// Tentatively allocate equal time to each remaining address.
	timeout := timeRemaining / time.Duration(addrsRemaining)
	// If the time per address is too short, steal from the end of the list.
	const saneMinimum = 2 * time.Second
	if timeout < saneMinimum {
		if timeRemaining < saneMinimum {
			timeout = timeRemaining
		} else {
			timeout = saneMinimum
		}
	}
	return now.Add(timeout), nil
}

func (d *Dialer) fallbackDelay() time.Duration {
	if d.FallbackDelay > 0 {
		return d.FallbackDelay
	} else {
		return 300 * time.Millisecond
	}
}

// network估计是tcp:、upd:这种格式;找到network中的协议部分对应的协议号，但这里只处理ip、ip4和ip6
func parseNetwork(ctx context.Context, network string, needsProto bool) (afnet string, proto int, err error) {
	// network为纯地址，不带协议名，而needsProto=true时，返回err
	i := last(network, ':')
	if i < 0 { // no colon
		switch network {
		case "tcp", "tcp4", "tcp6":
		case "udp", "udp4", "udp6":
		case "ip", "ip4", "ip6":
			if needsProto {
				return "", 0, UnknownNetworkError(network)
			}
		case "unix", "unixgram", "unixpacket":
		default:
			return "", 0, UnknownNetworkError(network)
		}
		return network, 0, nil
	}
	afnet = network[:i]
	switch afnet {
	case "ip", "ip4", "ip6":
		protostr := network[i+1:]
		proto, i, ok := dtoi(protostr)
		if !ok || i != len(protostr) {
			proto, err = lookupProtocol(ctx, protostr)
			if err != nil {
				return "", 0, err
			}
		}
		return afnet, proto, nil
	}
	return "", 0, UnknownNetworkError(network)
}

// resolveAddrList resolves addr using hint and returns a list of
// addresses. The result contains at least one address when error is
// nil.
func (r *Resolver) resolveAddrList(ctx context.Context, op, network, addr string, hint Addr) (addrList, error) {
	afnet, _, err := parseNetwork(ctx, network, true) // 找出协议对应的协议号，但这边置为了_？？？
	if err != nil {
		return nil, err
	}
	if op == "dial" && addr == "" {
		return nil, errMissingAddress
	}
	switch afnet {
	case "unix", "unixgram", "unixpacket":
		addr, err := ResolveUnixAddr(afnet, addr)
		if err != nil {
			return nil, err
		}
		if op == "dial" && hint != nil && addr.Network() != hint.Network() {
			return nil, &AddrError{Err: "mismatched local address type", Addr: hint.String()}
		}
		return addrList{addr}, nil
	}
	addrs, err := r.internetAddrList(ctx, afnet, addr)
	if err != nil || op != "dial" || hint == nil {
		return addrs, err
	}
	var (
		tcp      *TCPAddr
		udp      *UDPAddr
		ip       *IPAddr
		wildcard bool
	)
	switch hint := hint.(type) {
	case *TCPAddr:
		tcp = hint
		wildcard = tcp.isWildcard()
	case *UDPAddr:
		udp = hint
		wildcard = udp.isWildcard()
	case *IPAddr:
		ip = hint
		wildcard = ip.isWildcard()
	}
	naddrs := addrs[:0]
	for _, addr := range addrs {
		if addr.Network() != hint.Network() {
			return nil, &AddrError{Err: "mismatched local address type", Addr: hint.String()}
		}
		switch addr := addr.(type) {
		case *TCPAddr:
			if !wildcard && !addr.isWildcard() && !addr.IP.matchAddrFamily(tcp.IP) {
				continue
			}
			naddrs = append(naddrs, addr)
		case *UDPAddr:
			if !wildcard && !addr.isWildcard() && !addr.IP.matchAddrFamily(udp.IP) {
				continue
			}
			naddrs = append(naddrs, addr)
		case *IPAddr:
			if !wildcard && !addr.isWildcard() && !addr.IP.matchAddrFamily(ip.IP) {
				continue
			}
			naddrs = append(naddrs, addr)
		}
	}
	if len(naddrs) == 0 {
		return nil, &AddrError{Err: errNoSuitableAddress.Error(), Addr: hint.String()}
	}
	return naddrs, nil
}

// Dial connects to the address on the named network.
//
// Known networks are "tcp", "tcp4" (IPv4-only), "tcp6" (IPv6-only),
// "udp", "udp4" (IPv4-only), "udp6" (IPv6-only), "ip", "ip4"
// (IPv4-only), "ip6" (IPv6-only), "unix", "unixgram" and
// "unixpacket".
//
// For TCP and UDP networks, the address has the form "host:port".
// The host must be a literal IP address, or a host name that can be
// resolved to IP addresses.
// The port must be a literal port number or a service name.
// If the host is a literal IPv6 address it must be enclosed in square
// brackets, as in "[2001:db8::1]:80" or "[fe80::1%zone]:80".
// The zone specifies the scope of the literal IPv6 address as defined
// in RFC 4007.
// The functions JoinHostPort and SplitHostPort manipulate a pair of
// host and port in this form.
// When using TCP, and the host resolves to multiple IP addresses,
// Dial will try each IP address in order until one succeeds.
//
// Examples:
//
//	Dial("tcp", "golang.org:http")
//	Dial("tcp", "192.0.2.1:http")
//	Dial("tcp", "198.51.100.1:80")
//	Dial("udp", "[2001:db8::1]:domain")
//	Dial("udp", "[fe80::1%lo0]:53")
//	Dial("tcp", ":80")
//
// For IP networks, the network must be "ip", "ip4" or "ip6" followed
// by a colon and a literal protocol number or a protocol name, and
// the address has the form "host". The host must be a literal IP
// address or a literal IPv6 address with zone.
// It depends on each operating system how the operating system
// behaves with a non-well known protocol number such as "0" or "255".
//
// Examples:
//
//	Dial("ip4:1", "192.0.2.1")
//	Dial("ip6:ipv6-icmp", "2001:db8::1")
//	Dial("ip6:58", "fe80::1%lo0")
//
// For TCP, UDP and IP networks, if the host is empty or a literal
// unspecified IP address, as in ":80", "0.0.0.0:80" or "[::]:80" for
// TCP and UDP, "", "0.0.0.0" or "::" for IP, the local system is
// assumed.
//
// For Unix networks, the address must be a file system path.
func Dial(network, address string) (Conn, error) {
	var d Dialer
	return d.Dial(network, address)
}

// DialTimeout acts like Dial but takes a timeout.
//
// The timeout includes name resolution, if required.
// When using TCP, and the host in the address parameter resolves to
// multiple IP addresses, the timeout is spread over each consecutive
// dial, such that each is given an appropriate fraction of the time
// to connect.
//
// See func Dial for a description of the network and address
// parameters.
func DialTimeout(network, address string, timeout time.Duration) (Conn, error) {
	d := Dialer{Timeout: timeout}
	return d.Dial(network, address)
}

// sysDialer contains a Dial's parameters and configuration.
type sysDialer struct {
	Dialer
	network, address string
	testHookDialTCP  func(ctx context.Context, net string, laddr, raddr *TCPAddr) (*TCPConn, error)
}

// Dial connects to the address on the named network.
//
// See func Dial for a description of the network and address
// parameters.
//
// Dial uses context.Background internally; to specify the context, use
// DialContext.
func (d *Dialer) Dial(network, address string) (Conn, error) {
	return d.DialContext(context.Background(), network, address)
}

// DialContext connects to the address on the named network using
// the provided context.
//
// The provided Context must be non-nil. If the context expires before
// the connection is complete, an error is returned. Once successfully
// connected, any expiration of the context will not affect the
// connection.
//
// When using TCP, and the host in the address parameter resolves to multiple
// network addresses, any dial timeout (from d.Timeout or ctx) is spread
// over each consecutive dial, such that each is given an appropriate
// fraction of the time to connect.
// For example, if a host has 4 IP addresses and the timeout is 1 minute,
// the connect to each single address will be given 15 seconds to complete
// before trying the next one.
//
// See func Dial for a description of the network and address
// parameters.
// NOTE 使用ctx的超时控制来对network:address进行拨号建立连接；如果使用network是tcp，并且address解析到多个ip地址，则ctx的超时时间会均分到这多个ip地址上
// 只需要知道拨号最终产生了一个连接到目标address的连接(Conn)即可
// 蛮叼的，dns解析->划分dns地址->竞争建立连接
func (d *Dialer) DialContext(ctx context.Context, network, address string) (Conn, error) {
	if ctx == nil {
		panic("nil context")
	}
	// NOTE 从d的Timeout(计算时要加上+now)、Deadline和ctx.Deadline()中选出一个最早的，如果不是ctx.Deadline()，则重写ctx
	deadline := d.deadline(ctx, time.Now())
	if !deadline.IsZero() {
		if d, ok := ctx.Deadline(); !ok || deadline.Before(d) {
			subCtx, cancel := context.WithDeadline(ctx, deadline)
			defer cancel()
			ctx = subCtx
		}
	}

	if oldCancel := d.Cancel; oldCancel != nil {
		subCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		go func() {
			select {
			case <-oldCancel:
				cancel()
			case <-subCtx.Done():
			}
		}()
		ctx = subCtx
	}

	// 不知道干啥的，dns解析的时候有用到
	// Shadow the nettrace (if any) during resolve so Connect events don't fire for DNS lookups.
	resolveCtx := ctx
	if trace, _ := ctx.Value(nettrace.TraceKey{}).(*nettrace.Trace); trace != nil {
		shadow := *trace
		shadow.ConnectStart = nil
		shadow.ConnectDone = nil
		resolveCtx = context.WithValue(resolveCtx, nettrace.TraceKey{}, &shadow)
	}

	// NOTE dns解析  这段先不看了，分分钟看哭你/我
	addrs, err := d.resolver().resolveAddrList(resolveCtx, "dial", network, address, d.LocalAddr)
	if err != nil {
		return nil, &OpError{Op: "dial", Net: network, Source: nil, Addr: nil, Err: err}
	}

	sd := &sysDialer{
		Dialer:  *d,
		network: network,
		address: address,
	}

	var primaries, fallbacks addrList
	if d.dualStack() && network == "tcp" {
		// 根据解析出来的地址的版本，将其划分为两部分，一部分是ipv4，一部分是ipv6
		primaries, fallbacks = addrs.partition(isIPv4)
	} else {
		primaries = addrs
	}
	// NOTE 两段addrs互相竞争(不过primaries先启动)，谁先成功建立连接就返回
	c, err := sd.dialParallel(ctx, primaries, fallbacks)
	if err != nil {
		return nil, err
	}

	if tc, ok := c.(*TCPConn); ok && d.KeepAlive >= 0 {
		setKeepAlive(tc.fd, true)
		ka := d.KeepAlive
		if d.KeepAlive == 0 {
			ka = defaultTCPKeepAlive
		}
		setKeepAlivePeriod(tc.fd, ka)
		testHookSetKeepAlive(ka)
	}
	return c, nil
}

// dialParallel races two copies of dialSerial, giving the first a
// head start. It returns the first established connection and
// closes the others. Otherwise it returns an error from the first
// primary address.
// TODO  这段代码🐂呀，可以收藏！两套ip地址，primaries率先发起连接建立，fallbacks后发起，谁先建立成功就返回建立的连接，剩下的就不要了！！！
func (sd *sysDialer) dialParallel(ctx context.Context, primaries, fallbacks addrList) (Conn, error) {
	// 独占，直接就primaries发起建立
	if len(fallbacks) == 0 {
		return sd.dialSerial(ctx, primaries)
	}

	returned := make(chan struct{})
	defer close(returned)

	type dialResult struct {
		Conn
		error
		primary bool
		done    bool
	}
	results := make(chan dialResult) // unbuffered   NOTE 阻塞式chan，一个拿到结果，剩下的就不用玩了

	startRacer := func(ctx context.Context, primary bool) {
		ras := primaries
		if !primary {
			ras = fallbacks
		}
		c, err := sd.dialSerial(ctx, ras) // 顺序发起请求，哪个成功就直接返回
		select {
		case results <- dialResult{Conn: c, error: err, primary: primary, done: true}: // 将结果写入，哪怕有err，在fallback的定时任务做控制
		case <-returned:
			if c != nil {
				c.Close()
			}
		}
	}

	var primary, fallback dialResult

	// Start the main racer.
	primaryCtx, primaryCancel := context.WithCancel(ctx)
	defer primaryCancel()
	go startRacer(primaryCtx, true) // 先搞primary的addrList，它只搞一次

	// Start the timer for the fallback racer.
	fallbackTimer := time.NewTimer(sd.fallbackDelay()) // NOTE fallback延时启动
	defer fallbackTimer.Stop()

	for {
		select {
		case <-fallbackTimer.C:
			fallbackCtx, fallbackCancel := context.WithCancel(ctx)
			defer fallbackCancel()
			go startRacer(fallbackCtx, false)

		case res := <-results: // primary并没有处理返回结果，放在这里处理了;通常第一个result应该是primary的
			if res.error == nil {
				return res.Conn, nil
			}
			if res.primary {
				primary = res
			} else {
				fallback = res
			}
			if primary.done && fallback.done { // 两个都执行好了，但结果都是失败，oh gg
				return nil, primary.error
			}
			if res.primary && fallbackTimer.Stop() { // 如果primary执行没成功，且fallback还没开始，则立即开始
				// If we were able to stop the timer, that means it
				// was running (hadn't yet started the fallback), but
				// we just got an error on the primary path, so start
				// the fallback immediately (in 0 nanoseconds).
				fallbackTimer.Reset(0)
			}
		}
	}
}

// dialSerial connects to a list of addresses in sequence, returning
// either the first successful connection, or the first error.
// NOTE 遍历addrList，依次发起连接，ctx的Deadline均分给多个连接(maybe?)；连接基本过程是->创建fd(syscall)->通过fd来建立connect
// 任何一个遇到err或者连接成功，直接返回
func (sd *sysDialer) dialSerial(ctx context.Context, ras addrList) (Conn, error) {
	var firstErr error // The error from the first address is most relevant.

	for i, ra := range ras {
		select {
		case <-ctx.Done():
			return nil, &OpError{Op: "dial", Net: sd.network, Source: sd.LocalAddr, Addr: ra, Err: mapErr(ctx.Err())}
		default:
		}

		dialCtx := ctx
		if deadline, hasDeadline := ctx.Deadline(); hasDeadline {
			partialDeadline, err := partialDeadline(time.Now(), deadline, len(ras)-i)
			if err != nil {
				// Ran out of time.
				if firstErr == nil {
					firstErr = &OpError{Op: "dial", Net: sd.network, Source: sd.LocalAddr, Addr: ra, Err: err}
				}
				break
			}
			// 该地址分配到的时候先总的deadline小，需要更新ctx
			if partialDeadline.Before(deadline) {
				var cancel context.CancelFunc
				dialCtx, cancel = context.WithDeadline(ctx, partialDeadline)
				defer cancel()
			}
		}

		c, err := sd.dialSingle(dialCtx, ra)
		if err == nil {
			return c, nil
		}
		if firstErr == nil {
			firstErr = err
		}
	}

	if firstErr == nil {
		firstErr = &OpError{Op: "dial", Net: sd.network, Source: nil, Addr: nil, Err: errMissingAddress}
	}
	return nil, firstErr
}

// dialSingle attempts to establish and returns a single connection to
// the destination address.
func (sd *sysDialer) dialSingle(ctx context.Context, ra Addr) (c Conn, err error) {
	trace, _ := ctx.Value(nettrace.TraceKey{}).(*nettrace.Trace)
	if trace != nil {
		raStr := ra.String()
		if trace.ConnectStart != nil {
			trace.ConnectStart(sd.network, raStr)
		}
		if trace.ConnectDone != nil {
			defer func() { trace.ConnectDone(sd.network, raStr, err) }()
		}
	}
	la := sd.LocalAddr
	switch ra := ra.(type) {
	case *TCPAddr:
		la, _ := la.(*TCPAddr)
		// NOTE 看不懂呀😭，一堆系统调用，先是不知道怎么搞，得出一个fd(貌似是系统调用)，然后又通过fd去Connect或init啥的(系统调用)，后面又包来包去，最终就得到了一个Conn；ctx是在fd进行Connect的时候用到
		c, err = sd.dialTCP(ctx, la, ra)
	case *UDPAddr:
		la, _ := la.(*UDPAddr)
		c, err = sd.dialUDP(ctx, la, ra)
	case *IPAddr:
		la, _ := la.(*IPAddr)
		c, err = sd.dialIP(ctx, la, ra)
	case *UnixAddr:
		la, _ := la.(*UnixAddr)
		c, err = sd.dialUnix(ctx, la, ra)
	default:
		return nil, &OpError{Op: "dial", Net: sd.network, Source: la, Addr: ra, Err: &AddrError{Err: "unexpected address type", Addr: sd.address}}
	}
	if err != nil {
		return nil, &OpError{Op: "dial", Net: sd.network, Source: la, Addr: ra, Err: err} // c is non-nil interface containing nil pointer
	}
	return c, nil
}

// ListenConfig contains options for listening to an address.
type ListenConfig struct {
	// If Control is not nil, it is called after creating the network
	// connection but before binding it to the operating system.
	//
	// Network and address parameters passed to Control method are not
	// necessarily the ones passed to Listen. For example, passing "tcp" to
	// Listen will cause the Control function to be called with "tcp4" or "tcp6".
	Control func(network, address string, c syscall.RawConn) error

	// KeepAlive specifies the keep-alive period for network
	// connections accepted by this listener.
	// If zero, keep-alives are enabled if supported by the protocol
	// and operating system. Network protocols or operating systems
	// that do not support keep-alives ignore this field.
	// If negative, keep-alives are disabled.
	KeepAlive time.Duration
}

// Listen announces on the local network address.
//
// See func Listen for a description of the network and address
// parameters.
func (lc *ListenConfig) Listen(ctx context.Context, network, address string) (Listener, error) {
	addrs, err := DefaultResolver.resolveAddrList(ctx, "listen", network, address, nil)
	if err != nil {
		return nil, &OpError{Op: "listen", Net: network, Source: nil, Addr: nil, Err: err}
	}
	sl := &sysListener{
		ListenConfig: *lc,
		network:      network,
		address:      address,
	}
	var l Listener
	la := addrs.first(isIPv4)
	switch la := la.(type) {
	case *TCPAddr:
		l, err = sl.listenTCP(ctx, la)
	case *UnixAddr:
		l, err = sl.listenUnix(ctx, la)
	default:
		return nil, &OpError{Op: "listen", Net: sl.network, Source: nil, Addr: la, Err: &AddrError{Err: "unexpected address type", Addr: address}}
	}
	if err != nil {
		return nil, &OpError{Op: "listen", Net: sl.network, Source: nil, Addr: la, Err: err} // l is non-nil interface containing nil pointer
	}
	return l, nil
}

// ListenPacket announces on the local network address.
//
// See func ListenPacket for a description of the network and address
// parameters.
func (lc *ListenConfig) ListenPacket(ctx context.Context, network, address string) (PacketConn, error) {
	addrs, err := DefaultResolver.resolveAddrList(ctx, "listen", network, address, nil)
	if err != nil {
		return nil, &OpError{Op: "listen", Net: network, Source: nil, Addr: nil, Err: err}
	}
	sl := &sysListener{
		ListenConfig: *lc,
		network:      network,
		address:      address,
	}
	var c PacketConn
	la := addrs.first(isIPv4)
	switch la := la.(type) {
	case *UDPAddr:
		c, err = sl.listenUDP(ctx, la)
	case *IPAddr:
		c, err = sl.listenIP(ctx, la)
	case *UnixAddr:
		c, err = sl.listenUnixgram(ctx, la)
	default:
		return nil, &OpError{Op: "listen", Net: sl.network, Source: nil, Addr: la, Err: &AddrError{Err: "unexpected address type", Addr: address}}
	}
	if err != nil {
		return nil, &OpError{Op: "listen", Net: sl.network, Source: nil, Addr: la, Err: err} // c is non-nil interface containing nil pointer
	}
	return c, nil
}

// sysListener contains a Listen's parameters and configuration.
type sysListener struct {
	ListenConfig
	network, address string
}

// Listen announces on the local network address.
//
// The network must be "tcp", "tcp4", "tcp6", "unix" or "unixpacket".
//
// For TCP networks, if the host in the address parameter is empty or
// a literal unspecified IP address, Listen listens on all available
// unicast and anycast IP addresses of the local system.
// To only use IPv4, use network "tcp4".
// The address can use a host name, but this is not recommended,
// because it will create a listener for at most one of the host's IP
// addresses.
// If the port in the address parameter is empty or "0", as in
// "127.0.0.1:" or "[::1]:0", a port number is automatically chosen.
// The Addr method of Listener can be used to discover the chosen
// port.
//
// See func Dial for a description of the network and address
// parameters.
//
// Listen uses context.Background internally; to specify the context, use
// ListenConfig.Listen.
func Listen(network, address string) (Listener, error) {
	var lc ListenConfig
	return lc.Listen(context.Background(), network, address)
}

// ListenPacket announces on the local network address.
//
// The network must be "udp", "udp4", "udp6", "unixgram", or an IP
// transport. The IP transports are "ip", "ip4", or "ip6" followed by
// a colon and a literal protocol number or a protocol name, as in
// "ip:1" or "ip:icmp".
//
// For UDP and IP networks, if the host in the address parameter is
// empty or a literal unspecified IP address, ListenPacket listens on
// all available IP addresses of the local system except multicast IP
// addresses.
// To only use IPv4, use network "udp4" or "ip4:proto".
// The address can use a host name, but this is not recommended,
// because it will create a listener for at most one of the host's IP
// addresses.
// If the port in the address parameter is empty or "0", as in
// "127.0.0.1:" or "[::1]:0", a port number is automatically chosen.
// The LocalAddr method of PacketConn can be used to discover the
// chosen port.
//
// See func Dial for a description of the network and address
// parameters.
//
// ListenPacket uses context.Background internally; to specify the context, use
// ListenConfig.ListenPacket.
func ListenPacket(network, address string) (PacketConn, error) {
	var lc ListenConfig
	return lc.ListenPacket(context.Background(), network, address)
}
