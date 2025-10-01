.. _nodes:

Docker nodes configuration
==========================


Pre-configured nodes
---------------------

By default, 4 kind of docker nodes have been pre-configured in gonetem

- `router` to emulate a router thanks to FRR
- `host` to emulate a simple machine
- `server` to emulate a server machine with DHCP/TFTP/HTTP/DNS server
- `p4sw` to emulate a P4 switch based on the bmv2 implementation

These 3 nodes has a default configuration that can be modified in the ``docker.nodes``
section of server configuration file. By default, this section looks like this:

.. code-block:: yaml

    docker:
      nodes:
        router:
          image: mroy31/gonetem-frr
        host:
          image: mroy31/gonetem-host
        server:
          image: mroy31/gonetem-server
        p4sw:
          image: mroy31/gonetem-bmv2


with only docker image set in the configuration. However, many other options 
exist and can be overriden in the server configuration file. For example, this is
the full configuration for the node ``host``:

.. code-block:: yaml

    docker:
      nodes:
        host:
          type: host
          image: mroy31/gonetem-host
          volumes: []
          options:
            log: true
            tso: false
          commands:
            console: /bin/bash
            shell: /bin/bash
            loadConfig:
            - command: network-config.py -l /tmp/custom.net.conf
              checkFiles:
              - /tmp/custom.net.conf
            - command: /bin/bash /gonetem-init.sh
              checkFiles:
              - /gonetem-init.sh
            saveConfig:
            - command: network-config.py -s /tmp/custom.net.conf
              checkFiles: []
          configurationFiles:
          - destSuffix: init.conf
            source: /gonetem-init.sh
            label: Init
          - destSuffix: net.conf
            source: /tmp/custom.net.conf
            label: Network
          - destSuffix: ntp.conf
            source: /etc/ntpsec/ntp.conf
            label: NTP

List of parameters:

- ``type`` (string): type of node (used in the topology file to declare a docker node)
- ``image`` (string): docker image used to launch the container
- ``volumes`` (string list): Allow to bind host path in container filesystem (like -v option in ``docker run```). The syntax is ``/host/path:/container/path`` and can be completed in the topology definition.
- ``options``

  - ``log`` (boolean): show output messages of loadConfig commands
  - ``tso`` (boolean): set to false to disable TSO (TCP Segmentation Offload) on interfaces node

- ``commands``: list of commands needed by gonetem to manage the node

  - ``console``: command used by gonetem when a console is launch
  - ``shell``: command used by gonetem when a shell is launch
  - ``loadConfig``: list of commands used by gonetem when the node is started to load the configuration

    - ``command``: command executed on the node
    - ``checkFiles``: list of files that have to exist to launch the command

  - ``saveConfig``: list of commands used by gonetem when the save command is required by the user
  - ``configurationFiles``: list of files save in the .gnet project for this kind of node


Define new nodes
----------------

It is also possible to declare a new kind of node than can be used in topology file, in the server
configuration file (section ``docker.extraNodes`` as a list). The parameters are exactly the same than 
the integrated nodes in gonetem. Example:

.. code-block:: yaml

    docker:
      extraNodes:
      - type: myhost
        image: mydocker-img
        volumes: []
        options:
          log: false
          tso: true
        commands:
          console: /bin/myconsole
          shell: /bin/bash
          loadConfig:
          - command: my-load-script.sh
            checkFiles: []
          saveConfig:
          - command: my-save-script.sh
            checkFiles: []
        configurationFiles:
        - destSuffix: myappconf.conf
          source: /path/myconf.conf
          label: MyConf

Once define in the server configuration file, you can use this new node in the topology like that:

.. code-block:: yaml

    nodes:
      Host:
        type: docker.myhost


VyOS
````

A concrete example of this feature is the possibility to use `VyOS <https://vyos.io/>`_ router with gonetem.
To do that:

1. Build VyOS docker image compatible with gonetem (look at the ``docker/README.md`` file to do that)
2. Add this extra node configuration in the server configuration file 

.. code-block:: yaml

    docker:
      extraNodes:
      - type: vyos
        image: gonetem-vyos:1.4
        options:
          log: false
          tso: true
        commands:
          console: su - vyos
          shell: /bin/bash
          loadConfig:
          - command: /bin/bash /usr/bin/start-vyos.sh
            checkFiles: []
          saveConfig:
          - command: /bin/bash /usr/bin/save-vyos-config.sh
            checkFiles: []
        configurationFiles:
        - destSuffix: vyos.conf
          source: /opt/vyatta/etc/config/config.boot
          label: VyOS

Finally, restart gonetem server and after you can use ``docker.vyos`` node in your topology.
