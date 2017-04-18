#pragma once

#include <assert.h>
#include <pthread.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#define UNUSED __attribute__ ((unused))
#define CTASSERT(x) _Static_assert ((x), "CTASSERT")
#define XHYVE_PAGE_SIZE 0x1000
#define XHYVE_PAGE_MASK (XHYVE_PAGE_SIZE - 1)
#define XHYVE_PAGE_SHIFT 12
#define __aligned(x) __attribute__ ((aligned ((x))))
#define __packed __attribute__ ((packed))
#define nitems(x) (sizeof((x)) / sizeof((x)[0]))
#define powerof2(x)	((((x)-1)&(x))==0)
#define roundup2(x, y) (((x)+((y)-1))&(~((y)-1))) /* if y is powers of two */
#define nitems(x) (sizeof((x)) / sizeof((x)[0]))
#define min(x, y) (((x) < (y)) ? (x) : (y))

#define xhyve_abort(...) \
	do { \
		fprintf(stderr, __VA_ARGS__); \
		abort(); \
	} while (0)

#define xhyve_warn(...) \
	do { \
		fprintf(stderr, __VA_ARGS__); \
	} while (0)

#ifdef XHYVE_CONFIG_ASSERT
#define KASSERT(exp, msg) if (!(exp)) xhyve_abort msg
#define KWARN(exp, msg) if (!(exp)) xhyve_warn msg
#else
#define KASSERT(exp, msg) if (0) xhyve_abort msg
#define KWARN(exp, msg) if (0) xhyve_warn msg
#endif

#define FALSE 0
#define TRUE 1

#define XHYVE_PROT_READ 1
#define XHYVE_PROT_WRITE 2
#define XHYVE_PROT_EXECUTE 4

#define	VM_SUCCESS 0

/* sys/sys/types.h */
typedef	unsigned char u_char;
typedef	unsigned short u_short;
typedef	unsigned int u_int;
typedef	unsigned long u_long;

static inline void cpuid_count(uint32_t ax, uint32_t cx, uint32_t *p) {
	__asm__ __volatile__ ("cpuid"
		: "=a" (p[0]), "=b" (p[1]), "=c" (p[2]), "=d" (p[3])
		:  "0" (ax), "c" (cx));
}

static inline void do_cpuid(unsigned ax, unsigned *p) {
	__asm__ __volatile__ ("cpuid"
		: "=a" (p[0]), "=b" (p[1]), "=c" (p[2]), "=d" (p[3])
		:  "0" (ax));
}

/* Used to trigger a self-shutdown */
extern void push_power_button(void);

/* Error checking pthread mutex operations */
static inline void xpthread_mutex_init(pthread_mutex_t *mutex)
{
	int rc = pthread_mutex_init(mutex, NULL);
	if (__builtin_expect(rc != 0, 0))
		xhyve_abort("pthread_mutex_init failed: %d: %s\n",
			    rc, strerror(rc));
}

static inline void xpthread_mutex_lock(pthread_mutex_t *mutex)
{
	int rc = pthread_mutex_lock(mutex);
	if (__builtin_expect(rc != 0, 0))
		xhyve_abort("pthread_mutex_lock failed: %d: %s\n",
			    rc, strerror(rc));
}
static inline void xpthread_mutex_unlock(pthread_mutex_t *mutex)
{
	int rc = pthread_mutex_unlock(mutex);
	if (__builtin_expect(rc != 0, 0))
		xhyve_abort("pthread_mutex_unlock failed: %d: %s\n",
			    rc, strerror(rc));
}
