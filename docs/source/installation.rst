.. _installation:

Installation
============

GoNetem is composed of 2 binaries:

* ``gonetem-server``: it's the core of gonetem to emulate networks. It needs root access to create/launch
  docker nodes / switches and to create links between them.
* ``gonetem-console``: it's the console client to control `gonetem-server`

Requirements
------------
To run ``gonetem-server``, you have to install the following programs
 * docker-ce
 * bridge-utils

To run ``gonetem-console``, you have to install the following programs
 * xterm
 * bridge-utils

Manual Installation
-------------------
You can install GoNetem with the following command (with superuser privileges):

.. code-block:: bash

    $ sudo make install

With this command, gonetem-console/server are installed in ``/usr/local/bin`` folder.
And server configuration file are copied in ``/etc/gonetem/config.yaml``
To remove gonetem, you can use the following command

.. code-block:: bash

    $ sudo make uninstall

Debian Package
--------------

A Debian packages are available on `github <https://github.com/mroy31/pynetem/releases>`_
for amd64 architecture. It includes:

* gonetem-console/server
* a default configuration file for the server located at ``/etc/gonetem/config.yaml``
* a systemd service to launch gonetem-server in background
