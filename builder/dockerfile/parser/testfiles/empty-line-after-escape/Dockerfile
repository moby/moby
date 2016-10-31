FROM busybox

# The following will create two instructions
# `Run foo`
# `bar`
# because empty line will break the escape.
# The parser will generate the following:
# (from "busybox")
# (run "foo")
# (bar "")
# And `bar` will return an error instruction later
# Note: Parse() will not immediately error out.
RUN foo \

bar
