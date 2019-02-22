#!/bin/bash

## Usage
## ${1} - usage
## ${2} - example
function Usage () {
	local RED="\E[1;31m"
	local GREEN="\E[1;32m"
	local YELLOW="\E[1;33m"
	local BLUE="\E[1;34m"
	local END="\E[0m"

	local usage="${1}"
	local example="${2}"

	printf "${RED}Usage${END}: ${BLUE}%s${END}\n" "$usage"
	printf "${RED}Example${END}: ${BLUE}%s${END}\n" "$example"
}
