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
#include <xhyve/support/segments.h>

#ifndef CTASSERT /* Allow lint to override */
#define	CTASSERT(x) _CTASSERT(x, __LINE__)
#define	_CTASSERT(x, y) __CTASSERT(x, y)
#define	__CTASSERT(x, y) typedef char __assert ## y[(x) ? 1 : -1]
#endif

#define	VMEXIT_CONTINUE (0)
#define	VMEXIT_ABORT (-1)

extern int guest_ncpus;
extern char *guest_uuid_str;
extern char *vmname;

void xh_vm_inject_fault(int vcpu, int vector, int errcode_valid,
    uint32_t errcode);

static __inline void
vm_inject_ud(int vcpuid)
{
	xh_vm_inject_fault(vcpuid, IDT_UD, 0, 0);
}

static __inline void
vm_inject_gp(int vcpuid)
{
	xh_vm_inject_fault(vcpuid, IDT_GP, 1, 0);
}

static __inline void
vm_inject_ac(int vcpuid, uint32_t errcode)
{
	xh_vm_inject_fault(vcpuid, IDT_AC, 1, errcode);
}

static __inline void
vm_inject_ss(int vcpuid, uint32_t errcode)
{
	xh_vm_inject_fault(vcpuid, IDT_SS, 1, errcode);
}

void *paddr_guest2host(uintptr_t addr, size_t len);

void vcpu_set_capabilities(int cpu);
void vcpu_add(int fromcpu, int newcpu, uint64_t rip);
int fbsdrun_vmexit_on_hlt(void);
int fbsdrun_vmexit_on_pause(void);
int fbsdrun_virtio_msix(void);
