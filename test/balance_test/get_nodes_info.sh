#!/bin/bash

source utils/usage.sh
source sdk.sh

USAGE="./get_nodes_info.sh "
EXAMPLE="./get_nodes_info.sh"
EINVAL=22

[ $# -eq 0 ] || ! Usage "$USAGE" "$EXAMPLE" || exit $EINVAL

password="tt"
ip_list="./data/ip.lst"

cat ${ip_list}|while read ip; do
    height=$(get_height $ip)
    echo "$ip : $height";
done

