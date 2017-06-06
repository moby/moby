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
#include <sys/time.h>
#include <xhyve/support/cpuset.h>
#include <xhyve/vmm/vmm_common.h>

struct iovec;

/*
 * Different styles of mapping the memory assigned to a VM into the address
 * space of the controlling process.
 */
enum vm_mmap_style {
	VM_MMAP_NONE,		/* no mapping */
	VM_MMAP_ALL,		/* fully and statically mapped */
	VM_MMAP_SPARSE,		/* mappings created on-demand */
};

void xh_hv_pause(int pause);
int xh_vm_create(void);
void xh_vm_destroy(void);
int xh_vcpu_create(int vcpu);
void xh_vcpu_destroy(int vcpu);
int xh_vm_get_memory_seg(uint64_t gpa, size_t *ret_len);
int xh_vm_setup_memory(size_t len, enum vm_mmap_style vms);
void *xh_vm_map_gpa(uint64_t gpa, size_t len);
int xh_vm_gla2gpa(int vcpu, struct vm_guest_paging *paging, uint64_t gla,
	int prot, uint64_t *gpa, int *fault);
uint32_t xh_vm_get_lowmem_limit(void);
void xh_vm_set_lowmem_limit(uint32_t limit);
void xh_vm_set_memflags(int flags);
size_t xh_vm_get_lowmem_size(void);
size_t xh_vm_get_highmem_size(void);
int xh_vm_set_desc(int vcpu, int reg, uint64_t base, uint32_t limit,
	uint32_t access);
int xh_vm_get_desc(int vcpu, int reg, uint64_t *base, uint32_t *limit,
	uint32_t *access);
int xh_vm_get_seg_desc(int vcpu, int reg, struct seg_desc *seg_desc);
int xh_vm_set_register(int vcpu, int reg, uint64_t val);
int xh_vm_get_register(int vcpu, int reg, uint64_t *retval);
int xh_vm_run(int vcpu, struct vm_exit *ret_vmexit);
int xh_vm_suspend(enum vm_suspend_how how);
int xh_vm_reinit(void);
int xh_vm_apicid2vcpu(int apicid);
int xh_vm_inject_exception(int vcpu, int vector, int errcode_valid,
	uint32_t errcode, int restart_instruction);
int xh_vm_lapic_irq(int vcpu, int vector);
int xh_vm_lapic_local_irq(int vcpu, int vector);
int xh_vm_lapic_msi(uint64_t addr, uint64_t msg);
int xh_vm_ioapic_assert_irq(int irq);
int xh_vm_ioapic_deassert_irq(int irq);
int xh_vm_ioapic_pulse_irq(int irq);
int xh_vm_ioapic_pincount(int *pincount);
int xh_vm_isa_assert_irq(int atpic_irq, int ioapic_irq);
int xh_vm_isa_deassert_irq(int atpic_irq, int ioapic_irq);
int xh_vm_isa_pulse_irq(int atpic_irq, int ioapic_irq);
int xh_vm_isa_set_irq_trigger(int atpic_irq, enum vm_intr_trigger trigger);
int xh_vm_inject_nmi(int vcpu);
int xh_vm_capability_name2type(const char *capname);
const char *xh_vm_capability_type2name(int type);
int xh_vm_get_capability(int vcpu, enum vm_cap_type cap, int *retval);
int xh_vm_set_capability(int vcpu, enum vm_cap_type cap, int val);
int xh_vm_get_intinfo(int vcpu, uint64_t *i1, uint64_t *i2);
int xh_vm_set_intinfo(int vcpu, uint64_t exit_intinfo);
uint64_t *xh_vm_get_stats(int vcpu, struct timeval *ret_tv, int *ret_entries);
const char *xh_vm_get_stat_desc(int index);
int xh_vm_get_x2apic_state(int vcpu, enum x2apic_state *s);
int xh_vm_set_x2apic_state(int vcpu, enum x2apic_state s);
int xh_vm_get_hpet_capabilities(uint32_t *capabilities);
int xh_vm_copy_setup(int vcpu, struct vm_guest_paging *pg, uint64_t gla,
	size_t len, int prot, struct iovec *iov, int iovcnt, int *fault);
void xh_vm_copyin(struct iovec *iov, void *dst, size_t len);
void xh_vm_copyout(const void *src, struct iovec *iov, size_t len);
int xh_vm_rtc_write(int offset, uint8_t value);
int xh_vm_rtc_read(int offset, uint8_t *retval);
int xh_vm_rtc_settime(time_t secs);
int xh_vm_rtc_gettime(time_t *secs);
int xh_vcpu_reset(int vcpu);
int xh_vm_active_cpus(cpuset_t *cpus);
int xh_vm_suspended_cpus(cpuset_t *cpus);
int xh_vm_activate_cpu(int vcpu);
int xh_vm_restart_instruction(int vcpu);
int xh_vm_emulate_instruction(int vcpu, uint64_t gpa, struct vie *vie,
	struct vm_guest_paging *paging, mem_region_read_t memread,
	mem_region_write_t memwrite, void *memarg);
void xh_vm_vcpu_dump(int vcpu);
