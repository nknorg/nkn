#! /bin/bash

function Usage () {
    local RED="\E[1;31m"
    local GREEN="\E[1;32m"
    local YELLOW="\E[1;33m"
    local BLUE="\E[1;34m"
    local END="\E[0m"
    printf "${RED}Usage${END}: ${BLUE}%s${END} ${GREEN}<EXEC_DIR>${END} ${YELLOW}[join|create]${END}\n" $1
    printf "${RED}Example${END}: ${BLUE}%s${END} ${GREEN}./testbed/node0001${END} ${YELLOW}join${END}\n" $1
}

function initWallet () {
    rm -f wallet.dat
    ./nknc wallet -c <<EOF
testbed
testbed
EOF
    return $?
}

function start () {
    ./nknd -test $1 <<EOF
testbed
EOF
}

ulimit -c unlimited
export GOTRACEBACK=crash

[ -n "$1" ] || ! Usage $0 || exit $EINVAL
EXEC_DIR=$1
[ -n "$2" ] && MODE=$2 || MODE="join"

cd ${EXEC_DIR}
mkdir -p ./Log
[ -e "wallet.dat" ] || initWallet || ! echo "Init Wallet fail" || exit 1
start ${MODE} 1> ./Log/nohup.out.$(date +%F_%T) 2>&1
