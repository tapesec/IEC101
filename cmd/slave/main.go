package main

import (
	"bufio"
	"io"
	"log"
	"time"

	"IEC101/pkg/iec101"

	"go.bug.st/serial"
)

var (
	// Simulated Data
	powerDrawn      int16 = 1234  // Watts
	powerLimitation int16 = 22000 // Watts (max)

	// IEC Settings
	linkAddrLen = 2
	// commonAddr  = uint16(1)
)

func main() {
	portName := "./dev/slave.sock"
	mode := &serial.Mode{
		BaudRate: 9600,
		Parity:   serial.EvenParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		log.Fatalf("Failed to open serial port %s: %v", portName, err)
	}
	defer port.Close()

	log.Printf("IEC101 Slave (Gateway) started on %s", portName)

	// Simulate changing power drawn
	go func() {
		for {
			time.Sleep(5 * time.Second)
			powerDrawn += 10
			if powerDrawn > 1300 {
				powerDrawn = 1200
			}
		}
	}()

	handleConnection(port)
}

func handleConnection(port serial.Port) {
	// Serial port doesn't accept connections like TCP, it's a direct stream.
	// But we still need a reader buffer.
	reader := bufio.NewReader(port)

	log.Println("Slave: Waiting for Link Initialization...")

	for {
		frame, err := iec101.ReadFrame(reader, linkAddrLen)
		if err != nil {
			if err != io.EOF {
				log.Printf("Read error: %v", err)
				// On serial, error might be temporary or fatal.
				// If fatal, we might exit.
				time.Sleep(1 * time.Second)
			}
			continue
		}

		if frame.IsFixed {
			handleFixedFrame(port, frame)
		} else {
			handleVariableFrame(port, frame)
		}
	}
}

func handleFixedFrame(port serial.Port, frame *iec101.Frame) {
	fc := frame.C & 0x0F

	switch fc {
	case 0: // Reset Remote Link
		log.Printf("Received Reset Link Command from %d", frame.A)
		// Respond with ACK (FC=0)
		sendFixed(port, 0, frame.A)
	case 9: // Request Status
		sendFixed(port, 0, frame.A)
	case 10, 11: // Poll Class 1 / Class 2
		// For simulation, sending NACK (0xE5) means "No Data".
		sendSingleChar(port, 0xE5)
	default:
		log.Printf("Unknown Fixed FC: %d", fc)
	}
}

func handleVariableFrame(port serial.Port, frame *iec101.Frame) {
	// Handle ASDU
	asdu, err := iec101.DecodeASDU(frame.ASDU)
	if err != nil {
		log.Printf("ASDU Decode Error: %v", err)
		return
	}

	log.Printf("Received ASDU Type: %d, Cot: %d, CA: %d", asdu.TypeID, asdu.COT, asdu.CommonAddr)

	switch asdu.TypeID {
	case iec101.TypeC_IC_NA_1: // Interrogation
		handleInterrogation(port, frame.A, asdu)
	case iec101.TypeC_SE_NB_1: // Setpoint
		handleSetpoint(port, frame.A, asdu)
	default:
		log.Printf("Unsupported ASDU Type: %d", asdu.TypeID)
	}
}

// Handler functions

func handleInterrogation(port serial.Port, linkAddr uint16, req *iec101.ASDU) {
	ioa, qoi, err := iec101.ParseInterrogation(req.Data)
	if err != nil {
		log.Printf("Interrogation parse error: %v", err)
		return
	}
	log.Printf("Interrogation Request (IOA: %d, QOI: %d)", ioa, qoi)

	// 1. Send ACT_CON
	sendASDU(port, linkAddr, iec101.TypeC_IC_NA_1, iec101.CauseActCon, req.CommonAddr, iec101.EncodeInterrogation(ioa, qoi))

	// 2. Send Data (Power Drawn)
	time.Sleep(100 * time.Millisecond) // Simulating processing
	log.Printf("Sending Power Drawn: %d", powerDrawn)
	sendASDU(port, linkAddr, iec101.TypeM_ME_NB_1, iec101.CauseInrogen, req.CommonAddr, iec101.EncodeMeasuredValueScaled(1, powerDrawn, 0))

	// 3. Send Limit (Power Limit)
	time.Sleep(100 * time.Millisecond)
	log.Printf("Sending Power Limit: %d", powerLimitation)
	sendASDU(port, linkAddr, iec101.TypeM_ME_NB_1, iec101.CauseInrogen, req.CommonAddr, iec101.EncodeMeasuredValueScaled(2, powerLimitation, 0))

	// 4. Send ACT_TERM
	time.Sleep(100 * time.Millisecond)
	sendASDU(port, linkAddr, iec101.TypeC_IC_NA_1, iec101.CauseActTerm, req.CommonAddr, iec101.EncodeInterrogation(ioa, qoi))
}

func handleSetpoint(port serial.Port, linkAddr uint16, req *iec101.ASDU) {
	ioa, val, qos, err := iec101.ParseSetpointScaled(req.Data)
	if err != nil {
		log.Printf("Setpoint parse error: %v", err)
		return
	}
	log.Printf("Received Setpoint Command (IOA: %d): %d W (QOS: %d)", ioa, val, qos)

	// Update Internal State
	if ioa == 2 {
		powerLimitation = val
	}

	// Send ACT_CON
	sendASDU(port, linkAddr, iec101.TypeC_SE_NB_1, iec101.CauseActCon, req.CommonAddr, iec101.EncodeMeasuredValueScaled(ioa, val, qos))
}

// Send Helpers

func sendFixed(port serial.Port, funcCode byte, linkAddr uint16) {
	c := funcCode & 0x0F
	frame := &iec101.Frame{
		IsFixed: true,
		C:       c,
		A:       linkAddr,
		ALen:    linkAddrLen,
	}
	if err := frame.Encode(port); err != nil {
		log.Printf("Send Fixed Error: %v", err)
	}
}

func sendSingleChar(port serial.Port, b byte) {
	port.Write([]byte{b})
}

func sendASDU(port serial.Port, linkAddr uint16, typeID byte, cot byte, ca uint16, data []byte) {
	asdu := &iec101.ASDU{
		TypeID:     typeID,
		VSQ:        1,
		COT:        cot,
		CommonAddr: ca,
		Data:       data,
	}
	payload := asdu.Encode()

	frame := &iec101.Frame{
		IsFixed: false,
		C:       8, // User Data
		A:       linkAddr,
		ALen:    linkAddrLen,
		ASDU:    payload,
	}
	if err := frame.Encode(port); err != nil {
		log.Printf("Send ASDU Error: %v", err)
	}
}
