// Copyright (c) 2020 The Meter.io developers
// Distributed under the GNU Lesser General Public License v3.0 software license, see the accompanying

// file LICENSE or <https://www.gnu.org/licenses/lgpl-3.0.html>

// Modified for Tendermint
// Originally Copyright (c) 2013-2014 Conformal Systems LLC.
// https://github.com/conformal/btcd/blob/master/LICENSE

package types

import (
	//"encoding/hex"
	"flag"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	cmn "github.com/Loragon-chain/loragonBFT/libs/common"
)

// NetAddress defines information about a peer on the network
// including its ID, IP address, and port.
type NetAddress struct {
	IP   net.IP `json:"ip"`
	Port uint32 `json:"port"`

	// memoize .String()
	str string `json:"name"`
}

// NewNetAddress returns a new NetAddress using the provided TCP
// address. When testing, other net.Addr (except TCP) will result in
// using 0.0.0.0:0. When normal run, other net.Addr (except TCP) will
// panic.
// TODO: socks proxies?
func NewNetAddress(addr net.Addr) *NetAddress {
	tcpAddr, ok := addr.(*net.TCPAddr)
	if !ok {
		if flag.Lookup("test.v") == nil { // normal run
			cmn.PanicSanity(fmt.Sprintf("Only TCPAddrs are supported. Got: %v", addr))
		} else { // in testing
			netAddr := NewNetAddressFromNetIP(netip.MustParseAddr("0.0.0.0"), 0)
			return netAddr
		}
	}
	ip := tcpAddr.IP
	port := uint32(tcpAddr.Port)
	na := NewNetAddressFromNetIP(netip.MustParseAddr(ip.String()), port)
	return na
}

// NewNetAddressIPPort returns a new NetAddress using the provided IP
// and port number.
func NewNetAddressFromNetIP(ip netip.Addr, port uint32) *NetAddress {
	return &NetAddress{
		IP:   net.IP(ip.AsSlice()),
		Port: port,
	}
}

// Equals reports whether na and other are the same addresses,
// including their ID, IP, and Port.
func (na *NetAddress) Equals(other interface{}) bool {
	if o, ok := other.(*NetAddress); ok {
		return na.String() == o.String()
	}
	return false
}

// Same returns true is na has the same non-empty ID or DialString as other.
func (na *NetAddress) Same(other interface{}) bool {
	if o, ok := other.(*NetAddress); ok {
		if na.DialString() == o.DialString() {
			return true
		}
	}
	return false
}

// String representation: <IP>:<PORT>
func (na *NetAddress) String() string {
	if na.str == "" {
		addrStr := na.DialString()
		na.str = addrStr
	}
	return na.str
}

func (na *NetAddress) DialString() string {
	return net.JoinHostPort(
		na.IP.String(),
		strconv.FormatUint(uint64(na.Port), 10),
	)
}

// Dial calls net.Dial on the address.
func (na *NetAddress) Dial() (net.Conn, error) {
	conn, err := net.Dial("tcp", na.DialString())
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// DialTimeout calls net.DialTimeout on the address.
func (na *NetAddress) DialTimeout(timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", na.DialString(), timeout)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

// Routable returns true if the address is routable.
func (na *NetAddress) Routable() bool {
	// TODO(oga) bitcoind doesn't include RFC3849 here, but should we?
	return na.Valid() && !(na.RFC1918() || na.RFC3927() || na.RFC4862() ||
		na.RFC4193() || na.RFC4843() || na.Local())
}

// For IPv4 these are either a 0 or all bits set address. For IPv6 a zero
// address or one that matches the RFC3849 documentation address format.
func (na *NetAddress) Valid() bool {
	return na.IP != nil && !(na.IP.IsUnspecified() || na.RFC3849() ||
		na.IP.Equal(net.IPv4bcast))
}

// Local returns true if it is a local address.
func (na *NetAddress) Local() bool {
	return na.IP.IsLoopback() || zero4.Contains(na.IP)
}

// ReachabilityTo checks whenever o can be reached from na.
func (na *NetAddress) ReachabilityTo(o *NetAddress) int {
	const (
		Unreachable = 0
		Default     = iota
		Teredo
		Ipv6_weak
		Ipv4
		Ipv6_strong
	)
	if !na.Routable() {
		return Unreachable
	} else if na.RFC4380() {
		if !o.Routable() {
			return Default
		} else if o.RFC4380() {
			return Teredo
		} else if o.IP.To4() != nil {
			return Ipv4
		} else { // ipv6
			return Ipv6_weak
		}
	} else if na.IP.To4() != nil {
		if o.Routable() && o.IP.To4() != nil {
			return Ipv4
		}
		return Default
	} else /* ipv6 */ {
		var tunnelled bool
		// Is our v6 is tunnelled?
		if o.RFC3964() || o.RFC6052() || o.RFC6145() {
			tunnelled = true
		}
		if !o.Routable() {
			return Default
		} else if o.RFC4380() {
			return Teredo
		} else if o.IP.To4() != nil {
			return Ipv4
		} else if tunnelled {
			// only prioritise ipv6 if we aren't tunnelling it.
			return Ipv6_weak
		}
		return Ipv6_strong
	}
}

// RFC1918: IPv4 Private networks (10.0.0.0/8, 192.168.0.0/16, 172.16.0.0/12)
// RFC3849: IPv6 Documentation address  (2001:0DB8::/32)
// RFC3927: IPv4 Autoconfig (169.254.0.0/16)
// RFC3964: IPv6 6to4 (2002::/16)
// RFC4193: IPv6 unique local (FC00::/7)
// RFC4380: IPv6 Teredo tunneling (2001::/32)
// RFC4843: IPv6 ORCHID: (2001:10::/28)
// RFC4862: IPv6 Autoconfig (FE80::/64)
// RFC6052: IPv6 well known prefix (64:FF9B::/96)
// RFC6145: IPv6 IPv4 translated address ::FFFF:0:0:0/96
var rfc1918_10 = net.IPNet{IP: net.ParseIP("10.0.0.0"), Mask: net.CIDRMask(8, 32)}
var rfc1918_192 = net.IPNet{IP: net.ParseIP("192.168.0.0"), Mask: net.CIDRMask(16, 32)}
var rfc1918_172 = net.IPNet{IP: net.ParseIP("172.16.0.0"), Mask: net.CIDRMask(12, 32)}
var rfc3849 = net.IPNet{IP: net.ParseIP("2001:0DB8::"), Mask: net.CIDRMask(32, 128)}
var rfc3927 = net.IPNet{IP: net.ParseIP("169.254.0.0"), Mask: net.CIDRMask(16, 32)}
var rfc3964 = net.IPNet{IP: net.ParseIP("2002::"), Mask: net.CIDRMask(16, 128)}
var rfc4193 = net.IPNet{IP: net.ParseIP("FC00::"), Mask: net.CIDRMask(7, 128)}
var rfc4380 = net.IPNet{IP: net.ParseIP("2001::"), Mask: net.CIDRMask(32, 128)}
var rfc4843 = net.IPNet{IP: net.ParseIP("2001:10::"), Mask: net.CIDRMask(28, 128)}
var rfc4862 = net.IPNet{IP: net.ParseIP("FE80::"), Mask: net.CIDRMask(64, 128)}
var rfc6052 = net.IPNet{IP: net.ParseIP("64:FF9B::"), Mask: net.CIDRMask(96, 128)}
var rfc6145 = net.IPNet{IP: net.ParseIP("::FFFF:0:0:0"), Mask: net.CIDRMask(96, 128)}
var zero4 = net.IPNet{IP: net.ParseIP("0.0.0.0"), Mask: net.CIDRMask(8, 32)}

func (na *NetAddress) RFC1918() bool {
	return rfc1918_10.Contains(na.IP) ||
		rfc1918_192.Contains(na.IP) ||
		rfc1918_172.Contains(na.IP)
}
func (na *NetAddress) RFC3849() bool { return rfc3849.Contains(na.IP) }
func (na *NetAddress) RFC3927() bool { return rfc3927.Contains(na.IP) }
func (na *NetAddress) RFC3964() bool { return rfc3964.Contains(na.IP) }
func (na *NetAddress) RFC4193() bool { return rfc4193.Contains(na.IP) }
func (na *NetAddress) RFC4380() bool { return rfc4380.Contains(na.IP) }
func (na *NetAddress) RFC4843() bool { return rfc4843.Contains(na.IP) }
func (na *NetAddress) RFC4862() bool { return rfc4862.Contains(na.IP) }
func (na *NetAddress) RFC6052() bool { return rfc6052.Contains(na.IP) }
func (na *NetAddress) RFC6145() bool { return rfc6145.Contains(na.IP) }

func removeProtocolIfDefined(addr string) string {
	if strings.Contains(addr, "://") {
		return strings.Split(addr, "://")[1]
	}
	return addr

}
