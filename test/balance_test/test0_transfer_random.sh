#!/bin/bash

source utils/usage.sh
source sdk.sh

USAGE="./test0_transfer_random.sh"
EXAMPLE="./test0_transfer_random.sh"
EINVAL=22

[ $# -eq 0 ] || ! Usage "$USAGE" "$EXAMPLE" || exit $EINVAL

ip="35.237.99.200"
password="tt"
addr_lst="./wallets/addr.lst"

cat ${addr_lst}|while read name addr; do
    to=`sort -R ${addr_lst} |head  -n1|cut -d " " -f 2`
    balance=`get_balance $ip ./wallets/$name $password`
    nonce=`get_nonce $ip ./wallets/$name $password`
    echo "from ${addr}(${balance} , ${nonce}) transfer 0.00000001 to $to"
    transfer_asset $ip ./wallets/$name $password $to 0.00000001 $nonce 
done

