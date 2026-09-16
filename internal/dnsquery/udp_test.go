package dnsquery_test

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/78tacos/dns-bench/internal/dnsquery"
)

func TestQueryALocalhost(t *testing.T) {
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 512)
		n, addr, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}
		id := binary.BigEndian.Uint16(buf[0:2])
		resp, err := dnsquery.EncodeResponse(id, dnsquery.RCodeNoError, "example.com", []dnsquery.ARecord{
			{Name: "example.com", TTL: 30, IP: "192.0.2.9"},
		})
		if err != nil {
			return
		}
		_, _ = pc.WriteTo(resp, addr)
		_ = n
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res := dnsquery.QueryA(ctx, pc.LocalAddr().String(), "example.com", time.Second)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if res.RCode != dnsquery.RCodeNoError {
		t.Fatalf("rcode %v", res.RCode)
	}
	if len(res.Answers) != 1 || res.Answers[0].IP != "192.0.2.9" {
		t.Fatalf("answers %+v", res.Answers)
	}
	if res.RTT <= 0 {
		t.Fatal("expected rtt")
	}
	<-done
}

func TestQueryATimeout(t *testing.T) {
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()

	ctx := context.Background()
	res := dnsquery.QueryA(ctx, pc.LocalAddr().String(), "example.com", 40*time.Millisecond)
	if res.Err == nil {
		t.Fatal("expected timeout")
	}
}

func TestQueryARejectsBadName(t *testing.T) {
	res := dnsquery.QueryA(context.Background(), "127.0.0.1:1", "", time.Millisecond)
	if res.Err == nil {
		t.Fatal("expected error")
	}
}
