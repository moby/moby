/*-
 * Copyright (c) 1998 Doug Rabson
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY THE AUTHOR AND CONTRIBUTORS ``AS IS'' AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED.  IN NO EVENT SHALL THE AUTHOR OR CONTRIBUTORS BE LIABLE
 * FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 * OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
 * LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
 * OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
 * SUCH DAMAGE.
 *
 * $FreeBSD$
 */

#pragma once

#include <stdint.h>
#include <xhyve/support/misc.h>

#define	__compiler_membar()	__asm __volatile(" " : : : "memory")

#define	mb()	__asm __volatile("mfence;" : : : "memory")
#define	wmb()	__asm __volatile("sfence;" : : : "memory")
#define	rmb()	__asm __volatile("lfence;" : : : "memory")

/*
 * Various simple operations on memory, each of which is atomic in the
 * presence of interrupts and multiple processors.
 *
 * atomic_set_char(P, V)	(*(u_char *)(P) |= (V))
 * atomic_clear_char(P, V)	(*(u_char *)(P) &= ~(V))
 * atomic_add_char(P, V)	(*(u_char *)(P) += (V))
 * atomic_subtract_char(P, V)	(*(u_char *)(P) -= (V))
 *
 * atomic_set_short(P, V)	(*(u_short *)(P) |= (V))
 * atomic_clear_short(P, V)	(*(u_short *)(P) &= ~(V))
 * atomic_add_short(P, V)	(*(u_short *)(P) += (V))
 * atomic_subtract_short(P, V)	(*(u_short *)(P) -= (V))
 *
 * atomic_set_int(P, V)		(*(u_int *)(P) |= (V))
 * atomic_clear_int(P, V)	(*(u_int *)(P) &= ~(V))
 * atomic_add_int(P, V)		(*(u_int *)(P) += (V))
 * atomic_subtract_int(P, V)	(*(u_int *)(P) -= (V))
 * atomic_swap_int(P, V)	(return (*(u_int *)(P)); *(u_int *)(P) = (V);)
 * atomic_readandclear_int(P)	(return (*(u_int *)(P)); *(u_int *)(P) = 0;)
 *
 * atomic_set_long(P, V)	(*(u_long *)(P) |= (V))
 * atomic_clear_long(P, V)	(*(u_long *)(P) &= ~(V))
 * atomic_add_long(P, V)	(*(u_long *)(P) += (V))
 * atomic_subtract_long(P, V)	(*(u_long *)(P) -= (V))
 * atomic_swap_long(P, V)	(return (*(u_long *)(P)); *(u_long *)(P) = (V);)
 * atomic_readandclear_long(P)	(return (*(u_long *)(P)); *(u_long *)(P) = 0;)
 */

#define	MPLOCKED	"lock ; "

/*
 * The assembly is volatilized to avoid code chunk removal by the compiler.
 * GCC aggressively reorders operations and memory clobbering is necessary
 * in order to avoid that for memory barriers.
 */
#define	ATOMIC_ASM(NAME, TYPE, OP, CONS, V)		\
static __inline void					\
atomic_##NAME##_##TYPE(volatile u_##TYPE *p, u_##TYPE v)\
{							\
	__asm __volatile(MPLOCKED OP			\
	: "+m" (*p)					\
	: CONS (V)					\
	: "cc");					\
}							\
							\
static __inline void					\
atomic_##NAME##_barr_##TYPE(volatile u_##TYPE *p, u_##TYPE v)\
{							\
	__asm __volatile(MPLOCKED OP			\
	: "+m" (*p)					\
	: CONS (V)					\
	: "memory", "cc");				\
}							\
struct __hack

/*
 * Atomic compare and set, used by the mutex functions
 *
 * if (*dst == expect) *dst = src (all 32 bit words)
 *
 * Returns 0 on failure, non-zero on success
 */

static __inline int
atomic_cmpset_int(volatile u_int *dst, u_int expect, u_int src)
{
	u_char res;

	__asm __volatile(
	"	" MPLOCKED "		"
	"	cmpxchgl %3,%1 ;	"
	"       sete	%0 ;		"
	"# atomic_cmpset_int"
	: "=q" (res),			/* 0 */
	  "+m" (*dst),			/* 1 */
	  "+a" (expect)			/* 2 */
	: "r" (src)			/* 3 */
	: "memory", "cc");
	return (res);
}

static __inline int
atomic_cmpset_long(volatile u_long *dst, u_long expect, u_long src)
{
	u_char res;

	__asm __volatile(
	"	" MPLOCKED "		"
	"	cmpxchgq %3,%1 ;	"
	"       sete	%0 ;		"
	"# atomic_cmpset_long"
	: "=q" (res),			/* 0 */
	  "+m" (*dst),			/* 1 */
	  "+a" (expect)			/* 2 */
	: "r" (src)			/* 3 */
	: "memory", "cc");
	return (res);
}

/*
 * Atomically add the value of v to the integer pointed to by p and return
 * the previous value of *p.
 */
static __inline u_int
atomic_fetchadd_int(volatile u_int *p, u_int v)
{

	__asm __volatile(
	"	" MPLOCKED "		"
	"	xaddl	%0,%1 ;		"
	"# atomic_fetchadd_int"
	: "+r" (v),			/* 0 */
	  "+m" (*p)			/* 1 */
	: : "cc");
	return (v);
}

/*
 * Atomically add the value of v to the long integer pointed to by p and return
 * the previous value of *p.
 */
static __inline u_long
atomic_fetchadd_long(volatile u_long *p, u_long v)
{

	__asm __volatile(
	"	" MPLOCKED "		"
	"	xaddq	%0,%1 ;		"
	"# atomic_fetchadd_long"
	: "+r" (v),			/* 0 */
	  "+m" (*p)			/* 1 */
	: : "cc");
	return (v);
}

static __inline int
atomic_testandset_int(volatile u_int *p, u_int v)
{
	u_char res;

	__asm __volatile(
	"	" MPLOCKED "		"
	"	btsl	%2,%1 ;		"
	"	setc	%0 ;		"
	"# atomic_testandset_int"
	: "=q" (res),			/* 0 */
	  "+m" (*p)			/* 1 */
	: "Ir" (v & 0x1f)		/* 2 */
	: "cc");
	return (res);
}

static __inline int
atomic_testandset_long(volatile u_long *p, u_int v)
{
	u_char res;

	__asm __volatile(
	"	" MPLOCKED "		"
	"	btsq	%2,%1 ;		"
	"	setc	%0 ;		"
	"# atomic_testandset_long"
	: "=q" (res),			/* 0 */
	  "+m" (*p)			/* 1 */
	: "Jr" ((u_long)(v & 0x3f))	/* 2 */
	: "cc");
	return (res);
}

/*
 * We assume that a = b will do atomic loads and stores.  Due to the
 * IA32 memory model, a simple store guarantees release semantics.
 *
 * However, loads may pass stores, so for atomic_load_acq we have to
 * ensure a Store/Load barrier to do the load in SMP kernels.  We use
 * "lock cmpxchg" as recommended by the AMD Software Optimization
 * Guide, and not mfence.  For UP kernels, however, the cache of the
 * single processor is always consistent, so we only need to take care
 * of the compiler.
 */
#define	ATOMIC_STORE(TYPE)				\
static __inline void					\
atomic_store_rel_##TYPE(volatile u_##TYPE *p, u_##TYPE v)\
{							\
	__compiler_membar();				\
	*p = v;						\
}							\
struct __hack

#define	ATOMIC_LOAD(TYPE, LOP)				\
static __inline u_##TYPE				\
atomic_load_acq_##TYPE(volatile u_##TYPE *p)		\
{							\
	u_##TYPE res;					\
							\
	__asm __volatile(MPLOCKED LOP			\
	: "=a" (res),			/* 0 */		\
	  "+m" (*p)			/* 1 */		\
	: : "memory", "cc");				\
	return (res);					\
}							\
struct __hack

ATOMIC_ASM(set,	     char,  "orb %b1,%0",  "iq",  v);
ATOMIC_ASM(clear,    char,  "andb %b1,%0", "iq", ~v);
ATOMIC_ASM(add,	     char,  "addb %b1,%0", "iq",  v);
ATOMIC_ASM(subtract, char,  "subb %b1,%0", "iq",  v);

ATOMIC_ASM(set,	     short, "orw %w1,%0",  "ir",  v);
ATOMIC_ASM(clear,    short, "andw %w1,%0", "ir", ~v);
ATOMIC_ASM(add,	     short, "addw %w1,%0", "ir",  v);
ATOMIC_ASM(subtract, short, "subw %w1,%0", "ir",  v);

ATOMIC_ASM(set,	     int,   "orl %1,%0",   "ir",  v);
ATOMIC_ASM(clear,    int,   "andl %1,%0",  "ir", ~v);
ATOMIC_ASM(add,	     int,   "addl %1,%0",  "ir",  v);
ATOMIC_ASM(subtract, int,   "subl %1,%0",  "ir",  v);

ATOMIC_ASM(set,	     long,  "orq %1,%0",   "ir",  v);
ATOMIC_ASM(clear,    long,  "andq %1,%0",  "ir", ~v);
ATOMIC_ASM(add,	     long,  "addq %1,%0",  "ir",  v);
ATOMIC_ASM(subtract, long,  "subq %1,%0",  "ir",  v);

ATOMIC_LOAD(char,  "cmpxchgb %b0,%1");
ATOMIC_LOAD(short, "cmpxchgw %w0,%1");
ATOMIC_LOAD(int,   "cmpxchgl %0,%1");
ATOMIC_LOAD(long,  "cmpxchgq %0,%1");

ATOMIC_STORE(char);
ATOMIC_STORE(short);
ATOMIC_STORE(int);
ATOMIC_STORE(long);

#undef ATOMIC_ASM
#undef ATOMIC_LOAD
#undef ATOMIC_STORE

/* Read the current value and store a new value in the destination. */

static __inline u_int
atomic_swap_int(volatile u_int *p, u_int v)
{

	__asm __volatile(
	"	xchgl	%1,%0 ;		"
	"# atomic_swap_int"
	: "+r" (v),			/* 0 */
	  "+m" (*p));			/* 1 */
	return (v);
}

static __inline u_long
atomic_swap_long(volatile u_long *p, u_long v)
{

	__asm __volatile(
	"	xchgq	%1,%0 ;		"
	"# atomic_swap_long"
	: "+r" (v),			/* 0 */
	  "+m" (*p));			/* 1 */
	return (v);
}

#define	atomic_set_acq_char		atomic_set_barr_char
#define	atomic_set_rel_char		atomic_set_barr_char
#define	atomic_clear_acq_char		atomic_clear_barr_char
#define	atomic_clear_rel_char		atomic_clear_barr_char
#define	atomic_add_acq_char		atomic_add_barr_char
#define	atomic_add_rel_char		atomic_add_barr_char
#define	atomic_subtract_acq_char	atomic_subtract_barr_char
#define	atomic_subtract_rel_char	atomic_subtract_barr_char

#define	atomic_set_acq_short		atomic_set_barr_short
#define	atomic_set_rel_short		atomic_set_barr_short
#define	atomic_clear_acq_short		atomic_clear_barr_short
#define	atomic_clear_rel_short		atomic_clear_barr_short
#define	atomic_add_acq_short		atomic_add_barr_short
#define	atomic_add_rel_short		atomic_add_barr_short
#define	atomic_subtract_acq_short	atomic_subtract_barr_short
#define	atomic_subtract_rel_short	atomic_subtract_barr_short

#define	atomic_set_acq_int		atomic_set_barr_int
#define	atomic_set_rel_int		atomic_set_barr_int
#define	atomic_clear_acq_int		atomic_clear_barr_int
#define	atomic_clear_rel_int		atomic_clear_barr_int
#define	atomic_add_acq_int		atomic_add_barr_int
#define	atomic_add_rel_int		atomic_add_barr_int
#define	atomic_subtract_acq_int		atomic_subtract_barr_int
#define	atomic_subtract_rel_int		atomic_subtract_barr_int
#define	atomic_cmpset_acq_int		atomic_cmpset_int
#define	atomic_cmpset_rel_int		atomic_cmpset_int

#define	atomic_set_acq_long		atomic_set_barr_long
#define	atomic_set_rel_long		atomic_set_barr_long
#define	atomic_clear_acq_long		atomic_clear_barr_long
#define	atomic_clear_rel_long		atomic_clear_barr_long
#define	atomic_add_acq_long		atomic_add_barr_long
#define	atomic_add_rel_long		atomic_add_barr_long
#define	atomic_subtract_acq_long	atomic_subtract_barr_long
#define	atomic_subtract_rel_long	atomic_subtract_barr_long
#define	atomic_cmpset_acq_long		atomic_cmpset_long
#define	atomic_cmpset_rel_long		atomic_cmpset_long

#define	atomic_readandclear_int(p)	atomic_swap_int(p, 0)
#define	atomic_readandclear_long(p)	atomic_swap_long(p, 0)

/* Operations on 8-bit bytes. */
#define	atomic_set_8		atomic_set_char
#define	atomic_set_acq_8	atomic_set_acq_char
#define	atomic_set_rel_8	atomic_set_rel_char
#define	atomic_clear_8		atomic_clear_char
#define	atomic_clear_acq_8	atomic_clear_acq_char
#define	atomic_clear_rel_8	atomic_clear_rel_char
#define	atomic_add_8		atomic_add_char
#define	atomic_add_acq_8	atomic_add_acq_char
#define	atomic_add_rel_8	atomic_add_rel_char
#define	atomic_subtract_8	atomic_subtract_char
#define	atomic_subtract_acq_8	atomic_subtract_acq_char
#define	atomic_subtract_rel_8	atomic_subtract_rel_char
#define	atomic_load_acq_8	atomic_load_acq_char
#define	atomic_store_rel_8	atomic_store_rel_char

/* Operations on 16-bit words. */
#define	atomic_set_16		atomic_set_short
#define	atomic_set_acq_16	atomic_set_acq_short
#define	atomic_set_rel_16	atomic_set_rel_short
#define	atomic_clear_16		atomic_clear_short
#define	atomic_clear_acq_16	atomic_clear_acq_short
#define	atomic_clear_rel_16	atomic_clear_rel_short
#define	atomic_add_16		atomic_add_short
#define	atomic_add_acq_16	atomic_add_acq_short
#define	atomic_add_rel_16	atomic_add_rel_short
#define	atomic_subtract_16	atomic_subtract_short
#define	atomic_subtract_acq_16	atomic_subtract_acq_short
#define	atomic_subtract_rel_16	atomic_subtract_rel_short
#define	atomic_load_acq_16	atomic_load_acq_short
#define	atomic_store_rel_16	atomic_store_rel_short

/* Operations on 32-bit double words. */
#define	atomic_set_32		atomic_set_int
#define	atomic_set_acq_32	atomic_set_acq_int
#define	atomic_set_rel_32	atomic_set_rel_int
#define	atomic_clear_32		atomic_clear_int
#define	atomic_clear_acq_32	atomic_clear_acq_int
#define	atomic_clear_rel_32	atomic_clear_rel_int
#define	atomic_add_32		atomic_add_int
#define	atomic_add_acq_32	atomic_add_acq_int
#define	atomic_add_rel_32	atomic_add_rel_int
#define	atomic_subtract_32	atomic_subtract_int
#define	atomic_subtract_acq_32	atomic_subtract_acq_int
#define	atomic_subtract_rel_32	atomic_subtract_rel_int
#define	atomic_load_acq_32	atomic_load_acq_int
#define	atomic_store_rel_32	atomic_store_rel_int
#define	atomic_cmpset_32	atomic_cmpset_int
#define	atomic_cmpset_acq_32	atomic_cmpset_acq_int
#define	atomic_cmpset_rel_32	atomic_cmpset_rel_int
#define	atomic_swap_32		atomic_swap_int
#define	atomic_readandclear_32	atomic_readandclear_int
#define	atomic_fetchadd_32	atomic_fetchadd_int
#define	atomic_testandset_32	atomic_testandset_int

/* Operations on 64-bit quad words. */
#define	atomic_set_64		atomic_set_long
#define	atomic_set_acq_64	atomic_set_acq_long
#define	atomic_set_rel_64	atomic_set_rel_long
#define	atomic_clear_64		atomic_clear_long
#define	atomic_clear_acq_64	atomic_clear_acq_long
#define	atomic_clear_rel_64	atomic_clear_rel_long
#define	atomic_add_64		atomic_add_long
#define	atomic_add_acq_64	atomic_add_acq_long
#define	atomic_add_rel_64	atomic_add_rel_long
#define	atomic_subtract_64	atomic_subtract_long
#define	atomic_subtract_acq_64	atomic_subtract_acq_long
#define	atomic_subtract_rel_64	atomic_subtract_rel_long
#define	atomic_load_acq_64	atomic_load_acq_long
#define	atomic_store_rel_64	atomic_store_rel_long
#define	atomic_cmpset_64	atomic_cmpset_long
#define	atomic_cmpset_acq_64	atomic_cmpset_acq_long
#define	atomic_cmpset_rel_64	atomic_cmpset_rel_long
#define	atomic_swap_64		atomic_swap_long
#define	atomic_readandclear_64	atomic_readandclear_long
#define	atomic_testandset_64	atomic_testandset_long

/* Operations on pointers. */
#define	atomic_set_ptr		atomic_set_long
#define	atomic_set_acq_ptr	atomic_set_acq_long
#define	atomic_set_rel_ptr	atomic_set_rel_long
#define	atomic_clear_ptr	atomic_clear_long
#define	atomic_clear_acq_ptr	atomic_clear_acq_long
#define	atomic_clear_rel_ptr	atomic_clear_rel_long
#define	atomic_add_ptr		atomic_add_long
#define	atomic_add_acq_ptr	atomic_add_acq_long
#define	atomic_add_rel_ptr	atomic_add_rel_long
#define	atomic_subtract_ptr	atomic_subtract_long
#define	atomic_subtract_acq_ptr	atomic_subtract_acq_long
#define	atomic_subtract_rel_ptr	atomic_subtract_rel_long
#define	atomic_load_acq_ptr	atomic_load_acq_long
#define	atomic_store_rel_ptr	atomic_store_rel_long
#define	atomic_cmpset_ptr	atomic_cmpset_long
#define	atomic_cmpset_acq_ptr	atomic_cmpset_acq_long
#define	atomic_cmpset_rel_ptr	atomic_cmpset_rel_long
#define	atomic_swap_ptr		atomic_swap_long
#define	atomic_readandclear_ptr	atomic_readandclear_long
