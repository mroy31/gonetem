gonetem
=======

Description
-----------

gonetem is a network emulator written in go based on

* docker to emulate nodes (host, server or router)
* OpenvSwitch to emulate switch

Architecture
------------

gonetem is composed of 2 parts:

* `gonetem-server`: it's the core of gonetem to emulate networks. It needs root access to create/launch
  docker nodes / switches and to create links between them.
* `gonetem-console`: it's the console client to control `gonetem-server`

Requirements
------------

* `gonetem-server` depends on [docker](https://www.docker.com/)
* `gonetem-console` depends on:
  * `xterm` to open console
  * [wireshark](https://www.wireshark.org/) to capture trafic

Installation
------------

You can compile gonetem with the command

    $make build-[amd64|armv7|arm64]

The 2 binaries are then available in the `bin` folder

Import docker images
--------------------

Before using gonetem, you need to pull from docker hub images used by default by gonetem.

* mroy31/gonetem-frr -> to emulate router based on frr software
* mroy31/gonetem-host -> tp emulate host
* mroy31/gonetem-server -> to emulate server
* mroy31/gonetem-ovs -> to emulate switch with Openvswitch

To do that, you can use the following command:

    $gonetem-console pull

Usage
-----

To use gonetem, firstly you have lo launch gonetem-server with the root
right. You can use the following command for example:

    $sudo gonetem-server

Then, you can use gonetem-console to create/launch project. For example
to create an empty project:

    $gonetem-console create ./myproject.gnet

And after to open it

    $gonetem-console open ./myproject.gnet
    $[myproject]> edit # if you want to edit the topology
    $[myproject]> reload # to reload the new topology
    $[myproject]> console all # to open all consoles
    $[myproject]> quit


Graphical User Interface
------------------------

Now, a graphical user interface is available for gonetem,
see [gonetem-ui](https://github.com/mroy31/gonetem-ui)


License
-------

GNU General Public License v3.0 or later

See [COPYING](COPYING) to see the full text.
