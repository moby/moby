from busybox
add f /
run [ "$(cat /f)" = "hello" ]
add f /abc
run [ "$(cat /abc)" = "hello" ]
add f /x/y/z
run [ "$(cat /x/y/z)" = "hello" ]
add f /x/y/d/
run [ "$(cat /x/y/d/f)" = "hello" ]
add d /
run [ "$(cat /ga)" = "bu" ]
add d /somewhere
run [ "$(cat /somewhere/ga)" = "bu" ]
add d /anotherplace/
run [ "$(cat /anotherplace/ga)" = "bu" ]
add d /somewheeeere/over/the/rainbooow
run [ "$(cat /somewheeeere/over/the/rainbooow/ga)" = "bu" ]
