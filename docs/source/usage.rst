.. _usage:

Usage
=====

Console
-------

Once gonetem-server is running, you can use gonetem-console
to create/launch project. For example to create an empty project:

.. code-block:: bash

    $ gonetem-console create ./myproject.gnet

And after to open it

.. code-block:: bash

    $ gonetem-console open ./myproject.gnet
    [myproject]> edit # if you want to edit the topology
    [myproject]> reload # to reload the new topology
    [myproject]> console all # to open all consoles
    [myproject]> quit

For more details, See
  * :ref:`topology` for more detail to build a network
  * :ref:`commands` for the list of available commands in the prompt

Available commands
------------------

.. code-block:: bash

    Usage:
    gonetem-console [command]

    Available Commands:
    clean       Prune containers not used by any project
    config      Configure gonetem-console
    connect     Connect to a running project
    console     Open a console to the specified node
    create      Create a project
    extract     Extract files from a project
    help        Help about any command
    list        List running projects on the server
    open        Open a project
    pull        Pull required docker images on the server
    version     Print the version number of gonetem

    Flags:
    -h, --help            help for gonetem-console
    -s, --server string   Override server uri defined in config file
