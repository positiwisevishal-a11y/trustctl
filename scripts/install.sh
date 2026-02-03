#!/usr/bin/env bash
set -euo pipefail

# Install layout under /opt/trustctl with correct permissions
DEST=/opt/trustctl
echo "Creating directory layout under $DEST"
sudo mkdir -p $DEST/{bin,plugins,credentials,certs,configs/servers,logs}

echo "Setting ownership to root and permissions"
sudo chown -R root:root $DEST
sudo chmod 700 $DEST/plugins $DEST/certs
sudo chmod 600 $DEST/credentials || true

echo "Copying binary (build first)"
if [ -f ./trustctl ]; then
  sudo install -m 700 ./trustctl $DEST/bin/trustctl
else
  echo "Build the binary first: go build -o trustctl ."
fi

echo "Installation complete. Place plugin .so files into $DEST/plugins and credential YAMLs into $DEST/credentials"
