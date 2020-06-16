#!/bin/sh
# Script to build DISCO with the correct `go get` flags.  This script
# was designed and tested to run as part of the container build process.
set -ex

COMMIT=$(git log -1 --format=%h)
versionflags="${versionflags} -X github.com/m-lab/go/prometheusx.GitShortCommit=${COMMIT}"
go get -v -t -ldflags "$versionflags -extldflags \"-static\""                   \
