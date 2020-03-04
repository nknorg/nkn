#!/bin/sh
set -e

if [ "$1" == "nknd" ]; then
  ! [ -s web ] && echo "Copying web directory..." && cp -R ../web web
  ! [ -s certs ] && echo "Copying default certs..." && cp -R ../certs certs
  ! [ -s config.json ] && echo "Copying default config.json..." && cp /nkn/config.mainnet.json config.json
  if echo "$@" | grep -q -v " --web-gui-create-wallet"; then
    ! [ -s wallet.json ] && ! [ -s wallet.pswd ] && echo "Creating wallet.pswd..." && head -c 1024 /dev/urandom | tr -dc A-Za-z0-9 | head -c 32 > wallet.pswd && echo >> wallet.pswd
    ! [ -s wallet.json ] && echo "Creating wallet.json..." && cat wallet.pswd wallet.pswd | nknc wallet -c
  fi
fi

exec "$@"
