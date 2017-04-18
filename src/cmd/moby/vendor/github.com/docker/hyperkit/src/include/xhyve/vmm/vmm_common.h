/*-
 * Copyright (c) 2011 NetApp, Inc.
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
 * THIS SOFTWARE IS PROVIDED BY NETAPP, INC ``AS IS'' AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED.  IN NO EVENT SHALL NETAPP, INC OR CONTRIBUTORS BE LIABLE
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

#define	VM_MAXCPU 64 /* maximum virtual cpus */

enum vm_suspend_how {
	VM_SUSPEND_NONE,
	VM_SUSPEND_RESET,
	VM_SUSPEND_POWEROFF,
	VM_SUSPEND_HALT,
	VM_SUSPEND_TRIPLEFAULT,
	VM_SUSPEND_LAST
};

enum vm_cap_type {
	VM_CAP_HALT_EXIT,
	VM_CAP_MTRAP_EXIT,
	VM_CAP_PAUSE_EXIT,
	VM_CAP_MAX
};

enum vm_intr_trigger {
	EDGE_TRIGGER,
	LEVEL_TRIGGER
};

enum x2apic_state {
	X2APIC_DISABLED,
	X2APIC_ENABLED,
	X2APIC_STATE_LAST
};

enum vm_cpu_mode {
	CPU_MODE_REAL,
	CPU_MODE_PROTECTED,
	CPU_MODE_COMPATIBILITY, /* IA-32E mode (CS.L = 0) */
	CPU_MODE_64BIT, /* IA-32E mode (CS.L = 1) */
};

enum vm_paging_mode {
	PAGING_MODE_FLAT,
	PAGING_MODE_32,
	PAGING_MODE_PAE,
	PAGING_MODE_64,
};

struct seg_desc {
	uint64_t	base;
	uint32_t	limit;
	uint32_t	access;
};

#define	SEG_DESC_TYPE(access) ((access) & 0x001f)
#define	SEG_DESC_DPL(access) (((access) >> 5) & 0x3)
#define	SEG_DESC_PRESENT(access) (((access) & 0x0080) ? 1 : 0)
#define	SEG_DESC_DEF32(access) (((access) & 0x4000) ? 1 : 0)
#define	SEG_DESC_GRANULARITY(access) (((access) & 0x8000) ? 1 : 0)
#define	SEG_DESC_UNUSABLE(access) (((access) & 0x10000) ? 1 : 0)

struct vm_guest_paging {
	uint64_t cr3;
	int cpl;
	enum vm_cpu_mode cpu_mode;
	enum vm_paging_mode paging_mode;
};

enum vm_reg_name {
	VM_REG_GUEST_RAX,
	VM_REG_GUEST_RBX,
	VM_REG_GUEST_RCX,
	VM_REG_GUEST_RDX,
	VM_REG_GUEST_RSI,
	VM_REG_GUEST_RDI,
	VM_REG_GUEST_RBP,
	VM_REG_GUEST_R8,
	VM_REG_GUEST_R9,
	VM_REG_GUEST_R10,
	VM_REG_GUEST_R11,
	VM_REG_GUEST_R12,
	VM_REG_GUEST_R13,
	VM_REG_GUEST_R14,
	VM_REG_GUEST_R15,
	VM_REG_GUEST_CR0,
	VM_REG_GUEST_CR3,
	VM_REG_GUEST_CR4,
	VM_REG_GUEST_DR7,
	VM_REG_GUEST_RSP,
	VM_REG_GUEST_RIP,
	VM_REG_GUEST_RFLAGS,
	VM_REG_GUEST_ES,
	VM_REG_GUEST_CS,
	VM_REG_GUEST_SS,
	VM_REG_GUEST_DS,
	VM_REG_GUEST_FS,
	VM_REG_GUEST_GS,
	VM_REG_GUEST_LDTR,
	VM_REG_GUEST_TR,
	VM_REG_GUEST_IDTR,
	VM_REG_GUEST_GDTR,
	VM_REG_GUEST_EFER,
	VM_REG_GUEST_CR2,
	VM_REG_GUEST_PDPTE0,
	VM_REG_GUEST_PDPTE1,
	VM_REG_GUEST_PDPTE2,
	VM_REG_GUEST_PDPTE3,
	VM_REG_GUEST_INTR_SHADOW,
	VM_REG_LAST
};

enum vm_exitcode {
	VM_EXITCODE_INOUT,
	VM_EXITCODE_VMX,
	VM_EXITCODE_BOGUS,
	VM_EXITCODE_RDMSR,
	VM_EXITCODE_WRMSR,
	VM_EXITCODE_HLT,
	VM_EXITCODE_MTRAP,
	VM_EXITCODE_PAUSE,
	VM_EXITCODE_PAGING,
	VM_EXITCODE_INST_EMUL,
	VM_EXITCODE_SPINUP_AP,
	VM_EXITCODE_DEPRECATED1,	/* used to be SPINDOWN_CPU */
	VM_EXITCODE_RENDEZVOUS,
	VM_EXITCODE_IOAPIC_EOI,
	VM_EXITCODE_SUSPENDED,
	VM_EXITCODE_INOUT_STR,
	VM_EXITCODE_TASK_SWITCH,
	VM_EXITCODE_MONITOR,
	VM_EXITCODE_MWAIT,
	VM_EXITCODE_MAX
};

struct vm_inout {
	uint16_t bytes:3; /* 1 or 2 or 4 */
	uint16_t in:1;
	uint16_t string:1;
	uint16_t rep:1;
	uint16_t port;
	uint32_t eax; /* valid for out */
};

struct vm_inout_str {
	struct vm_inout inout; /* must be the first element */
	struct vm_guest_paging paging;
	uint64_t rflags;
	uint64_t cr0;
	uint64_t index;
	uint64_t count; /* rep=1 (%rcx), rep=0 (1) */
	int addrsize;
	enum vm_reg_name seg_name;
	struct seg_desc seg_desc;
};

struct vie_op {
	uint8_t op_byte; /* actual opcode byte */
	uint8_t op_type; /* type of operation (e.g. MOV) */
	uint16_t op_flags;
};

#define	VIE_INST_SIZE	15
struct vie {
	uint8_t inst[VIE_INST_SIZE]; /* instruction bytes */
	uint8_t num_valid; /* size of the instruction */
	uint8_t num_processed;
	uint8_t addrsize:4, opsize:4; /* address and operand sizes */
	uint8_t rex_w:1, /* REX prefix */
			rex_r:1,
			rex_x:1,
			rex_b:1,
			rex_present:1,
			repz_present:1, /* REP/REPE/REPZ prefix */
			repnz_present:1, /* REPNE/REPNZ prefix */
			opsize_override:1, /* Operand size override */
			addrsize_override:1, /* Address size override */
			segment_override:1; /* Segment override */
	uint8_t mod:2, /* ModRM byte */
			reg:4,
			rm:4;
	uint8_t ss:2, /* SIB byte */
			index:4,
			base:4;
	uint8_t disp_bytes;
	uint8_t imm_bytes;
	uint8_t scale;
	int base_register; /* VM_REG_GUEST_xyz */
	int index_register; /* VM_REG_GUEST_xyz */
	int segment_register; /* VM_REG_GUEST_xyz */
	int64_t displacement; /* optional addr displacement */
	int64_t immediate; /* optional immediate operand */
	uint8_t decoded; /* set to 1 if successfully decoded */
	struct vie_op op; /* opcode description */
};

enum task_switch_reason {
	TSR_CALL,
	TSR_IRET,
	TSR_JMP,
	TSR_IDT_GATE /* task gate in IDT */
};

struct vm_task_switch {
	uint16_t tsssel; /* new TSS selector */
	int ext; /* task switch due to external event */
	uint32_t errcode;
	int errcode_valid; /* push 'errcode' on the new stack */
	enum task_switch_reason reason;
	struct vm_guest_paging paging;
};

struct vm_exit {
	enum vm_exitcode exitcode;
	int inst_length; /* 0 means unknown */
	uint64_t rip;
	union {
		struct vm_inout inout;
		struct vm_inout_str inout_str;
		struct {
			uint64_t gpa;
			int fault_type;
		} paging;
		struct {
			uint64_t gpa;
			uint64_t gla;
			uint64_t cs_base;
			int cs_d; /* CS.D */
			struct vm_guest_paging paging;
			struct vie vie;
		} inst_emul;
		/*
		 * VMX specific payload. Used when there is no "better"
		 * exitcode to represent the VM-exit.
		 */
		struct {
			int status; /* vmx inst status */
			/*
			 * 'exit_reason' and 'exit_qualification' are valid
			 * only if 'status' is zero.
			 */
			uint32_t exit_reason;
			uint64_t exit_qualification;
			/*
			 * 'inst_error' and 'inst_type' are valid
			 * only if 'status' is non-zero.
			 */
			int inst_type;
			int inst_error;
		} vmx;
		struct {
			uint32_t code; /* ecx value */
			uint64_t wval;
		} msr;
		struct {
			int vcpu;
			uint64_t rip;
		} spinup_ap;
		struct {
			uint64_t rflags;
		} hlt;
		struct {
			int vector;
		} ioapic_eoi;
		struct {
			enum vm_suspend_how how;
		} suspended;
		struct vm_task_switch task_switch;
	} u;
};

/* FIXME remove */
struct vm_memory_segment {
	uint64_t gpa; /* in */
	size_t len;
};

typedef int (*mem_region_read_t)(void *vm, int cpuid, uint64_t gpa,
	uint64_t *rval, int rsize, void *arg);

typedef int (*mem_region_write_t)(void *vm, int cpuid, uint64_t gpa,
	uint64_t wval, int wsize, void *arg);

uint64_t vie_size2mask(int size);

int vie_calculate_gla(enum vm_cpu_mode cpu_mode, enum vm_reg_name seg,
	struct seg_desc *desc, uint64_t off, int length, int addrsize, int prot,
	uint64_t *gla);

int vie_alignment_check(int cpl, int operand_size, uint64_t cr0,
	uint64_t rflags, uint64_t gla);
