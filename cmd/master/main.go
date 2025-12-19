package main

import (
	"bufio"
	"io"
	"log"
	"math/rand"
	"time"

	"IEC101/pkg/iec101"

	"go.bug.st/serial"
)

var (
	linkAddrLen = 2
	linkAddr    = uint16(1)
	commonAddr  = uint16(1)
)

func main() {
	time.Sleep(1 * time.Second) // Wait for slave to ensure port exists? No, socat makes ports.

	portName := "./dev/master.sock"
	mode := &serial.Mode{
		BaudRate: 9600,
		Parity:   serial.EvenParity,
		DataBits: 8,
		StopBits: serial.OneStopBit,
	}

	log.Printf("Connecting to Slave at %s...", portName)

	port, err := serial.Open(portName, mode)
	if err != nil {
		log.Fatalf("Failed to open serial port %s: %v", portName, err)
	}
	defer port.Close()

	reader := bufio.NewReader(port)

	// 1. Reset Remote Link
	log.Println("Master: Sending Reset Remote Link...")
	sendFixed(port, 0, linkAddr) // FC=0 (Reset), linkAddr

	// Wait for ACK
	waitForACK(reader)

	// 2. General Interrogation
	log.Println("Master: Sending General Interrogation...")
	sendInterrogation(port, linkAddr, commonAddr)

	// 3. Loop
	ticker := time.NewTicker(10 * time.Second)
	msgChan := make(chan *iec101.Frame)
	errorChan := make(chan error)

	go readLoop(reader, msgChan, errorChan)

	for {
		select {
		case <-ticker.C:
			// Send Random Setpoint
			val := int16(20000 + rand.Intn(5000))
			log.Printf("Master: Sending Power Limit Setpoint: %d", val)
			sendSetpoint(port, linkAddr, commonAddr, 2, val) // IOA 2 for Limit

		case frame := <-msgChan:
			handleFrame(frame)

		case err := <-errorChan:
			log.Fatalf("Connection error: %v", err)
		}
	}
}

func readLoop(reader *bufio.Reader, msgChan chan *iec101.Frame, errorChan chan error) {
	for {
		frame, err := iec101.ReadFrame(reader, linkAddrLen)
		if err != nil {
			if err != io.EOF {
				errorChan <- err
				return
			}
			// EOF on serial might just mean no data yet?
			// No, typically Read blocks. EOF means closed/disconnected.
			// Reconnect logic? For now, fatal.
			time.Sleep(100 * time.Millisecond)
			continue
		}
		msgChan <- frame
	}
}

func waitForACK(reader *bufio.Reader) {
	// Simple wait for next frame being ACK (Fixed with C=0 or E5)
	// We read frames until we get it or timeout.
	// For simplicity, just blocking read here.
	frame, err := iec101.ReadFrame(reader, linkAddrLen)
	if err != nil {
		log.Fatalf("Failed to receive ACK: %v", err)
	}
	if frame.IsSingleChar {
		// Single Char E5 (NACK/ACK in some contexts, but usually standard is Fixed)
		// Standard: Reset Link confirmed by Fixed Frame with FC=0 (ACK).
		log.Println("Master: Received Single Char (0xE5) instead of Fixed ACK?")
		// Assume okay?
	} else if frame.IsFixed && (frame.C&0x0F) == 0 {
		log.Println("Master: Received ACK (Link Reset confirmed)")
	} else {
		log.Printf("Master: Received unexpected %v", frame)
	}
}

func handleFrame(frame *iec101.Frame) {
	if frame.IsFixed {
		fc := frame.C & 0x0F
		log.Printf("Master: Received Fixed Frame FC=%d", fc)
		return
	}
	if frame.IsSingleChar {
		log.Printf("Master: Received Single Char 0xE5")
		return
	}

	asdu, err := iec101.DecodeASDU(frame.ASDU)
	if err != nil {
		log.Printf("ASDU Decode Error: %v", err)
		return
	}

	log.Printf("Master: Received ASDU Type: %d (COT: %d)", asdu.TypeID, asdu.COT)

	switch asdu.TypeID {
	case iec101.TypeM_ME_NB_1:
		ioa, val, qds, _ := iec101.ParseSetpointScaled(asdu.Data)
		log.Printf(" -> DATA: IOA=%d, Value=%d, Quality=%d", ioa, val, qds)

	case iec101.TypeC_IC_NA_1:
		if asdu.COT == iec101.CauseActCon {
			log.Println(" -> Interrogation Activation CONFIRMED")
		} else if asdu.COT == iec101.CauseActTerm {
			log.Println(" -> Interrogation COMPLETED")
		}

	case iec101.TypeC_SE_NB_1:
		if asdu.COT == iec101.CauseActCon {
			log.Println(" -> Setpoint Command CONFIRMED")
		}
	}
}

func sendFixed(port serial.Port, funcCode byte, linkAddr uint16) {
	c := 0x40 | (funcCode & 0x0F)
	frame := &iec101.Frame{
		IsFixed: true,
		C:       c,
		A:       linkAddr,
		ALen:    linkAddrLen,
	}
	frame.Encode(port)
}

func sendInterrogation(port serial.Port, linkAddr, commonAddr uint16) {
	payload := iec101.EncodeInterrogation(0, 20)
	sendASDU(port, linkAddr, iec101.TypeC_IC_NA_1, iec101.CauseAct, commonAddr, payload)
}

func sendSetpoint(port serial.Port, linkAddr, commonAddr, ioa uint16, val int16) {
	payload := iec101.EncodeMeasuredValueScaled(ioa, val, 0)
	sendASDU(port, linkAddr, iec101.TypeC_SE_NB_1, iec101.CauseAct, commonAddr, payload)
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
		C:       0x43, // Send Data with Confirmation
		A:       linkAddr,
		ALen:    linkAddrLen,
		ASDU:    payload,
	}
	frame.Encode(port)
}
