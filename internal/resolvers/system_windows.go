//go:build windows

package resolvers

import (
	"fmt"
	"syscall"
	"unsafe"
)

// GetNetworkParams (iphlpapi) ERROR_BUFFER_OVERFLOW.
const errorBufferOverflow = 111

// ipAddrString matches Windows IP_ADDR_STRING.
type ipAddrString struct {
	Next      *ipAddrString
	IpAddress [16]byte
	IpMask    [16]byte
	Context   uint32
}

// fixedInfo matches Windows FIXED_INFO (MAX_HOSTNAME_LEN=128, MAX_SCOPE_ID_LEN=256).
type fixedInfo struct {
	HostName         [132]byte
	DomainName       [132]byte
	CurrentDnsServer *ipAddrString
	DnsServerList    ipAddrString
	NodeType         uint32
	ScopeId          [260]byte
	EnableRouting    uint32
	EnableProxy      uint32
	EnableDns        uint32
}

var (
	iphlpapi             = syscall.NewLazyDLL("iphlpapi.dll")
	procGetNetworkParams = iphlpapi.NewProc("GetNetworkParams")
)

// System returns OS-configured IPv4 resolvers via GetNetworkParams.
func System() ([]Resolver, error) {
	ips, err := windowsDNSServers()
	if err != nil {
		return nil, err
	}
	return fromNameserverIPs(ips), nil
}

func windowsDNSServers() ([]string, error) {
	if err := procGetNetworkParams.Find(); err != nil {
		return nil, fmt.Errorf("GetNetworkParams: %w", err)
	}

	buf := make([]byte, 1)
	size := uint32(len(buf))
	r1, _, callErr := procGetNetworkParams.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r1 != 0 && r1 != errorBufferOverflow {
		return nil, fmt.Errorf("GetNetworkParams: %w", callErr)
	}
	if size == 0 {
		return nil, fmt.Errorf("GetNetworkParams: empty buffer size")
	}

	buf = make([]byte, size)
	r1, _, callErr = procGetNetworkParams.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r1 != 0 {
		return nil, fmt.Errorf("GetNetworkParams: %w", callErr)
	}

	info := (*fixedInfo)(unsafe.Pointer(&buf[0]))
	var ips []string
	for p := &info.DnsServerList; p != nil; p = p.Next {
		s := cString(p.IpAddress[:])
		if s != "" {
			ips = append(ips, s)
		}
	}
	return ips, nil
}

func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
