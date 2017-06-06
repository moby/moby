#pragma once

#include <stdint.h>

/*
 * USERBOOT interface versions
 */
#define USERBOOT_VERSION_1 1
#define USERBOOT_VERSION_2 2
#define USERBOOT_VERSION_3 3

/*
 * Exit codes from the loader
 */
#define USERBOOT_EXIT_QUIT 1
#define USERBOOT_EXIT_REBOOT 2

struct loader_callbacks {
	/* Console i/o */

	/* Wait until a key is pressed on the console and then return it */
	int (*getc)(void *arg);
	/* Write the character ch to the console */
	void (*putc)(void *arg, int ch);
	/* Return non-zero if a key can be read from the console */
	int (*poll)(void *arg);

	/* Host filesystem i/o */

	/* Open a file in the host filesystem */
	int (*open)(void *arg, const char *filename, void **h_return);
	/* Close a file */
	int	(*close)(void *arg, void *h);
	/* Return non-zero if the file is a directory */
	int (*isdir)(void *arg, void *h);
	/* Read size bytes from a file. The number of bytes remaining in dst after
	 * reading is returned in *resid_return
	 */
	int (*read)(void *arg, void *h, void *dst, size_t size,
		size_t *resid_return);
	/* Read an entry from a directory. The entry's inode number is returned in
	 * fileno_return, its type in *type_return and the name length in
	 * *namelen_return. The name itself is copied to the buffer name which must
	 * be at least PATH_MAX in size.
	 */
	int (*readdir)(void *arg, void *h, uint32_t *fileno_return,
		uint8_t *type_return, size_t *namelen_return, char *name);
	/* Seek to a location within an open file */
	int (*seek)(void *arg, void *h, uint64_t offset, int whence);
	/* Return some stat(2) related information about the file */
	int (*stat)(void *arg, void *h, int *mode_return, int *uid_return,
		int *gid_return, uint64_t *size_return);

	/* Disk image i/o */

	/* Read from a disk image at the given offset */
	int	(*diskread)(void *arg, int unit, uint64_t offset, void *dst,
		size_t size, size_t *resid_return);

	/* Guest virtual machine i/o */

	/* Copy to the guest address space */
	int (*copyin)(void *arg, const void *from, uint64_t to, size_t size);
	/* Copy from the guest address space */
	int (*copyout)(void *arg, uint64_t from, void *to, size_t size);
	/* Set a guest register value */
	void (*setreg)(void *arg, int, uint64_t);
	/* Set a guest MSR value */
	void (*setmsr)(void *arg, int, uint64_t);
	/* Set a guest CR value */
	void (*setcr)(void *arg, int, uint64_t);
	/* Set the guest GDT address */
	void (*setgdt)(void *arg, uint64_t, size_t);
	/* Transfer control to the guest at the given address */
	void (*exec)(void *arg, uint64_t pc);

	/* Misc */

	/* Sleep for usec microseconds */
	void (*delay)(void *arg, int usec);
	/* Exit with the given exit code */
	void (*exit)(void);
	/* Return guest physical memory map details */
	void (*getmem)(void *arg, uint64_t *lowmem, uint64_t *highmem);
	/* ioctl interface to the disk device */
	int (*diskioctl)(void *arg, int unit, u_long cmd, void *data);
	/*
	 * Returns an environment variable in the form "name=value".
	 *
	 * If there are no more variables that need to be set in the
	 * loader environment then return NULL.
	 *
	 * 'num' is used as a handle for the callback to identify which
	 * environment variable to return next. It will begin at 0 and
	 * each invocation will add 1 to the previous value of 'num'.
	 */
	const char * (*getenv)(void *arg, int num);
};

void fbsd_init(char *userboot_path, char *bootvolume_path, char *kernelenv,
	char *cons);
uint64_t fbsd_load(void);
