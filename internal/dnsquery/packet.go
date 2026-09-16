package dnsquery

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
)

const (
	TypeA   uint16 = 1
	ClassIN uint16 = 1
	flagRD  uint16 = 1 << 8
	flagQR  uint16 = 1 << 15
)

// RCode is a DNS response code.
type RCode uint16

const (
	RCodeNoError  RCode = 0
	RCodeFormErr  RCode = 1
	RCodeServFail RCode = 2
	RCodeNXDOMAIN RCode = 3
	RCodeNotImpl  RCode = 4
	RCodeRefused  RCode = 5
)

func (c RCode) String() string {
	switch c {
	case RCodeNoError:
		return "NOERROR"
	case RCodeFormErr:
		return "FORMERR"
	case RCodeServFail:
		return "SERVFAIL"
	case RCodeNXDOMAIN:
		return "NXDOMAIN"
	case RCodeNotImpl:
		return "NOTIMP"
	case RCodeRefused:
		return "REFUSED"
	default:
		return fmt.Sprintf("RCODE%d", uint16(c))
	}
}

// ARecord is an IPv4 answer.
type ARecord struct {
	Name string
	TTL  uint32
	IP   string
}

// Response is a parsed DNS message.
type Response struct {
	ID      uint16
	QR      bool
	RCode   RCode
	Answers []ARecord
}

// EncodeQuery builds a standard recursive A (or other type) query.
func EncodeQuery(id uint16, name string, qtype uint16) ([]byte, error) {
	encoded, err := encodeName(name)
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 12+len(encoded)+4)
	binary.BigEndian.PutUint16(buf[0:2], id)
	binary.BigEndian.PutUint16(buf[2:4], flagRD)
	binary.BigEndian.PutUint16(buf[4:6], 1)
	copy(buf[12:], encoded)
	off := 12 + len(encoded)
	binary.BigEndian.PutUint16(buf[off:off+2], qtype)
	binary.BigEndian.PutUint16(buf[off+2:off+4], ClassIN)
	return buf, nil
}

func encodeName(name string) ([]byte, error) {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".")
	if name == "" {
		return nil, errors.New("empty qname")
	}
	labels := strings.Split(strings.ToLower(name), ".")
	var b []byte
	for _, lab := range labels {
		if lab == "" {
			return nil, fmt.Errorf("empty label in %q", name)
		}
		if len(lab) > 63 {
			return nil, fmt.Errorf("label too long in %q", name)
		}
		b = append(b, byte(len(lab)))
		b = append(b, lab...)
	}
	b = append(b, 0)
	if len(b) > 255 {
		return nil, fmt.Errorf("qname too long")
	}
	return b, nil
}

// ParseResponse decodes a DNS message, collecting A records.
func ParseResponse(msg []byte) (Response, error) {
	if len(msg) < 12 {
		return Response{}, errors.New("short DNS message")
	}
	var r Response
	r.ID = binary.BigEndian.Uint16(msg[0:2])
	flags := binary.BigEndian.Uint16(msg[2:4])
	r.QR = flags&flagQR != 0
	r.RCode = RCode(flags & 0xF)
	qd := int(binary.BigEndian.Uint16(msg[4:6]))
	an := int(binary.BigEndian.Uint16(msg[6:8]))
	off := 12
	for i := 0; i < qd; i++ {
		_, n, err := readName(msg, off)
		if err != nil {
			return r, err
		}
		off = n + 4
		if off > len(msg) {
			return r, errors.New("truncated question")
		}
	}
	for i := 0; i < an; i++ {
		nm, n, err := readName(msg, off)
		if err != nil {
			return r, err
		}
		off = n
		if off+10 > len(msg) {
			return r, errors.New("truncated RR header")
		}
		typ := binary.BigEndian.Uint16(msg[off : off+2])
		class := binary.BigEndian.Uint16(msg[off+2 : off+4])
		ttl := binary.BigEndian.Uint32(msg[off+4 : off+8])
		rdlen := int(binary.BigEndian.Uint16(msg[off+8 : off+10]))
		off += 10
		if off+rdlen > len(msg) {
			return r, errors.New("truncated rdata")
		}
		if typ == TypeA && class == ClassIN && rdlen == 4 {
			ip := net.IPv4(msg[off], msg[off+1], msg[off+2], msg[off+3]).String()
			r.Answers = append(r.Answers, ARecord{Name: nm, TTL: ttl, IP: ip})
		}
		off += rdlen
	}
	return r, nil
}

func readName(msg []byte, off int) (string, int, error) {
	var labels []string
	jumped := false
	end := off
	hops := 0
	for {
		if off >= len(msg) {
			return "", 0, errors.New("name overflow")
		}
		l := int(msg[off])
		if l == 0 {
			off++
			if !jumped {
				end = off
			}
			break
		}
		if l&0xC0 == 0xC0 {
			if off+1 >= len(msg) {
				return "", 0, errors.New("bad compression pointer")
			}
			ptr := int(l&0x3F)<<8 | int(msg[off+1])
			if ptr >= len(msg) {
				return "", 0, errors.New("compression pointer out of range")
			}
			if !jumped {
				end = off + 2
				jumped = true
			}
			off = ptr
			hops++
			if hops > 10 {
				return "", 0, errors.New("compression pointer loop")
			}
			continue
		}
		if l&0xC0 != 0 {
			return "", 0, errors.New("unsupported label type")
		}
		off++
		if off+l > len(msg) {
			return "", 0, errors.New("label overflow")
		}
		labels = append(labels, string(msg[off:off+l]))
		off += l
		if !jumped {
			end = off
		}
	}
	return strings.Join(labels, "."), end, nil
}

// EncodeResponse is used by tests and the localhost fake server.
func EncodeResponse(id uint16, rcode RCode, qname string, answers []ARecord) ([]byte, error) {
	q, err := encodeName(qname)
	if err != nil {
		return nil, err
	}
	flags := flagQR | flagRD | uint16(rcode)
	buf := make([]byte, 12)
	binary.BigEndian.PutUint16(buf[0:2], id)
	binary.BigEndian.PutUint16(buf[2:4], flags)
	binary.BigEndian.PutUint16(buf[4:6], 1)
	binary.BigEndian.PutUint16(buf[6:8], uint16(len(answers)))
	buf = append(buf, q...)
	var qtail [4]byte
	binary.BigEndian.PutUint16(qtail[0:2], TypeA)
	binary.BigEndian.PutUint16(qtail[2:4], ClassIN)
	buf = append(buf, qtail[:]...)
	for _, a := range answers {
		nm, err := encodeName(a.Name)
		if err != nil {
			nm = q
		}
		buf = append(buf, nm...)
		var rr [10]byte
		binary.BigEndian.PutUint16(rr[0:2], TypeA)
		binary.BigEndian.PutUint16(rr[2:4], ClassIN)
		binary.BigEndian.PutUint32(rr[4:8], a.TTL)
		binary.BigEndian.PutUint16(rr[8:10], 4)
		buf = append(buf, rr[:]...)
		ip := net.ParseIP(a.IP).To4()
		if ip == nil {
			return nil, fmt.Errorf("bad A rdata %q", a.IP)
		}
		buf = append(buf, ip...)
	}
	return buf, nil
}
