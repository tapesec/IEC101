logfile="sockets/log-$(date +"%s").txt"
master="./dev/master.sock"
slave="./dev/slave.sock"

# Create directory if it doesn't exist
mkdir -p "./dev"
mkdir -p "sockets"

function fileCmd() {
  echo "pty,link=$1,raw,echo=0,user=$USER,group=staff"
}

#socat -d -d pty,raw,echo=0 pty,raw,echo=0
socat -d -d $(fileCmd $master) $(fileCmd $slave)