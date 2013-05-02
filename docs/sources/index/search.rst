=======================
Docker Index Search API
=======================

Search
------

**URL:** /v1/search?q={{search_term}}

**Results:**

.. code-block:: json

   {"query":"{{search_term}}",
    "num_results": 27,
    "results" : [
       {"name": "dotcloud/base", "description": "A base ubuntu64  image..."},
       {"name": "base2", "description": "A base ubuntu64  image..."},
     ]
   }