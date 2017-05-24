#!/bin/bash

set -u

RIEMANN_VERSION=$1
RIEMANN_URL=https://github.com/riemann/riemann/releases/download/${RIEMANN_VERSION}/riemann-${RIEMANN_VERSION}.tar.bz2

echo "Download Riemann"
wget ${RIEMANN_URL}
echo "Untar Riemann"
tar xjf riemann-${RIEMANN_VERSION}.tar.bz2
echo "Launch Riemann"
riemann-${RIEMANN_VERSION}/bin/riemann &
RIEMANN_PID=$!
sleep 10
echo "Launch tests"
echo
echo
go test -tags=integration
echo
echo
echo "Stop Riemann"
kill -9 ${RIEMANN_PID}
