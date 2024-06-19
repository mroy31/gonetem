#! /bin/sh

/usr/sbin/ovsdb-server --detach \
    --remote=punix:/var/run/openvswitch/db.sock \
    --pidfile=ovsdb-server.pid --remote=ptcp:6640

/usr/sbin/ovs-vswitchd --pidfile
