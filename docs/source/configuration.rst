.. _configuration:

Configuration
=============

Server Configuration
--------------------

Configuration file
``````````````````

After the installation, the configuration of gonetem-server is done with the file
``/etc/gonetem/config.yaml``. Below, you will find the configuration by default :

.. code-block:: yaml

    listen: "localhost:10110"
    workdir: /tmp
    docker:
      images:
        server: mroy31/gonetem-server
        host: mroy31/gonetem-host
        router: mroy31/gonetem-frr
        ovs: mroy31/gonetem-ovs


Pull docker images
``````````````````

Before using gonetem, you have to pull docker images built for it
and available on docker hub. For that, you can use the following command after
the launch of gonetem-server:

.. code-block:: bash

    $ gonetem-console pull

Launch server
`````````````

If you install gonetem manually, you have lo launch gonetem-server with the root
right. It is required to execute docker or netlink actions. You can use
the following command for example:

.. code-block:: bash

    $ sudo gonetem-server

Below, you will find available arguments to launch gonetem-server

.. code-block:: bash

    Usage of gonetem-server:
        -conf-file string
                Configuration path (default "/etc/gonetem/config.yaml")
        -log-file string
                Path of the log file (default: stdout)
        -verbose
                Display more messages

If you use debian package, gonetem-server is launch thanks to systemd.


MPLS support
````````````

gonetem supports MPLS. To work, you must enable MPLS features
in linux kernel by loading the following modules :

- mpls_iptunnel
- mpls_router

Console Configuration
---------------------

The first time you launch `gonetem-console`, a configuration file is created
at this location: ``~/.config/gonetem-console/console.yaml``
You can view the current configuration with the following command:

.. code-block:: bash

    $ gonetem-console config show

For each parameter, you can modify the configuration with the command:

.. code-block:: bash

    $ gonetem-console config set <param-key> <param-value>

For example, to use `nano` as topology editor, simply enter the command:

.. code-block:: bash

    $ gonetem-console config set editor nano

For now, the following options are available:

- ``server`` to set the server uri used for connection (default to localhost:10110)
- ``editor`` to select the editor used to edit the topology file (default to vim)
- ``terminal`` to set the command line used to launch a console, default to

.. code-block:: bash

    xterm -xrm 'XTerm.vt100.allowTitleOps: false' -title {{.Name}} -e {{.Cmd}}

For example, if you want to change the font family/size used by xterm, you can enter the following command:

.. code-block:: bash

    gonetem-console config set terminal "xterm -fa 'Monospace' -fs 13 -xrm 'XTerm.vt100.allowTitleOps: false' -title {{.Name}} -e {{.Cmd}}"
