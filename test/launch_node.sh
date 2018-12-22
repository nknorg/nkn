#! /bin/bash

function Usage () {
    local RED="\E[1;31m"
    local GREEN="\E[1;32m"
    local YELLOW="\E[1;33m"
    local BLUE="\E[1;34m"
    local END="\E[0m"
    printf "${RED}Usage${END}: ${BLUE}%s${END} ${GREEN}<EXEC_DIR>${END} ${YELLOW}[args...]${END}\n" $1
    printf "${RED}Example${END}: ${BLUE}%s${END} ${GREEN}./testbed/node0001${END} ${YELLOW}-seed 127.0.0.1:30003${END}\n" $1
}

function initWallet () {
    rm -f wallet.dat
    RANDOM_PASSWD=$(head -c 1024 /dev/urandom | shasum -a 512 -b | xxd -r -p | base64 | head -c 32)
    ./nknc wallet -c <<EOF
${RANDOM_PASSWD}
${RANDOM_PASSWD}
EOF
    echo ${RANDOM_PASSWD} > ./wallet.pswd
    return $?
}

function start () {
    RANDOM_PASSWD=$(cat ./wallet.pswd)
    ./nknd "$@" <<EOF
${RANDOM_PASSWD}
${RANDOM_PASSWD}
EOF
}

ulimit -n 4096
ulimit -c unlimited
export GOTRACEBACK=crash

[ -n "$1" ] || ! Usage $0 || exit $EINVAL
EXEC_DIR=$1
shift 1     ### Skip $1 pass to start()

cd ${EXEC_DIR}
mkdir -p ./Log
[ -e "wallet.dat" ] || initWallet || ! echo "Init Wallet fail" || exit 1
start "$@" 1> ./Log/nohup.out.$(date +%F_%T) 2>&1
