/*
Package query provides the means the filter a collection of arbitrary data structures according to user provided query.

Here's an example for a query over docker containers:

  matcher, err := query.Parse("!running & (exit!=0 | name~junk", ...)
  if matcher.Match(container) {
    :
    :
  }

The queries supported by this library are parsed from a string represetnation according to the following rules:

Boolean fields

The simplest form a query is a simple boolean field:

  running

String fields

Fields of type string are supported and can be used with the following operators:

- "=" : Exact equality
  name=library/container

- "!=" : Not equal to:
  name!=value

- "~" : Like, i.e. the field value must contain the provided value:
  name~value

- "!~" : Not like:
  name!~value

String slice fields

bateau/query also supports multi-valued fields.
The same operators as for string fields (=, !=, ~, !~) are available for multi-valued fields.


- "=" : Matches if any value of the slice is exactly equal to the provided value
  cmd=/bin/sh

- "!=" : Fails if any value of the slice is exactly equal to the provided value
  cmd!=/bin/sh

- "~" : Matches if any value of the slice contains the provided value
  cmd~sh

- "!~" : Fails if any value of the slice contains the provided value
  cmd!~sh

Negation

A condition can be negated using the "!" operator, e.g.:

  !running

or

  !name=server

And

Two or more conditions can be combined using the boolean and operator "&"

  running & name=server

or

  running & image=mongo & cmd~sh

Or

Two or more conditions can be combined using the boolean or operator "|"

  running | name=server

or

  running | image=mongo | cmd~sh

Parenthesis

To control the evaluation precedence, conditions can be wrapped between "(" and ")":

  running & !(name=server | image~mongo)


Grammar

The query langauge is described below using the EBNF notation:

  expr     -> or
  or       -> and ('|' and)*
  and      -> atom ('&' atom)*
  atom     -> cond | '(' expr ')' | '!' atom
  cond     -> LITERAL (OPERATOR LITERAL)?
  LITERAL  -> "[^|&!=~]+"
  OPERATOR -> '=' | '!=' | '~' | '!~' | '<' | '<=' | '>' | '>='
*/
package query
