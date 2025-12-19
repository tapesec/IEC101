package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	logFile, _ := os.Create("verification_go.log")
	defer logFile.Close()
	mw := io.MultiWriter(os.Stdout, logFile)

	fmt.Fprintln(mw, "Starting Go Verification...")

	// Cleanup
	os.RemoveAll("dev")
	os.Mkdir("dev", 0755)

	// Start Socat
	// socat -d -d pty,link=dev/master.sock,raw,echo=0 pty,link=dev/slave.sock,raw,echo=0
	socatCmd := exec.Command("socat", "-d", "-d", "pty,link=dev/master.sock,raw,echo=0", "pty,link=dev/slave.sock,raw,echo=0")
	socatOut, _ := socatCmd.StderrPipe()
	if err := socatCmd.Start(); err != nil {
		fmt.Fprintf(mw, "Failed to start socat: %v\n", err)
		return
	}
	go streamOutput("SOCAT", socatOut, mw)
	fmt.Fprintf(mw, "Socat started (PID %d)\n", socatCmd.Process.Pid)
	defer func() {
		if socatCmd.Process != nil {
			socatCmd.Process.Kill()
		}
	}()

	time.Sleep(2 * time.Second)

	// Verify sockets exist
	if _, err := os.Stat("dev/master.sock"); os.IsNotExist(err) {
		fmt.Fprintln(mw, "Error: dev/master.sock not created")
	} else {
		fmt.Fprintln(mw, "dev/master.sock exists")
	}

	// Start Slave
	slavePath, _ := filepath.Abs("slave")
	slaveCmd := exec.Command(slavePath)
	slaveOut, _ := slaveCmd.StdoutPipe()
	slaveErr, _ := slaveCmd.StderrPipe()
	if err := slaveCmd.Start(); err != nil {
		fmt.Fprintf(mw, "Failed to start slave: %v\n", err)
		return
	}
	go streamOutput("SLAVE", slaveOut, mw)
	go streamOutput("SLAVE_ERR", slaveErr, mw)
	fmt.Fprintf(mw, "Slave started (PID %d)\n", slaveCmd.Process.Pid)
	defer func() {
		if slaveCmd.Process != nil {
			slaveCmd.Process.Kill()
		}
	}()

	time.Sleep(1 * time.Second)

	// Start Master
	masterPath, _ := filepath.Abs("master")
	masterCmd := exec.Command(masterPath)
	masterOut, _ := masterCmd.StdoutPipe()
	masterErr, _ := masterCmd.StderrPipe()
	if err := masterCmd.Start(); err != nil {
		fmt.Fprintf(mw, "Failed to start master: %v\n", err)
		return
	}
	go streamOutput("MASTER", masterOut, mw)
	go streamOutput("MASTER_ERR", masterErr, mw)
	fmt.Fprintf(mw, "Master started (PID %d)\n", masterCmd.Process.Pid)
	defer func() {
		if masterCmd.Process != nil {
			masterCmd.Process.Kill()
		}
	}()

	time.Sleep(15 * time.Second)
	fmt.Fprintln(mw, "Stopping verification...")
}

func streamOutput(prefix string, r io.Reader, w io.Writer) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		fmt.Fprintf(w, "[%s] %s\n", prefix, scanner.Text())
	}
}
