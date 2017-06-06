/*
 * C interface to the mirage-block subsystem
 *
 * Rules for usage:
 * - do not mix with other libraries which embed OCaml runtimes
 * - before calling any other function, call `mirage_block_init` from one
 *   thread: this initialises the runtime
 * - before calling open, register your thread with `mirage_block_register_thread`
 */

#include <sys/types.h>
#include <sys/uio.h>
#include <unistd.h>

/* Initialise the mirage-block subsystem. This must be called before calling
   mirage_block_register_thread. */
extern void
mirage_block_init(void);

/* Every thread that uses a mirage-block device must be registered with the
   runtime system. Call this on every thread you will use, after calling
	 mirage_block_init once in one thread. */
extern void
mirage_block_register_thread(void);

/* When a thread has finished using mirage-block devices, call this to free
   associated resources. */
extern void
mirage_block_unregister_thread(void);

/* An opened mirage-block device */
typedef int mirage_block_handle;

/* Open a mirage block device with the given optional string configuration.
   To use the default configuration, pass NULL for options. */
extern mirage_block_handle
mirage_block_open(const char *config, const char *options);

struct mirage_block_stat {
	int candelete;      /* 1 if the device supports TRIM/DELETE/DISCARD */
};

/* Query a mirage block device. */
extern int
mirage_block_stat(mirage_block_handle h, struct stat *stat, struct mirage_block_stat *buf);

/* Read data from a mirage block device. Note the offset must be sector-aligned
   and the memory buffers must also be sector-aligned. */
extern ssize_t
mirage_block_preadv(mirage_block_handle h,
	const struct iovec *iov, int iovcnt, off_t offset);

/* Write data to a mirage block device. Note the offset must be sector-aligned
   and the memory buffers must also be sector-aligned. */
extern ssize_t
mirage_block_pwritev(mirage_block_handle h,
	const struct iovec *iov, int iovcnt, off_t offset);

/* TRIM/DELETE/DISCARD the range of sectors */
extern int
mirage_block_delete(mirage_block_handle h, off_t offset, ssize_t len);

/* Flush any outstanding I/O */
extern
int mirage_block_flush(mirage_block_handle h);

/* Close an open device; subsequent I/O requests will fail. */
extern
int mirage_block_close(mirage_block_handle h);
