.. _tls:

TLS/SSL configuration
=====================

This page explains how to secure gRPC connection between console and server with TLS/SSL. 
By default, the connection is insecure since server listen on the loopback and the console is running on the same computer than the server.
However, in some cases, it can be interesting to have the server on a remote linux computer and the console running locally on MAC OS X or Windows. 
In this case, it is mandatory to secure the gRPC connection. That's why this option is available with gonetem.

Generation of the certificates
------------------------------

The first step to secure the connection is to generate auto-signed certificates. 
To do that, you can use the script ``scripts/generate-cert.sh``. However, before using this file, you need to create 2 configuration files:

- ``client-ext.cnf``: extension for console certificates. It can be empty.
- ``server-ext.cnf``: extension for server certificates. It must at least contain the subjectAltName parameter in order to be valid for the client. An example:

.. code-block::

    subjectAltName=DNS:localhost,DNS:*.mydomain.org

Once the 2 configuration files has been created, you can run the script.

.. code-block:: bash

    $ sh scripts/generate-cert.sh

This script generates the following files :

- ``ca-cert.pem``: Our Certificates Authority certificate to sign console/server certificates 
- ``client-cert.pem``, ``client-key.pem``: certificate and private key for the console, sign by the CA certicate
- ``server-cert.pem``, ``server-key.pem``: certificate and private key for the server, sign by the CA certicate

Configuration of gonetem
------------------------

Server configuration
````````````````````

To configure the server:

- Copy the files ``ca-cert.pem``, ``server-cert.pem`` and ``server-key.pem`` in the folder ``/etc/gonetem``
- Edit the server configuration file ``/etc/gonetem/config.yaml``, and update the TLS option:

.. code-block:: yaml

    tls:
      enabled: true
      ca: /etc/gonetem/ca-cert.pem
      cert: /etc/gonetem/server-cert.pem
      key: /etc/gonetem/server-key.pem


Console configuration
`````````````````````

To configure the server:

- Copy the files ``ca-cert.pem``, ``client-cert.pem`` and ``client-key.pem`` in a user folder for example ``/users/myuser/mydir``
- Run the following commands to cufigure the console

.. code-block:: bash

    $ gonetem-console config set tls.enabled true
    $ gonetem-console config set tls.cert /users/myuser/mydir/client-cert.pem
    $ gonetem-console config set tls.key /users/myuser/mydir/client-key.pem
