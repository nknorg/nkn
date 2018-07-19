#! /bin/bash
### NOTICE, This script ONLY work well on gcloud image:
### https://www.googleapis.com/compute/v1/projects/nkn-testnet/global/images/testnet-disk-baseline

source $HOME/.bash_profile

CI_DIR=$(dirname $(realpath $0))
NKN_HOME=$(realpath $CI_DIR/../..)
BIN_DIR="testbed/node_0001"

### Setup seed list by cmdline or hard code a default
[ -z "$1" ] && DEFAULT_SEED="35.201.21.23:30003" || DEFAULT_SEED=$1

###
# func: init bin dir and adjust config.json for adapted to run on cloud
# arg1: name of bin dir
# arg2: ip:port on seed list
###
function init_bin () {
    ./test/create_testbed.sh 2 $(dirname ${1})
    sed -i '/127.0.0.1/d' ./${1}/config.json
    sed -i '/SeedList/a"'$2'"' ./${1}/config.json
}

### init bin dir if it not exist
cd $NKN_HOME
[ -d ./${BIN_DIR} ] || init_bin ${BIN_DIR} ${DEFAULT_SEED}

### Obtain latest src code and build binary
git pull
make all
cp -a nkn[cd] ./${BIN_DIR}/

./test/launch_node.sh ./${BIN_DIR}/
