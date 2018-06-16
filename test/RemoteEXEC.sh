#! /bin/bash

EINVAL=22

### args: node_lst, user, cmd
function remote_exec () {
    [ $# -ne 3 ] || [ -z "$3" ] && echo "Invalid args [$@]" && return $EINVAL

    #echo CMD="$3"
    for ip in $(awk '{print $2}' ${1})
    do
        ssh ${2}@${ip} 'echo "$(printf "\n### %s ###\n" $(hostname); '$3')"' &
    done
    wait
}

### Example
#remote_exec ~/cluster2.lst admin "cd ~/nknorg/src/github.com/nknorg/nkn/ && git pull"
#remote_exec ~/cluster2.lst admin 'export PATH=/usr/lib/go-1.10/bin:$PATH && export GOPATH=$HOME/nknorg && make -C ~/nknorg/src/github.com/nknorg/nkn/ all'
#remote_exec ~/cluster2.lst admin "cd ~/nknorg/src/github.com/nknorg/nkn/ && ./test/create_testbed.sh 2"
#remote_exec ~/cluster2.lst admin "cd ~/nknorg/src/github.com/nknorg/nkn/ && ./test/launch_node.sh ./testbed/node_0001 join"
