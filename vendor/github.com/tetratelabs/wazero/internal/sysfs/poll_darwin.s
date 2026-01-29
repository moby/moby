// lifted from golang.org/x/sys unix
#include "textflag.h"

TEXT libc_poll_trampoline<>(SB), NOSPLIT, $0-0
	JMP libc_poll(SB)

GLOBL ·libc_poll_trampoline_addr(SB), RODATA, $8
DATA ·libc_poll_trampoline_addr(SB)/8, $libc_poll_trampoline<>(SB)
