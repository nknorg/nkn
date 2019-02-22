#!/bin/bash

## create_wallet_address.sh
## Generate wallet files in batches
## Parameters:
## ${1} - count
## ${2} - password

source utils/usage.sh
source sdk.sh

USAGE="./create_wallet_address.sh <count> <password>"
EXAMPLE="./create_wallet_address.sh 5 pwd"
EINVAL=22

[ $# -eq 2 ] || ! Usage "$USAGE" "$EXAMPLE" || exit $EINVAL

count=$1
password=$2
wallet_path=./wallets


mkdir -p ${wallet_path}

for (( i = 0; i < $count; i++ )); do
    create_wallet "${wallet_path}/${i}.dat" $password
done

for file in `ls $wallet_path`; do
    echo $(echo -e "$file \c" && get_address "${wallet_path}/$file" $password) | tee -a ${wallet_path}/addr.lst
done

