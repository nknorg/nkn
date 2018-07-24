#!/bin/bash

EINVAL=22

function Usage () {
    local RED="\E[1;31m"
    local GREEN="\E[1;32m"
    local YELLOW="\E[1;33m"
    local BLUE="\E[1;34m"
    local END="\E[0m"
    printf "${RED}Usage${END}: ${BLUE}%s${END} ${GREEN}<start|stop>${END} ${YELLOW}[count] [regions...]${END}\n" $1
    printf "${RED}Example${END}: ${BLUE}%s${END} ${GREEN}start${END} ${YELLOW}100${END}\n" $1
    printf "${RED}\t${END}: ${BLUE}%s${END} ${GREEN}start${END} ${YELLOW}100 asia-east1 us-west1 europe-north1${END}\n" $1
}

### Check target exist or NOT
# $1: type
# $2: target
# $3: filter
# $4: output format
# $5: misc
function isExist() {
    local F_PREFIX
    local O_PREFIX
    [ -z $3 ] || F_PREFIX="--filter"
    [ -z $4 ] || O_PREFIX="--format"

    #echo gcloud compute $1 list ${F_PREFIX} $3 ${O_PREFIX} $4 $5
    gcloud compute $1 list ${F_PREFIX} $3 ${O_PREFIX} $4 $5 | grep "$2"
    return $?
}

### Create image if NOT exist
# $1: name
function CreateImage() {
    local FORMAT="value(NAME,PROJECT,FAMILY)"

    isExist images "$1" "name:$1" "${FORMAT}" || \
        gcloud compute images create "$1" --source-disk testbed-demo --family testnet-disk
    return $?
}

### Create instance-templates if NOT exist
# $1: name
# $2: region
function CreateTemplate() {
    local FORMAT="value(NAME,MACHINE_TYPE,CREATION_TIMESTAMP)"

    isExist "instance-templates" "${1}" "name:${1}" "${FORMAT}" || \
        gcloud compute instance-templates create "$1" \
            --machine-type n1-standard-1 \
            --image-family testnet-disk \
            --image-project nkn-testnet \
            --boot-disk-size 20GB \
            --subnet default \
            --tags nkntestnet,http-server,https-server \
            --region "$2"
    return $?
}

### Create instance-groups if NOT exist
# $1: name
# $2: count
# $3: region
function CreateGroup() {
    local FORMAT="value(NAME,LOCATION,SCOPE,MANAGED,INSTANCES)"

    isExist "instance-groups" "${1}" "name:${1}" "${FORMAT}"
    if [ $? -eq 0 ]; then    ### resize it if existed already
        gcloud compute instance-groups managed resize ${1} --size "$2" --region "$3"
    else    ### create it if not existed
        gcloud beta compute instance-groups managed create "$1" \
            --base-instance-name testbed-demo --size "$2" --template "${3}-node-template" --region "$3"
    fi
    return $?
}

function Start() {
    [ $# -ge 1 ] || ! echo missing instances count. || return ${EINVAL}
    [ $1 -ge 0 ] || ! echo $1 NOT a positive integer. || return ${EINVAL}

    CNT=$1
    shift 1

    ### Default deploy to all regions if NOT indicate regions list
    [ $# -ne 0 ] && REGION_LIST=($@) || REGION_LIST=($(gcloud compute regions list --format="value(name)"))
    echo "Deploy $CNT instances on ${#REGION_LIST[@]} regions [${REGION_LIST[@]}]"

    PER_REG=$(($CNT/${#REGION_LIST[@]}))
    MOD=$(($CNT%${#REGION_LIST[@]}))
    echo $CNT instances / ${#REGION_LIST[@]} regions = $PER_REG mod $MOD

    ### Check image
    CreateImage testnet-disk-baseline

    for reg in ${REGION_LIST[@]:0:${MOD}}; do
        cnt=$((${PER_REG}+1))
        echo Start ${cnt} instances in region:${reg}

        CreateTemplate "${reg}-node-template" ${reg} && \
            CreateGroup "testnet-${reg}-group" ${cnt} ${reg} &
    done

    if [ $PER_REG -gt 0 ]; then
        for reg in ${REGION_LIST[@]:${MOD}}; do
            cnt=${PER_REG}
            echo Start ${cnt} instances in region:${reg}

            CreateTemplate "${reg}-node-template" ${reg} && \
                CreateGroup "testnet-${reg}-group" ${cnt} ${reg} &
        done
    fi
    wait
}

function Stop() {
    ### Default deploy to all regions if NOT indicate regions list
    [ $# -ne 0 ] && REGION_LIST=($@) || REGION_LIST=($(gcloud compute regions list --format="value(name)"))
    echo "Destroy instances on ${#REGION_LIST[@]} regions [${REGION_LIST[@]}]"

    for reg in ${REGION_LIST[@]}; do
        echo "Stop all instances in region:${reg}"
        gcloud compute instance-groups managed resize "testnet-${reg}-group" --size 0 --region ${reg} &
    done
    wait
}

### Check requirements and argument
gcloud version || ! echo gcloud NOT found. || exit $?

OP=$1
shift 1
case "${OP}" in
    start  ) Start $@;;
    stop   ) Stop  $@;;
    *      ) echo "$0: unknown operation.: $OP $@";;
esac

[ $? -ne 0 ] && Usage $0
