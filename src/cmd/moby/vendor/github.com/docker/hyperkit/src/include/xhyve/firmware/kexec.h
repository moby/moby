#pragma once

#include <stdint.h>

struct setup_header {
	uint8_t setup_sects; /* The size of the setup in sectors */
	uint16_t root_flags; /* If set, the root is mounted readonly */
	uint32_t syssize; /* The size of the 32-bit code in 16-byte paras */
	uint16_t ram_size; /* DO NOT USE - for bootsect.S use only */
	uint16_t vid_mode; /* Video mode control */
	uint16_t root_dev; /* Default root device number */
	uint16_t boot_flag; /* 0xAA55 magic number */
	uint16_t jump; /* Jump instruction */
	uint32_t header; /* Magic signature "HdrS" */
	uint16_t version; /* Boot protocol version supported */
	uint32_t realmode_swtch; /* Boot loader hook (see below) */
	uint16_t start_sys_seg; /* The load-low segment (0x1000) (obsolete) */
	uint16_t kernel_version; /* Pointer to kernel version string */
	uint8_t type_of_loader; /* Boot loader identifier */
	uint8_t loadflags; /* Boot protocol option flags */
	uint16_t setup_move_size; /* Move to high memory size (used with hooks) */
	uint32_t code32_start; /* Boot loader hook (see below) */
	uint32_t ramdisk_image; /* initrd load address (set by boot loader) */
	uint32_t ramdisk_size; /* initrd size (set by boot loader) */
	uint32_t bootsect_kludge; /* DO NOT USE - for bootsect.S use only */
	uint16_t heap_end_ptr; /* Free memory after setup end */
	uint8_t ext_loader_ver; /* Extended boot loader version */
	uint8_t ext_loader_type; /* Extended boot loader ID */
	uint32_t cmd_line_ptr; /* 32-bit pointer to the kernel command line */
	uint32_t initrd_addr_max; /* Highest legal initrd address */
	uint32_t kernel_alignment; /* Physical addr alignment required for kernel */
	uint8_t relocatable_kernel; /* Whether kernel is relocatable or not */
	uint8_t min_alignment; /* Minimum alignment, as a power of two */
	uint16_t xloadflags; /* Boot protocol option flags */
	uint32_t cmdline_size; /* Maximum size of the kernel command line */
	uint32_t hardware_subarch; /* Hardware subarchitecture */
	uint64_t hardware_subarch_data; /* Subarchitecture-specific data */
	uint32_t payload_offset; /* Offset of kernel payload */
	uint32_t payload_length; /* Length of kernel payload */
	uint64_t setup_data; /* 64bit pointer to linked list of struct setup_data */
	uint64_t pref_address; /* Preferred loading address */
	uint32_t init_size; /* Linear memory required during initialization */
	uint32_t handover_offset; /* Offset of handover entry point */
} __attribute__((packed));

struct zero_page {
	uint8_t screen_info[64];
	uint8_t apm_bios_info[20];
	uint8_t _0[4];
	uint64_t tboot_addr;
	uint8_t ist_info[16];
	uint8_t _1[16];
	uint8_t hd0_info[16];
	uint8_t hd1_info[16];
	uint8_t sys_desc_table[16];
	uint8_t olpc_ofw_header[16];
	uint32_t ext_ramdisk_image;
	uint32_t ext_ramdisk_size;
	uint32_t ext_cmd_line_ptr;
	uint8_t _2[116];
	uint8_t edid_info[128];
	uint8_t efi_info[32];
	uint32_t alt_mem_k;
	uint32_t scratch;
	uint8_t e820_entries;
	uint8_t eddbuf_entries;
	uint8_t edd_mbr_sig_buf_entries;
	uint8_t kbd_status;
	uint8_t _3[3];
	uint8_t sentinel;
	uint8_t _4[1];
	struct setup_header setup_header;
	uint8_t _5[(0x290 - 0x1f1 - sizeof(struct setup_header))];
	uint32_t edd_mbr_sig_buffer[16];
	struct {
		uint64_t addr;
		uint64_t size;
		uint32_t type;
	} __attribute__((packed)) e820_map[128];
	uint8_t _6[48];
	uint8_t eddbuf[492];
	uint8_t _7[276];
} __attribute__((packed));

void kexec_init(char *kernel_path, char *initrd_path, char *cmdline);
uint64_t kexec(void);
