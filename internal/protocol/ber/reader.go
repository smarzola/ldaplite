package ber

import "fmt"

const MaxPacketSize = 16 << 20

type Packet struct {
	Tag      byte
	Value    []byte
	Children []Packet
}

func ReadPacket(data []byte) (Packet, int, error) {
	if len(data) < 2 {
		return Packet{}, 0, fmt.Errorf("BER packet too short")
	}

	tag := data[0]
	length, headerLen, err := readLength(data)
	if err != nil {
		return Packet{}, 0, err
	}
	if length > MaxPacketSize {
		return Packet{}, 0, fmt.Errorf("BER packet length %d exceeds limit", length)
	}
	totalLen := headerLen + length
	if totalLen > len(data) {
		return Packet{}, 0, fmt.Errorf("BER packet truncated: need %d bytes, got %d", totalLen, len(data))
	}

	value := data[headerLen:totalLen]
	packet := Packet{Tag: tag, Value: value}
	if tag&Constructed != 0 {
		children, err := ReadPackets(value)
		if err != nil {
			return Packet{}, 0, err
		}
		packet.Children = children
	}
	return packet, totalLen, nil
}

func ReadPackets(data []byte) ([]Packet, error) {
	var packets []Packet
	for len(data) > 0 {
		packet, n, err := ReadPacket(data)
		if err != nil {
			return nil, err
		}
		packets = append(packets, packet)
		data = data[n:]
	}
	return packets, nil
}

func readLength(data []byte) (int, int, error) {
	if len(data) < 2 {
		return 0, 0, fmt.Errorf("BER length missing")
	}

	lengthByte := data[1]
	if lengthByte&0x80 == 0 {
		return int(lengthByte), 2, nil
	}

	lengthBytes := int(lengthByte & 0x7f)
	if lengthBytes == 0 {
		return 0, 0, fmt.Errorf("BER indefinite lengths are not supported")
	}
	if lengthBytes > 4 {
		return 0, 0, fmt.Errorf("BER length uses %d bytes, max 4", lengthBytes)
	}
	if len(data) < 2+lengthBytes {
		return 0, 0, fmt.Errorf("BER long-form length truncated")
	}
	if data[2] == 0 {
		return 0, 0, fmt.Errorf("BER length is not minimally encoded")
	}

	length := 0
	for i := 0; i < lengthBytes; i++ {
		length = (length << 8) | int(data[2+i])
	}
	if length < 0x80 {
		return 0, 0, fmt.Errorf("BER length should use short form")
	}
	return length, 2 + lengthBytes, nil
}

func (p Packet) RequireTag(tag byte) error {
	if p.Tag != tag {
		return fmt.Errorf("BER tag 0x%02x, want 0x%02x", p.Tag, tag)
	}
	return nil
}

func (p Packet) Int() (int, error) {
	if len(p.Value) == 0 {
		return 0, fmt.Errorf("BER integer is empty")
	}
	value := 0
	for _, b := range p.Value {
		value = (value << 8) | int(b)
	}
	return value, nil
}

func (p Packet) String() string {
	return string(p.Value)
}

func (p Packet) Bool() (bool, error) {
	if len(p.Value) != 1 {
		return false, fmt.Errorf("BER boolean length %d, want 1", len(p.Value))
	}
	return p.Value[0] != 0, nil
}
