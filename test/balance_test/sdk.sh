#!/bin/bash

source "utils/usage.sh"

EINVAL=22


## parameters:
## $1 - wallet name
## $2 - password
function create_wallet() {
    local USAGE="./create_wallet <wallet_full_name> <password>"
    local EXAMPLE="./create wallet.dat pwd"

    [ $# -eq 2 ] || ! Usage "${USAGE}" "${EXAMPLE}" || exit ${EINVAL}

    local wallet_name=${1}
    local password=${2}

    ./nknc wallet --name "${wallet_name}" -p "$password" -c

    [ $? -ne 0 ] && exit ${EINVAL}
}


## parameters:
## $1 - ip address of NKN node
## $2 - wallet name
## $3 - wallet password
function get_nonce() {
    local USAGE="./get_nonce <nonde_ip_address> <wallet_full_name> <password>"
    local EXAMPLE="./get_nonce 127.0.0.1 wallet.dat pwd"

    [ $# -eq 3 ] || ! Usage "${USAGE}" "${EXAMPLE}" || exit

    local ip_addr=${1}
    local wallet_name=${2}
    local password=${3}

    ./nknc --ip $ip_addr wallet --name $wallet_name -p $password -l nonce|jq '.result.nonce'

    [ $? -ne 0 ] && exit ${EINVAL}
}

## parameters:
## $1 - ip address of NKN node
## $2 - wallet name
## $3 - wallet password
function get_balance() {
    local USAGE="./get_balance <nonde_ip_address> <wallet_full_name> <password>"
    local EXAMPLE="./get_balance 127.0.0.1 wallet.dat pwd"

    [ $# -eq 3 ] || ! Usage "${USAGE}" "${EXAMPLE}" || exit

    local ip_addr=${1}
    local wallet_name=${2}
    local password=${3}

    ./nknc --ip $ip_addr wallet --name $wallet_name -p $password -l balance | jq '.result.amount'| tr -d \"

    [ $? -ne 0 ] && exit ${EINVAL}
}

## parameters:
## $1 - ip address of NKN node
function get_height() {
    local USAGE="./get_height <nonde_ip_address> "
    local EXAMPLE="./get_height 127.0.0.1"

    [ $# -eq 1 ] || ! Usage "${USAGE}" "${EXAMPLE}" || exit

    local ip_addr=${1}

    ./nknc --ip $ip_addr info -c | jq '.result'

    [ $? -ne 0 ] && exit ${EINVAL}
}


## parameters:
## $1 - ip address of NKN node
function get_address() {
    local USAGE="./get_address <wallet_full_name> <password>"
    local EXAMPLE="./get_address wallet.dat pwd"

    [ $# -eq 2 ] || ! Usage "${USAGE}" "${EXAMPLE}" || exit

    local wallet_name=${1}
    local password=${2}

    ./nknc wallet --name "$wallet_name" -p $password -l account | grep N | cut -d " " -f 1

    [ $? -ne 0 ] && exit ${EINVAL}
}


## parameters:
## $1 - ip address of NKN node
## $2 - wallet
## $3 - password
## $4 - to
## $5 - value
## $6 - nonce
function transfer_asset() {
    local USAGE="tranfer_asset <ip> <wallet_full_name> <password> <to> <value> <nonce>"
    local EXAMPLE="tranfer_asset 127.0.0.1 wallet.dat pwd NdqxfcHnE8izuhWmajCh6dkH6rsgYrwU3Q 1 200"

    [ $# -eq 6 ] || ! Usage "${USAGE}" "${EXAMPLE}" || exit

    local ip=$1
    local wallet=$2
    local password=$3
    local to=$4
    local value=$5
    local nonce=$6

    ./nknc --ip $ip asset --transfer -value $value  -to $to --nonce $nonce --wallet $wallet -p $password | jq '.result'

    [ $? -ne 0 ] && exit ${EINVAL}
}


