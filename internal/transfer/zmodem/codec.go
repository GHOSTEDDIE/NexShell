// Package zmodem implements the ZMODEM wire protocol in Go. It does not execute rz/sz.
package zmodem

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
)

const (
	ZRQINIT byte = iota
	ZRINIT
	ZSINIT
	ZACK
	ZFILE
	ZSKIP
	ZNAK
	ZABORT
	ZFIN
	ZRPOS
	ZDATA
	ZEOF
	ZFERR
	ZCRC
)
const (
	zdle  byte = 0x18
	ZCRCE byte = 'h'
	ZCRCG byte = 'i'
	ZCRCQ byte = 'j'
	ZCRCW byte = 'k'
)

type Header struct {
	Type  byte
	Data  [4]byte
	CRC32 bool
}

func (h Header) Position() uint32 { return binary.LittleEndian.Uint32(h.Data[:]) }
func Position(kind byte, pos uint32) Header {
	h := Header{Type: kind}
	binary.LittleEndian.PutUint32(h.Data[:], pos)
	return h
}

type Codec struct {
	Reader *bufio.Reader
	Writer io.Writer
}

func New(r io.Reader, w io.Writer) *Codec {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}
	return &Codec{br, w}
}
func CRC16(b []byte) uint16 {
	var crc uint16
	for _, v := range b {
		crc ^= uint16(v) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
func (c *Codec) WriteHeader(h Header) error {
	raw := append([]byte{h.Type}, h.Data[:]...)
	crc := CRC16(raw)
	raw = append(raw, byte(crc>>8), byte(crc))
	b := []byte{'*', '*', zdle, 'B'}
	b = hex.AppendEncode(b, raw)
	b = append(b, '\r', '\n')
	if h.Type != ZFIN && h.Type != ZACK {
		b = append(b, 0x11)
	}
	_, err := c.Writer.Write(b)
	return err
}
func (c *Codec) raw() (byte, error) {
	for {
		b, e := c.Reader.ReadByte()
		if e != nil {
			return 0, e
		}
		if b != 0x11 && b != 0x13 {
			return b, nil
		}
	}
}
func (c *Codec) escaped() (byte, error) {
	b, e := c.raw()
	if e != nil {
		return 0, e
	}
	if b != zdle {
		return b, nil
	}
	v, e := c.raw()
	if e != nil {
		return 0, e
	}
	switch v {
	case 'l':
		return 0x7f, nil
	case 'm':
		return 0xff, nil
	case zdle:
		return 0, errors.New("传输已取消")
	}
	if v&0x60 == 0x40 {
		return v ^ 0x40, nil
	}
	return 0, fmt.Errorf("invalid ZDLE escape %02x", v)
}
func (c *Codec) ReadHeader() (Header, error) {
	var h Header
	for skipped := 0; skipped < 65536; skipped++ {
		b, e := c.raw()
		if e != nil {
			return h, e
		}
		if b == zdle {
			if next, _ := c.Reader.Peek(4); len(next) == 4 && next[0] == zdle && next[1] == zdle && next[2] == zdle && next[3] == zdle {
				return h, errors.New("传输已取消")
			}
		}
		if b != '*' {
			continue
		}
		for {
			b, e = c.raw()
			if e != nil {
				return h, e
			}
			if b != '*' {
				break
			}
		}
		if b != zdle {
			continue
		}
		kind, e := c.raw()
		if e != nil {
			return h, e
		}
		var raw []byte
		switch kind {
		case 'B':
			encoded := make([]byte, 14)
			if _, e = io.ReadFull(c.Reader, encoded); e != nil {
				return h, e
			}
			raw = make([]byte, 7)
			if _, e = hex.Decode(raw, encoded); e != nil {
				return h, e
			}
		case 'A', 'C':
			n := 7
			if kind == 'C' {
				n = 9
				h.CRC32 = true
			}
			raw = make([]byte, n)
			for i := range raw {
				raw[i], e = c.escaped()
				if e != nil {
					return h, e
				}
			}
		default:
			continue
		}
		if h.CRC32 {
			if crc32.ChecksumIEEE(raw[:5]) != binary.LittleEndian.Uint32(raw[5:]) {
				return h, errors.New("header CRC32 mismatch")
			}
		} else if CRC16(raw[:5]) != binary.BigEndian.Uint16(raw[5:]) {
			return h, errors.New("header CRC16 mismatch")
		}
		h.Type = raw[0]
		copy(h.Data[:], raw[1:5])
		if kind == 'B' {
			b, e := c.Reader.ReadByte()
			if e != nil {
				return h, e
			}
			if b == '\r' {
				b, e = c.Reader.ReadByte()
				if e != nil {
					return h, e
				}
				if b&0x7f != '\n' {
					_ = c.Reader.UnreadByte()
				}
			} else {
				_ = c.Reader.UnreadByte()
			}
		}
		return h, nil
	}
	return h, errors.New("ZMODEM header not found")
}
func escape(b []byte) []byte {
	out := make([]byte, 0, len(b))
	for _, v := range b {
		switch {
		case v == 0x7f:
			out = append(out, zdle, 'l')
		case v == 0xff:
			out = append(out, zdle, 'm')
		case v&0x60 == 0:
			out = append(out, zdle, v^0x40)
		default:
			out = append(out, v)
		}
	}
	return out
}
func (c *Codec) WriteData(data []byte, end byte, useCRC32 bool) error {
	checked := append(append([]byte{}, data...), end)
	b := escape(data)
	b = append(b, zdle, end)
	var checksum []byte
	if useCRC32 {
		checksum = make([]byte, 4)
		binary.LittleEndian.PutUint32(checksum, crc32.ChecksumIEEE(checked))
	} else {
		crc := CRC16(checked)
		checksum = []byte{byte(crc >> 8), byte(crc)}
	}
	b = append(b, escape(checksum)...)
	if end == ZCRCW {
		b = append(b, 0x11)
	}
	_, e := c.Writer.Write(b)
	return e
}
func (c *Codec) ReadData(useCRC32 bool) ([]byte, byte, error) {
	data := make([]byte, 0, 8192)
	for len(data) <= 1024*1024 {
		b, e := c.raw()
		if e != nil {
			return nil, 0, e
		}
		if b != zdle {
			data = append(data, b)
			continue
		}
		v, e := c.raw()
		if e != nil {
			return nil, 0, e
		}
		if v >= ZCRCE && v <= ZCRCW {
			n := 2
			if useCRC32 {
				n = 4
			}
			check := make([]byte, n)
			for i := range check {
				check[i], e = c.escaped()
				if e != nil {
					return nil, 0, e
				}
			}
			checked := append(append([]byte{}, data...), v)
			if useCRC32 {
				if crc32.ChecksumIEEE(checked) != binary.LittleEndian.Uint32(check) {
					return nil, 0, errors.New("data CRC32 mismatch")
				}
			} else if CRC16(checked) != binary.BigEndian.Uint16(check) {
				return nil, 0, errors.New("data CRC16 mismatch")
			}
			return data, v, nil
		}
		switch v {
		case 'l':
			data = append(data, 0x7f)
		case 'm':
			data = append(data, 0xff)
		case zdle:
			return nil, 0, errors.New("传输已取消")
		default:
			if v&0x60 != 0x40 {
				return nil, 0, errors.New("invalid data escape")
			}
			data = append(data, v^0x40)
		}
	}
	return nil, 0, errors.New("ZMODEM data packet too large")
}
func (c *Codec) Cancel() {
	_, _ = c.Writer.Write([]byte{zdle, zdle, zdle, zdle, zdle, zdle, zdle, zdle, 8, 8, 8, 8, 8, 8, 8, 8})
}
