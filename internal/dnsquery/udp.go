package dnsquery

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"time"
)

// Result is one UDP query attempt.
type Result struct {
	RTT     time.Duration
	RCode   RCode
	Answers []ARecord
	Err     error
}

// QueryA sends a recursive A query to an IPv4 DNS server (host or host:port).
func QueryA(ctx context.Context, server, qname string, timeout time.Duration) Result {
	start := time.Now()
	res := Result{}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	id := randomID()
	payload, err := EncodeQuery(id, qname, TypeA)
	if err != nil {
		res.Err = err
		res.RTT = time.Since(start)
		return res
	}
	addr := canonicalize(server)
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "udp4", addr)
	if err != nil {
		res.Err = err
		res.RTT = time.Since(start)
		return res
	}
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	if ctxd, ok := ctx.Deadline(); ok && ctxd.Before(deadline) {
		deadline = ctxd
	}
	if err := conn.SetDeadline(deadline); err != nil {
		res.Err = err
		res.RTT = time.Since(start)
		return res
	}
	if _, err := conn.Write(payload); err != nil {
		res.Err = err
		res.RTT = time.Since(start)
		return res
	}
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	res.RTT = time.Since(start)
	if err != nil {
		res.Err = err
		return res
	}
	parsed, err := ParseResponse(buf[:n])
	if err != nil {
		res.Err = err
		return res
	}
	if parsed.ID != id {
		res.Err = fmt.Errorf("dns id mismatch")
		return res
	}
	if !parsed.QR {
		res.Err = fmt.Errorf("dns message is not a response")
		return res
	}
	res.RCode = parsed.RCode
	res.Answers = parsed.Answers
	return res
}

func canonicalize(server string) string {
	if _, _, err := net.SplitHostPort(server); err == nil {
		return server
	}
	return net.JoinHostPort(server, "53")
}

func randomID() uint16 {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return uint16(time.Now().UnixNano())
	}
	return binary.BigEndian.Uint16(b[:])
}
