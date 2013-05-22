:title: Docker Index documentation
:description: Documentation for docker Index
:keywords: docker, index, api


=======================
Docker Index Search API
=======================

Search
------

.. http:get:: /v1/search

   Search the Index given a search term. It accepts :http:method:`get` only.

   **Example request**:

   .. sourcecode:: http

      GET /v1/search?q=search_term HTTP/1.1
      Host: example.com
      Accept: application/json

   **Example response**:

   .. sourcecode:: http

      HTTP/1.1 200 OK
      Vary: Accept
      Content-Type: application/json

      {"query":"search_term",
        "num_results": 2,
        "results" : [
           {"name": "dotcloud/base", "description": "A base ubuntu64  image..."},
           {"name": "base2", "description": "A base ubuntu64  image..."},
         ]
       }

   :query q: what you want to search for
   :statuscode 200: no error
   :statuscode 500: server error