###################
 Markdown rendering
###################

The interface is named eth<N> in a heading's text too, and ``eth<N>`` in code.

.. This comment holds <N>.

::

    ip link set eth<N> up

.. rationale:: An interface is named eth<N>
    :id: RAT-NET-TEST-OUTER

    The outer rationale holds another.

    .. rationale:: A rationale inside a rationale
        :id: RAT-NET-TEST-INNER

        The inner rationale.

A role in code: ``:ref:`x` and y``, and code that starts with a backquote:
:literal:`\`x\` and y`. A link to a file of the repository:
`openapi <../../../../api/openapi.yaml>`__.

The limit is
>=1280 bytes, set with ``echo 1
> /proc/x`` and kept by
- y + z.

.. list-table::
    :header-rows: 1

    - - Pattern
      - Meaning
    - - ``a|b`` and c|d
      - Either one
        or the other

.. _markdown-cells:

Markdown cells and anchors
==========================

The title's own id starts with the name of the document, as a label does.

A status at the start of a line is not a list: the request responds
403. A count is:
1. Not a list either.

>=1280 bytes start this paragraph.

# of retries starts this one.

.. list-table::
    :header-rows: 1

    - - Release
      - Change
    - - 29.0
      - First
    - - 29.10
      - Tenth

The README of the fixture is :doc:`../README`. A line of a paragraph
+ that starts with a plus, or
1) with a number.

2) A second
paragraph.

- 4789\. A list item that starts
  with a number.
