package dnsquery_test

import (
	"encoding/binary"
	"testing"

	"github.com/78tacos/dns-bench/internal/dnsquery"
)

func TestEncodeQueryAndParseA(t *testing.T) {
	payload, err := dnsquery.EncodeQuery(0xABCD, "Example.COM", dnsquery.TypeA)
	if err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint16(payload[0:2]) != 0xABCD {
		t.Fatal("id")
	}
	if binary.BigEndian.Uint16(payload[4:6]) != 1 {
		t.Fatal("qdcount")
	}

	resp, err := dnsquery.EncodeResponse(0xABCD, dnsquery.RCodeNoError, "example.com", []dnsquery.ARecord{
		{Name: "example.com", TTL: 60, IP: "93.184.216.34"},
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := dnsquery.ParseResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ID != 0xABCD || !parsed.QR || parsed.RCode != dnsquery.RCodeNoError {
		t.Fatalf("header %+v", parsed)
	}
	if len(parsed.Answers) != 1 || parsed.Answers[0].IP != "93.184.216.34" {
		t.Fatalf("answers %+v", parsed.Answers)
	}
	if parsed.Answers[0].Name != "example.com" {
		t.Fatalf("owner %q", parsed.Answers[0].Name)
	}
}

func TestParseNXDOMAIN(t *testing.T) {
	resp, err := dnsquery.EncodeResponse(1, dnsquery.RCodeNXDOMAIN, "nope.invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := dnsquery.ParseResponse(resp)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.RCode != dnsquery.RCodeNXDOMAIN {
		t.Fatalf("rcode %v", parsed.RCode)
	}
	if len(parsed.Answers) != 0 {
		t.Fatalf("answers %v", parsed.Answers)
	}
}

func TestParseCompressedName(t *testing.T) {
	// Hand-built message: question example.com, answer with pointer 0xC00C.
	q, err := dnsquery.EncodeQuery(7, "example.com", dnsquery.TypeA)
	if err != nil {
		t.Fatal(err)
	}
	// Turn query into a response with one compressed A record.
	binary.BigEndian.PutUint16(q[2:4], 0x8180) // QR+RD+RA, NOERROR
	binary.BigEndian.PutUint16(q[6:8], 1)      // ANCOUNT
	ans := []byte{
		0xC0, 0x0C, // pointer to qname at offset 12
		0x00, 0x01, // A
		0x00, 0x01, // IN
		0x00, 0x00, 0x00, 0x3C, // TTL 60
		0x00, 0x04, // rdlen
		10, 20, 30, 40,
	}
	msg := append(q, ans...)
	parsed, err := dnsquery.ParseResponse(msg)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.Answers) != 1 || parsed.Answers[0].IP != "10.20.30.40" {
		t.Fatalf("answers %+v", parsed.Answers)
	}
	if parsed.Answers[0].Name != "example.com" {
		t.Fatalf("compressed owner %q", parsed.Answers[0].Name)
	}
}

func TestEncodeQueryRejectsEmpty(t *testing.T) {
	if _, err := dnsquery.EncodeQuery(1, "", dnsquery.TypeA); err == nil {
		t.Fatal("expected error")
	}
	if _, err := dnsquery.EncodeQuery(1, "bad..name", dnsquery.TypeA); err == nil {
		t.Fatal("expected empty label error")
	}
}

func TestParseResponseShort(t *testing.T) {
	if _, err := dnsquery.ParseResponse([]byte{1, 2, 3}); err == nil {
		t.Fatal("expected short error")
	}
}

func TestRCodeString(t *testing.T) {
	if dnsquery.RCodeNXDOMAIN.String() != "NXDOMAIN" {
		t.Fatal(dnsquery.RCodeNXDOMAIN.String())
	}
	if dnsquery.RCode(99).String() != "RCODE99" {
		t.Fatal(dnsquery.RCode(99).String())
	}
}

func TestPointerLoop(t *testing.T) {
	msg := make([]byte, 14)
	binary.BigEndian.PutUint16(msg[4:6], 1) // QDCOUNT
	msg[12] = 0xC0
	msg[13] = 12 // pointer to itself
	if _, err := dnsquery.ParseResponse(msg); err == nil {
		t.Fatal("expected loop error")
	}
}
