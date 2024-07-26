#!/bin/bash

sed -i "s/host-name \"vyos\"/host-name \"$(cat /etc/hostname)\"/g" \
	/opt/vyatta/etc/config/config.boot

systemctl start vyos-configd
systemctl start vyos-router

TIMEOUT=30
i=0

while [ $i -le $TIMEOUT ] ; do
  if su -c "ls" vyos 1> /dev/null; then
      break
  fi
  sleep 1s
  ((i++))
done
