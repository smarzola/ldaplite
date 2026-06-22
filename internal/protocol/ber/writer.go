package ber

import "encoding/binary"

const (
	ClassUniversal       byte = 0x00
	ClassApplication     byte = 0x40
	ClassContextSpecific byte = 0x80

	Constructed byte = 0x20
)

const (
	TagBoolean    byte = 0x01
	TagInteger    byte = 0x02
	TagOctet      byte = 0x04
	TagNull       byte = 0x05
	TagEnumerated byte = 0x0a
	TagSequence   byte = 0x10
	TagSet        byte = 0x11
)

func TLV(tag byte, value []byte) []byte {
	out := make([]byte, 0, 1+lengthSize(len(value))+len(value))
	out = append(out, tag)
	out = appendLength(out, len(value))
	out = append(out, value...)
	return out
}

func Sequence(children ...[]byte) []byte {
	return TLV(ClassUniversal|Constructed|TagSequence, concat(children...))
}

func Set(children ...[]byte) []byte {
	return TLV(ClassUniversal|Constructed|TagSet, concat(children...))
}

func Integer(value int) []byte {
	return TLV(ClassUniversal|TagInteger, signedIntegerBytes(value))
}

func Enumerated(value int) []byte {
	return TLV(ClassUniversal|TagEnumerated, signedIntegerBytes(value))
}

func Boolean(value bool) []byte {
	if value {
		return TLV(ClassUniversal|TagBoolean, []byte{0xff})
	}
	return TLV(ClassUniversal|TagBoolean, []byte{0x00})
}

func OctetString(value string) []byte {
	return TLV(ClassUniversal|TagOctet, []byte(value))
}

func Null() []byte {
	return TLV(ClassUniversal|TagNull, nil)
}

func concat(parts ...[]byte) []byte {
	size := 0
	for _, part := range parts {
		size += len(part)
	}
	out := make([]byte, 0, size)
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

func appendLength(out []byte, length int) []byte {
	if length < 0x80 {
		return append(out, byte(length))
	}

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(length))
	start := 0
	for start < len(buf)-1 && buf[start] == 0 {
		start++
	}
	lengthBytes := buf[start:]
	out = append(out, 0x80|byte(len(lengthBytes)))
	out = append(out, lengthBytes...)
	return out
}

func lengthSize(length int) int {
	if length < 0x80 {
		return 1
	}
	size := 0
	for n := length; n > 0; n >>= 8 {
		size++
	}
	return 1 + size
}

func signedIntegerBytes(value int) []byte {
	if value == 0 {
		return []byte{0}
	}

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(value))
	start := 0
	for start < len(buf)-1 && buf[start] == 0 && buf[start+1]&0x80 == 0 {
		start++
	}
	return append([]byte(nil), buf[start:]...)
}
