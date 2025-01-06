.. _ovs:

Switch console
==============

gonetem implements a custom console for switch emulated
thanks to ``openvswitch``.

This page lists all commands available in the switch console.

Global commands
---------------

Show MAC address table
```````````````````````

.. code-block:: shell

  show mac address-table

Show interfaces
```````````````
Show ovs port number used for this switch 

.. code-block:: shell

  show interfaces


STP configuration
------------------

Show stp state
``````````````

.. code-block:: shell

  show stp

Enable/Disable stp
``````````````````

.. code-block:: shell

  set stp enable|disable

VLAN configuration
------------------

Show vlan configuration
```````````````````````

.. code-block:: shell

  show vlan

Set vlan configuration
``````````````````````

.. code-block:: shell

  set vlan port <port-number> access|trunk <tags>

Examples

.. code-block:: shell

  set vlan port O access 10
  set vlan port 1 trunk 20,30

Delete vlan configuration
`````````````````````````

.. code-block:: shell

  delete vlan port <port-number> access|trunk <tags>

Examples

.. code-block:: shell

  delete vlan port O access 10
  delete vlan port 1 trunk 20,30


Bonding configuration
---------------------

Show status of a bond interface
```````````````````````````````

.. code-block:: shell

  show bonding <bond-name>

Example

.. code-block:: shell

  show bonding my-bond
  

Create a new bond interface
```````````````````````````

.. code-block:: shell

  set bonding <bond-name> port <port-number1> <port-number2>

Example

.. code-block:: shell

  set bonding my-bond port 2 3

For now, the configuration of the bond interface is :

- Mode: active-backup
- LACP active

Delete a bond interface
```````````````````````

.. code-block:: shell

  delete bonding <bond-name>

Example

.. code-block:: shell

  delete bonding my-bond
