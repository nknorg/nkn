#!/bin/bash

source utils/usage.sh
source sdk.sh

USAGE="./distribute_token.sh <node_ip_address>"
EXAMPLE="./distribute_token.sh 127.0.0.1"
EINVAL=22

[ $# -eq 1 ] || ! Usage "$USAGE" "$EXAMPLE" || exit $EINVAL



ip_addr=${1}
addr_list="./wallets/addr.lst"
wallet="./data/wallet.dat"
password="tt"
nonce=$(get_nonce $ip_addr $wallet $password)

cat ${addr_list}|while read name addr; do
    echo "nonce:" $nonce $addr $name
    transfer_asset $ip_addr $wallet $password $addr 1 $nonce 
    nonce=$(expr $nonce + 1);
    sleep 1
done

