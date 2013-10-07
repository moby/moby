:title: Ls Command
:description: Ls all links for containers
:keywords: ls, docker, container, documentation, link, links

============================================================================
``ls`` -- List all links for containers
============================================================================

::

    Usage: docker ls

    List all links for containers and display the relationship between parent
    and child containers.


Examples:
--------

.. code-block:: bash

    $ docker ls
    NAME                                                                      ID                                                                 IMAGE
    /redis                                                                    39588b6a45100ef5b328b2c302ea085624f29e6cbab70f88be04793af02cec89   crosbymichael/redis:latest
    /webapp                                                                   cffb86ffa80b11cd8777d300759ee53c4e61729431c30ec9552dd9e6d3abc87d   demo:latest
    /webapp/redis                                                             39588b6a45100ef5b328b2c302ea085624f29e6cbab70f88be04793af02cec89   crosbymichael/redis:latest

This will display all links and the names that you can use the reference the link.  Parent child 
relationships are also displayed with ls.
