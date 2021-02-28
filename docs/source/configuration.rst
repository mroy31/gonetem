.. _configuration:

Configuration
=============

Configuration file
------------------
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


Server
------
If you install pynetem manually, you have lo launch gonetem-server with the root
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


Pull docker images
------------------

Before using gonetem, you have to pull docker images built for it
and available on docker hub. For that, you can use the following command after
the launch of gonetem-server:

.. code-block:: bash

    $ gonetem-console pull

MPLS support
------------

gonetem supports MPLS. To work, you must enable MPLS features
in linux kernel by loading the following modules :

- mpls_iptunnel
- mpls_router
