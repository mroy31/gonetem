.. _commands:

Commands
========

This page lists all commands available in the gonetem prompt.

capture
-------
Capture trafic on the given node interface with
`Wireshark <https://www.wireshark.org/>`_ (must be installed first)

Usage:

.. code-block:: bash

  capture <node_name>.<if_number>
  # example
  capture R1.0

check
-----
Check that the topology file is correct. If not, return found errors

config
------
Save all the node configuration files in a specific folder.

Usage:

.. code-block:: bash

  config <dest_path>

copy
----
Copy files/folder between a docker node and the host fs or vice versa.

Usage:

.. code-block:: bash

  copy node:/mypath/myfile.txt /hostpath/

console
-------
Open a console for a node (specifing by the *node's name*) or all the nodes
(specifing by *all*). ``xterm`` is used to launch the console.

The kind of console opened by this command depends on the type of node:

* For docker host/server node: ``bash``
* For docker frr node, run directly ``vtysh``
* For ovswitch node, it runs a custom prompt to manage switch.
  Commands available in this prompt are detailed :ref:`here <ovs>`.

edit
----
Edit the topology. The editor used to open the topology file is vim.

exec
----
Execute a command on a specific node

Usage:

.. code-block:: bash

  exec <node_name> "<cmd>"
  # example
  exec host1 "ip addr show"

ifState
-------
Enable/disable a node interface.

Usage:

.. code-block:: bash

  ifState <node_name>/<if_number> up|down
  # example
  ifState R1.0 down

quit | exit
-----------
Close the project and quit the gonetem-console.

reload
------
Reload the project. You have to run this command after modifing the
topology. It does the following actions:

- Stop all running swithes/nodes/bridges
- Load the new topology
- Start all switches/nodes/bridges

restart
-------
Restart a node or all the nodes. Same principle than *start* command.

run
----
If the project has not been start during gonetem-console launch, run this command to
load the topology and start all the nodes.

save
----
Save the project. This command does two things:

- save the current topology
- for each running node, save the current of the node

saveAs
------
Save the project in a new file

.. code-block:: bash

  # example
  saveAs /newPath/newProject.gnet

shell
-----
Same as *console* command, except run ``bash`` command whatever the node.

start
-----
Start a node or all the nodes

Usage:

.. code-block:: bash

  # start one node
  start <node_name>
  # start all the nodes
  start all

status
------
Display the status of the project/topology

stop
----
Stop a node or all the nodes. Same principle than *start* command.

Usage:

.. code-block:: bash

  # stop one node
  stop <node_name>
  # stop all the nodes
  stop all
