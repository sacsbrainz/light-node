#!/bin/bash

echo "Starting installation..."
rm -rf /usr/local/go
cd ~/
apt update -y && apt install build-essential pkg-config libssl-dev wget git-all -y 

# Check architecture and set appropriate Go download URL
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then
    GO_ARCH="amd64"
elif [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    GO_ARCH="arm64"
else
    echo "Unsupported architecture: $ARCH"
    exit 1
fi

echo "Detected architecture: $ARCH, downloading Go for $GO_ARCH"
rm -rf go1.23.7.linux*
wget https://go.dev/dl/go1.23.7.linux-$GO_ARCH.tar.gz
tar -C /usr/local -xzf go1.23.7.linux-$GO_ARCH.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
echo 'export GOPATH=$HOME/go' >> ~/.bashrc
echo 'export PATH=$PATH:$GOPATH/bin' >> ~/.bashrc
source ~/.bashrc 
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y  
source "$HOME/.cargo/env"

# check if light-node directory exists in the home directory
if [ -d "$HOME/light-node" ]; then
    echo "Directory light-node already exists, skipping cloning..."
else
    echo "Cloning light-node repository..."
    git clone https://github.com/sacsbrainz/light-node.git --depth 1
fi

cd light-node

sleep 1
echo "Private key should not start with 0x, if yours does kindly remove the starting 0x"
# Read input directly from the terminal
read -p "Please paste your private key below and press Enter: " PRIVATE_KEY </dev/tty

echo "Creating .env file with the provided private key..."
cat > .env << EOF
GRPC_URL=34.57.133.111:9090
CONTRACT_ADDR=cosmos1ufs3tlq4umljk0qfe8k5ya0x6hpavn897u2cnf9k0en9jr7qarqqt56709
ZK_PROVER_URL=https://layeredge.mintair.xyz
API_REQUEST_TIMEOUT=100
POINTS_API=https://light-node.layeredge.io
PRIVATE_KEY='$PRIVATE_KEY'
EOF

echo "Starting application..."
go run main.go