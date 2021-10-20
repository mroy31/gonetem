.. _ovs:

Switch console
==============

gonetem implements a custom console for switch emulated
thanks to ``openvswitch``.

This page lists all commands available in the switch console.

vlan_show
---------

Show vlan configuration

vlan_access
-----------

Assign a port to a VLAN in access mode

.. code-block:: bash

  # example
  vlan_access port 0 vlan 10

vlan_trunks
-----------

Set a port as trunk and assign VLANs

.. code-block:: bash

  # example
  vlan_trunks port 0 vlan 10,20

no_vlan_access
--------------

Cancel a ``vlan_access`` command

.. code-block:: bash

  # example
  no_vlan_access port 0 vlan 10

no_vlan_trunks
--------------

Cancel a ``vlan_trunks`` command

.. code-block:: bash

  # example
  no_vlan_trunks port 0 vlan 10,20

bonding_show
------------

Display informations about a bonding interface

.. code-block:: bash

  # example
  bonding_show bond0

bonding
-------

Set a bonding interface and attach physical interface (minimum 2)

.. code-block:: bash

  # example
  bonding bond0 0 1

no_bonding
----------

Remove a bonding interface

.. code-block:: bash

  # example
  no_bonding bond0
