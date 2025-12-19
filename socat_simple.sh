#!/bin/bash
mkdir -p dev
socat -d -d pty,link=dev/master.sock,raw,echo=0 pty,link=dev/slave.sock,raw,echo=0
