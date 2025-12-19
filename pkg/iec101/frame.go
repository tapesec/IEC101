package iec101

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	StartCharFixed    = 0x10
	StartCharVariable = 0x68
	EndChar           = 0x16
	SingleCharAck     = 0xE5
)

// Frame represents a generic IEC 101 frame
type Frame struct {
	IsFixed      bool
	IsSingleChar bool
	// Control Field
	C byte
	// Address Field (Link Address)
	A uint16
	// Address Field Length in bytes (1 or 2)
	ALen int

	// ASDU (only for variable frames)
	ASDU []byte
}

// Encode writes the frame to the writer
func (f *Frame) Encode(w io.Writer) error {
	if f.IsSingleChar {
		_, err := w.Write([]byte{SingleCharAck})
		return err
	}
	if f.IsFixed {
		// Fixed length frame: 10 C A CS 16
		sum := int(f.C)
		buf := []byte{StartCharFixed, f.C}

		if f.ALen == 1 {
			buf = append(buf, byte(f.A))
			sum += int(byte(f.A))
		} else {
			buf = append(buf, byte(f.A), byte(f.A>>8))
			sum += int(byte(f.A))
			sum += int(byte(f.A >> 8))
		}

		buf = append(buf, byte(sum%256), EndChar)
		_, err := w.Write(buf)
		return err
	}

	// Variable length frame: 68 L L 68 C A [ASDU] CS 16
	length := 1 + f.ALen + len(f.ASDU)
	if length > 255 {
		return errors.New("frame too large")
	}

	buf := []byte{StartCharVariable, byte(length), byte(length), StartCharVariable, f.C}
	sum := int(f.C)

	if f.ALen == 1 {
		buf = append(buf, byte(f.A))
		sum += int(byte(f.A))
	} else {
		buf = append(buf, byte(f.A), byte(f.A>>8))
		sum += int(byte(f.A))
		sum += int(byte(f.A >> 8))
	}

	for _, b := range f.ASDU {
		buf = append(buf, b)
		sum += int(b)
	}

	buf = append(buf, byte(sum%256), EndChar)
	_, err := w.Write(buf)
	return err
}

// ReadFrame reads a frame from the reader
// linkAddrLen: 1 or 2 bytes
func ReadFrame(r *bufio.Reader, linkAddrLen int) (*Frame, error) {
	start, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	if start == SingleCharAck {
		return &Frame{IsSingleChar: true}, nil
	}

	if start == StartCharFixed {
		// Fixed frame: 10 C A CS 16
		c, err := r.ReadByte()
		if err != nil {
			return nil, err
		}

		var a uint16
		var aBytes []byte
		if linkAddrLen == 1 {
			b, err := r.ReadByte()
			if err != nil {
				return nil, err
			}
			a = uint16(b)
			aBytes = []byte{b}
		} else {
			b := make([]byte, 2)
			if _, err := io.ReadFull(r, b); err != nil {
				return nil, err
			}
			a = binary.LittleEndian.Uint16(b)
			aBytes = b
		}

		cs, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		end, err := r.ReadByte()
		if err != nil {
			return nil, err
		}

		if end != EndChar {
			return nil, fmt.Errorf("invalid end char: %x", end)
		}

		// Checksum
		sum := int(c)
		for _, b := range aBytes {
			sum += int(b)
		}
		if byte(sum%256) != cs {
			return nil, fmt.Errorf("checksum error")
		}

		return &Frame{IsFixed: true, C: c, A: a, ALen: linkAddrLen}, nil
	} else if start == StartCharVariable {
		// Variable frame: 68 L L 68 ...
		l1, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		l2, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if l1 != l2 {
			return nil, fmt.Errorf("length mismatch: %d != %d", l1, l2)
		}

		start2, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if start2 != StartCharVariable {
			return nil, fmt.Errorf("invalid second start char: %x", start2)
		}

		// Read Body (C + A + ASDU)
		body := make([]byte, l1)
		if _, err := io.ReadFull(r, body); err != nil {
			return nil, err
		}

		cs, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		end, err := r.ReadByte()
		if err != nil {
			return nil, err
		}

		if end != EndChar {
			return nil, fmt.Errorf("invalid end char: %x", end)
		}

		// Verify Checksum
		sum := 0
		for _, b := range body {
			sum += int(b)
		}
		if byte(sum%256) != cs {
			return nil, fmt.Errorf("checksum error")
		}

		// Parse Body
		c := body[0]
		var a uint16
		var asdu []byte

		if linkAddrLen == 1 {
			a = uint16(body[1])
			asdu = body[2:]
		} else {
			a = binary.LittleEndian.Uint16(body[1:3])
			asdu = body[3:]
		}

		return &Frame{IsFixed: false, C: c, A: a, ALen: linkAddrLen, ASDU: asdu}, nil
	}

	return nil, fmt.Errorf("unknown start char: %x", start)
}
