#! /bin/bash

EINVAL=22

function Usage () {
    local RED="\E[1;31m"
    local GREEN="\E[1;32m"
    local YELLOW="\E[1;33m"
    local BLUE="\E[1;34m"
    local END="\E[0m"
    printf "${RED}Usage${END}: ${BLUE}%s${END} ${GREEN}<NodeNum>${END} ${YELLOW}[dest_dir]${END}\n" $1
    printf "${RED}Example${END}: ${BLUE}%s${END} ${GREEN}10${END} ${YELLOW}test_env${END}\n" $1
}

[ -n "$1" ] && [ "$1" -gt 1 ] || ! Usage $0 || exit $EINVAL
NodeNum=$1

TARGET_DIR=${2}
[ -z "$TARGET_DIR" ] && TARGET_DIR="testbed"


ECANCELED=125
[ -e $TARGET_DIR ] && printf "The testbed directory already existed\n" && exit ${ECANCELED}

mkdir -p ${TARGET_DIR}

seq 1 ${NodeNum} | xargs printf "%04d\n" | while read i
do
    mkdir -p ${TARGET_DIR}/node_${i}
    cp -a nknd nknc config.json ${TARGET_DIR}/node_${i}/  || exit $?

    ### init wallet.dat
    rm -f wallet.dat
    ./nknc wallet -c <<EOF
testbed
testbed
EOF
    [ $? -eq 0 ] || exit $?
    mv wallet.dat ${TARGET_DIR}/node_${i}/ || exit $?
done
