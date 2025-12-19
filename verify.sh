#!/bin/bash
rm -f verification.log
touch verification.log

echo "Starting Socat..." >> verification.log
mkdir -p dev
rm -f dev/*.sock
socat -d -d pty,link=dev/master.sock,raw,echo=0 pty,link=dev/slave.sock,raw,echo=0 >> verification.log 2>&1 &
SOCAT_PID=$!
sleep 2

echo "Starting Slave..." >> verification.log
./slave >> verification.log 2>&1 &
SLAVE_PID=$!
sleep 2

echo "Starting Master..." >> verification.log
./master >> verification.log 2>&1 &
MASTER_PID=$!

sleep 15
echo "Killing processes..." >> verification.log
kill $MASTER_PID
kill $SLAVE_PID
kill $SOCAT_PID
echo "Done." >> verification.log
