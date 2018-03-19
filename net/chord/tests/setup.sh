#!/bin/bash

for i in {1..5}; do
	var=$(($i*1))
	echo $var
	screen -dmS "$i-tests" ./tests -start $var
	sleep 2
done
