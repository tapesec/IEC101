package iec101

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// ASDU definitions
// We assume standard sizes for this simulation:
// COT: 1 byte (Cause) + 1 byte (Originator) IF configured. Let's assume 1 byte COT, 0 byte Orig.
// CA: 2 bytes.
// IOA: 2 bytes.

const (
	TypeM_ME_NB_1 = 11  // Measured Value, Scaled
	TypeC_SE_NB_1 = 48  // Setpoint Command, Scaled
	TypeC_IC_NA_1 = 100 // Interrogation Command

	CauseSpont    = 3  // Spontaneous
	CauseReq      = 5  // Request or Requested
	CauseAct      = 6  // Activation
	CauseActCon   = 7  // Activation Confirmation
	CauseDeact    = 8  // Deactivation
	CauseDeactCon = 9  // Deactivation Confirmation
	CauseActTerm  = 10 // Activation Termination
	CauseInrogen  = 20 // Interrogated by general interrogation
)

type ASDU struct {
	TypeID     byte
	VSQ        byte
	COT        byte
	CommonAddr uint16
	Data       []byte
}

func (a *ASDU) Encode() []byte {
	buf := new(bytes.Buffer)
	buf.WriteByte(a.TypeID)
	buf.WriteByte(a.VSQ)
	buf.WriteByte(a.COT)
	// Originator Address - skipping as per minimal assumption
	// Common Addr
	binary.Write(buf, binary.LittleEndian, a.CommonAddr)
	// Data
	buf.Write(a.Data)
	return buf.Bytes()
}

func DecodeASDU(data []byte) (*ASDU, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("ASDU too short")
	}
	a := &ASDU{}
	a.TypeID = data[0]
	a.VSQ = data[1]
	a.COT = data[2]
	// Assume 1 byte COT. If org addr is present, it's confusing.
	// But common addr starts here.
	a.CommonAddr = binary.LittleEndian.Uint16(data[3:5])
	a.Data = data[5:]
	return a, nil
}

// Helpers for specific types

// DecodeInterrogation parses payload for Type 100
func ParseInterrogation(data []byte) (ioa uint16, qoi byte, err error) {
	if len(data) < 3 {
		return 0, 0, fmt.Errorf("data too short for Interrogation")
	}
	ioa = binary.LittleEndian.Uint16(data[0:2])
	qoi = data[2]
	return
}

// EncodeInterrogation creates payload for Type 100
func EncodeInterrogation(ioa uint16, qoi byte) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, ioa)
	buf.WriteByte(qoi)
	return buf.Bytes()
}

// DecodeSetpointScaled parses payload for Type 48
func ParseSetpointScaled(data []byte) (ioa uint16, val int16, qos byte, err error) {
	// IOA (2) + Value (2) + QOS (1) = 5 bytes
	if len(data) < 5 {
		return 0, 0, 0, fmt.Errorf("data too short for Setpoint")
	}
	ioa = binary.LittleEndian.Uint16(data[0:2])
	val = int16(binary.LittleEndian.Uint16(data[2:4]))
	qos = data[4]
	return
}

// EncodeMeasuredValueScaled creates payload for Type 11
func EncodeMeasuredValueScaled(ioa uint16, val int16, qds byte) []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, ioa)
	binary.Write(buf, binary.LittleEndian, val)
	buf.WriteByte(qds)
	return buf.Bytes()
}
