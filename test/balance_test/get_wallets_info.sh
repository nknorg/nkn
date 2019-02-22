#!/bin/bash

source utils/usage.sh
source sdk.sh

USAGE="./get_wallets_info.sh <node_ip_address>"
EXAMPLE="./get_wallets_info.sh 127.0.0.1"
EINVAL=22

[ $# -eq 1 ] || ! Usage "$USAGE" "$EXAMPLE" || exit $EINVAL

ip="${1}"
password="tt"
addr_list="./wallets/addr.lst"

cat ${addr_list}|while read name addr; do
    balance=$(get_balance $ip ./wallets/$name $password)
    nonce=$(get_nonce $ip ./wallets/$name $password)
    echo "$addr : (${balance} , ${nonce})";
done

