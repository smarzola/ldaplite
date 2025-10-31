package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"

	"github.com/lor00x/goldap/message"
)

// ReadLDAPMessage reads a single BER-encoded LDAP message from the connection
// LDAP messages are ASN.1 BER encoded with a length prefix
func ReadLDAPMessage(conn net.Conn) (*message.LDAPMessage, error) {
	// Read the BER tag and length
	// BER format: [tag][length][value]
	// For LDAP messages, the outer tag is SEQUENCE (0x30)

	// Read initial bytes to determine message length
	// We need to read the full BER-encoded message
	var buf [4096]byte
	n, err := conn.Read(buf[:])
	if err != nil {
		if err == io.EOF {
			return nil, err
		}
		return nil, fmt.Errorf("failed to read from connection: %w", err)
	}

	if n < 2 {
		return nil, fmt.Errorf("message too short: %d bytes", n)
	}

	// Parse BER length to determine full message size
	messageLen, headerLen := parseBERLength(buf[:n])
	if messageLen < 0 {
		return nil, fmt.Errorf("invalid BER length encoding")
	}

	totalLen := headerLen + messageLen

	// If we haven't read the full message yet, read more
	data := make([]byte, totalLen)
	copy(data, buf[:n])

	if n < totalLen {
		_, err = io.ReadFull(conn, data[n:])
		if err != nil {
			return nil, fmt.Errorf("failed to read full message: %w", err)
		}
	}

	// Decode the LDAP message using goldap
	bytes := message.NewBytes(0, data)
	msg, err := message.ReadLDAPMessage(bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode LDAP message: %w", err)
	}

	return &msg, nil
}

// WriteLDAPMessage writes a BER-encoded LDAP message to the connection
func WriteLDAPMessage(conn net.Conn, msg *message.LDAPMessage) error {
	// Encode the message using goldap
	data, err := msg.Write()
	if err != nil {
		return fmt.Errorf("failed to encode LDAP message: %w", err)
	}

	// Write the encoded message to the connection
	_, err = conn.Write(data.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write to connection: %w", err)
	}

	return nil
}

// parseBERLength parses BER length encoding from a byte slice
// Returns: (content length, header length)
// BER length encoding:
// - Short form: 0xxxxxxx (0-127)
// - Long form: 1xxxxxxx [length bytes]
func parseBERLength(data []byte) (int, int) {
	if len(data) < 2 {
		return -1, 0
	}

	// Skip tag byte (0x30 for SEQUENCE)
	lengthByte := data[1]

	if lengthByte&0x80 == 0 {
		// Short form: length is in the byte itself
		return int(lengthByte), 2
	}

	// Long form: lengthByte & 0x7F = number of length bytes
	numLengthBytes := int(lengthByte & 0x7F)
	if numLengthBytes == 0 || numLengthBytes > 4 {
		return -1, 0
	}

	if len(data) < 2+numLengthBytes {
		return -1, 0
	}

	// Read length bytes (big-endian)
	length := 0
	for i := 0; i < numLengthBytes; i++ {
		length = (length << 8) | int(data[2+i])
	}

	return length, 2 + numLengthBytes
}

// WriteInteger is a helper to write an integer value
func WriteInteger(value int) []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(value))
	return buf
}
