#!/bin/sh

if [ ! -z "$COREFILE" ]; then
  echo "$COREFILE" > /coredns/Corefile
  corefile=/coredns/Corefile
else
  corefile=/etc/coredns/Corefile
fi

/coredns/coredns -conf $corefile "$@"
