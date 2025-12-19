# IEC101 Gateway & Simulator

This project implements a Go-based **IEC 60870-5-101 Slave (Gateway)** and a **Master Simulator** for testing purposes. It serves as a proof-of-concept for an EV charging station gateway that reports power usage and accepts power limitation commands via the IEC 101 protocol.

## Features

*   **Slave (Gateway)**:
    *   Simulates an EV station.
    *   Reports "Power Drawn" (Measured Value, Scaled - Type 11).
    *   Accepts "Power Limit" commands (Setpoint Command, Scaled - Type 48).
    *   Responds to Link Reset and General Interrogation (Type 100).
*   **Master (Simulator)**:
    *   Connects to the Slave.
    *   Performs Link Initialization and General Interrogation.
    *   Periodically sends random Power Limit Setpoints to the Slave.
*   **Transport**:
    *   Generic Serial Transport using `go.bug.st/serial`.
    *   Designed to work with physical serial ports or virtual ports (via `socat`).

## Prerequisites

*   **Go**: 1.18 or later.
*   **Socat**: Required for creating virtual serial ports for local testing.
    *   Install on macOS: `brew install socat`
    *   Install on Linux: `sudo apt install socat`

## Usage

### 1. Automated Verification (Recommended)

A helper tool is provided to automate the setup of virtual serial ports and run both the Master and Slave.

```bash
# Build the verification tool
go build -o verify_serial cmd/verify_serial/main.go

# Run the verification
./verify_serial
```

This will output the logs to `verification_go.log` and the console, showing the full interaction sequence.

### 2. Manual Execution

If you prefer to run components manually, follow these steps:

**Step 1: Create Virtual Serial Ports**
Open a terminal and run `socat` to create a pair of connected virtual ports:
```bash
# Creates ./dev/master.sock and ./dev/slave.sock
./socat_simple.sh
```
*Keep this terminal running.*

**Step 2: Start the Slave**
In a new terminal:
```bash
go run cmd/slave/main.go
```

**Step 3: Start the Master**
In a third terminal:
```bash
go run cmd/master/main.go
```

## Project Structure

*   `cmd/slave`: Main entry point for the IEC 101 Slave (Gateway).
*   `cmd/master`: Main entry point for the Master Simulator.
*   `cmd/verify_serial`: automation tool for local verification.
*   `pkg/iec101`: Shared library for IEC 101 Frame (FT1.2) handling and ASDU encoding/decoding.
*   `socat_simple.sh`: Helper script to create virtual serial ports.
