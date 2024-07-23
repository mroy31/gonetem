.. _nodes:

Docker nodes configuration
==========================


Gonetem default nodes
---------------------

By default, 3 kind of docker nodes can ben used in gonetem topology

- `router` to emulate a router thanks to FRR
- `host` to emulate a simple machine
- `server` to emulate a server machine with DHCP/TFTP/HTTP/DNS server

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


with only docker image set in the configuration. However, many other options 
exist and can be overriden in the server configuration file. For example, this is
the full configuration for the node ``host``:

.. code-block:: yaml

    docker:
      nodes:
        host:
          type: host
          image: mroy31/gonetem-host
          logOutput: true
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
- ``logOutput`` (boolean): show output messages of loadConfig commands
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
        logOutput: false
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


