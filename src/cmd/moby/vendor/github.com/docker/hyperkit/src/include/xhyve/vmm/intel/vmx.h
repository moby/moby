/*-
 * Copyright (c) 2011 NetApp, Inc.
 * Copyright (c) 2015 xhyve developers
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

#include <stddef.h>
#include <xhyve/support/misc.h>
#include <xhyve/vmm/vmm.h>
#include <xhyve/vmm/intel/vmcs.h>

struct vmxcap {
	int	set;
	uint32_t proc_ctls;
	uint32_t proc_ctls2;
};

struct vmxstate {
	uint64_t nextrip;	/* next instruction to be executed by guest */
	int	lastcpu;	/* host cpu that this 'vcpu' last ran on */
	uint16_t vpid;
};

struct apic_page {
	uint32_t reg[XHYVE_PAGE_SIZE / 4];
};
CTASSERT(sizeof(struct apic_page) == XHYVE_PAGE_SIZE);

/* Posted Interrupt Descriptor (described in section 29.6 of the Intel SDM) */
struct pir_desc {
	uint64_t	pir[4];
	uint64_t	pending;
	uint64_t	unused[3];
} __aligned(64);
CTASSERT(sizeof(struct pir_desc) == 64);

/* Index into the 'guest_msrs[]' array */
enum {
	IDX_MSR_LSTAR,
	IDX_MSR_CSTAR,
	IDX_MSR_STAR,
	IDX_MSR_SF_MASK,
	IDX_MSR_KGSBASE,
	IDX_MSR_PAT,
	GUEST_MSR_NUM		/* must be the last enumeration */
};

/* virtual machine softc */
struct vmx {
	struct apic_page apic_page[VM_MAXCPU]; /* one apic page per vcpu */
	uint64_t guest_msrs[VM_MAXCPU][GUEST_MSR_NUM];
	struct vmxcap cap[VM_MAXCPU];
	struct vmxstate state[VM_MAXCPU];
	struct vm *vm;
};

#define	VMX_GUEST_VMEXIT	0
#define	VMX_VMRESUME_ERROR	1
#define	VMX_VMLAUNCH_ERROR	2
#define	VMX_INVEPT_ERROR	3
void	vmx_call_isr(uintptr_t entry);

u_long	vmx_fix_cr0(u_long cr0);
u_long	vmx_fix_cr4(u_long cr4);

extern char	vmx_exit_guest[];
