:title: Attach Command
:description: Attach to a running container
:keywords: attach, container, docker, documentation

===========================================
``attach`` -- Attach to a running container
===========================================

::

    Usage: docker attach CONTAINER

    Attach to a running container.

You can detach from the container again (and leave it running) with
``CTRL-c`` (for a quiet exit) or ``CTRL-\`` to get a stacktrace of
the Docker client when it quits.

To stop a container, use ``docker stop``

To kill the container, use ``docker kill``
 
Examples:
---------

.. code-block:: bash

     $ ID=$(sudo docker run -d ubuntu /usr/bin/top -b)
     $ sudo docker attach $ID
     top - 02:05:52 up  3:05,  0 users,  load average: 0.01, 0.02, 0.05
     Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
     Cpu(s):  0.1%us,  0.2%sy,  0.0%ni, 99.7%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
     Mem:    373572k total,   355560k used,    18012k free,    27872k buffers
     Swap:   786428k total,        0k used,   786428k free,   221740k cached

     PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND            
      1 root      20   0 17200 1116  912 R    0  0.3   0:00.03 top                

      top - 02:05:55 up  3:05,  0 users,  load average: 0.01, 0.02, 0.05
      Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
      Cpu(s):  0.0%us,  0.2%sy,  0.0%ni, 99.8%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
      Mem:    373572k total,   355244k used,    18328k free,    27872k buffers
      Swap:   786428k total,        0k used,   786428k free,   221776k cached

        PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND            
	    1 root      20   0 17208 1144  932 R    0  0.3   0:00.03 top                


      top - 02:05:58 up  3:06,  0 users,  load average: 0.01, 0.02, 0.05
      Tasks:   1 total,   1 running,   0 sleeping,   0 stopped,   0 zombie
      Cpu(s):  0.2%us,  0.3%sy,  0.0%ni, 99.5%id,  0.0%wa,  0.0%hi,  0.0%si,  0.0%st
      Mem:    373572k total,   355780k used,    17792k free,    27880k buffers
      Swap:   786428k total,        0k used,   786428k free,   221776k cached

      PID USER      PR  NI  VIRT  RES  SHR S %CPU %MEM    TIME+  COMMAND            
           1 root      20   0 17208 1144  932 R    0  0.3   0:00.03 top                
     ^C$ 
     $ sudo docker stop $ID

