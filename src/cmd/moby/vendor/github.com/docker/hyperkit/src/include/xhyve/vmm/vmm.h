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
#include <stdbool.h>
#include <xhyve/support/misc.h>
#include <xhyve/support/cpuset.h>
#include <xhyve/support/segments.h>
#include <xhyve/vmm/vmm_common.h>


#define	VM_INTINFO_VECTOR(info)	((info) & 0xff)
#define	VM_INTINFO_DEL_ERRCODE	0x800
#define	VM_INTINFO_RSVD		0x7ffff000
#define	VM_INTINFO_VALID	0x80000000
#define	VM_INTINFO_TYPE		0x700
#define	VM_INTINFO_HWINTR	(0 << 8)
#define	VM_INTINFO_NMI		(2 << 8)
#define	VM_INTINFO_HWEXCEPTION	(3 << 8)
#define	VM_INTINFO_SWINTR	(4 << 8)

struct vm;
struct vm_exception;
struct vm_memory_segment;
struct seg_desc;
struct vm_exit;
struct vm_run;
struct vhpet;
struct vioapic;
struct vlapic;
struct vmspace;
struct vm_object;
struct vm_guest_paging;
struct pmap;

typedef int (*vmm_init_func_t)(void);
typedef int (*vmm_cleanup_func_t)(void);
typedef void *(*vmi_vm_init_func_t)(struct vm *vm);
typedef int (*vmi_vcpu_init_func_t)(void *vmi, int vcpu);
typedef void (*vmi_vcpu_dump_func_t)(void *vmi, int vcpu);
typedef int (*vmi_run_func_t)(void *vmi, int vcpu, register_t rip,
	void *rendezvous_cookie, void *suspend_cookie);
typedef void (*vmi_vm_cleanup_func_t)(void *vmi);
typedef void (*vmi_vcpu_cleanup_func_t)(void *vmi, int vcpu);
typedef int (*vmi_get_register_t)(void *vmi, int vcpu, int num,
	uint64_t *retval);
typedef int (*vmi_set_register_t)(void *vmi, int vcpu, int num,
	uint64_t val);
typedef int (*vmi_get_desc_t)(void *vmi, int vcpu, int num,
	struct seg_desc *desc);
typedef int (*vmi_set_desc_t)(void *vmi, int vcpu, int num,
	struct seg_desc *desc);
typedef int (*vmi_get_cap_t)(void *vmi, int vcpu, int num, int *retval);
typedef int (*vmi_set_cap_t)(void *vmi, int vcpu, int num, int val);
typedef struct vlapic * (*vmi_vlapic_init)(void *vmi, int vcpu);
typedef void (*vmi_vlapic_cleanup)(void *vmi, struct vlapic *vlapic);
typedef void (*vmi_interrupt)(int vcpu);

struct vmm_ops {
	vmm_init_func_t init; /* module wide initialization */
	vmm_cleanup_func_t cleanup;
	vmi_vm_init_func_t vm_init; /* vm-specific initialization */
	vmi_vcpu_init_func_t vcpu_init;
	vmi_vcpu_dump_func_t vcpu_dump;
	vmi_run_func_t vmrun;
	vmi_vm_cleanup_func_t vm_cleanup;
	vmi_vcpu_cleanup_func_t vcpu_cleanup;
	vmi_get_register_t vmgetreg;
	vmi_set_register_t vmsetreg;
	vmi_get_desc_t vmgetdesc;
	vmi_set_desc_t vmsetdesc;
	vmi_get_cap_t vmgetcap;
	vmi_set_cap_t vmsetcap;
	vmi_vlapic_init vlapic_init;
	vmi_vlapic_cleanup vlapic_cleanup;
	vmi_interrupt vcpu_interrupt;
};

extern struct vmm_ops vmm_ops_intel;

int vmm_init(void);
int vmm_cleanup(void);
int vm_create(struct vm **retvm);
void vm_signal_pause(struct vm *vm, bool pause);
void vm_check_for_unpause(struct vm *vm, int vcpuid);
int vcpu_create(struct vm *vm, int vcpu);
void vm_destroy(struct vm *vm);
void vcpu_destroy(struct vm *vm, int vcpu);
int vm_reinit(struct vm *vm);
const char *vm_name(struct vm *vm);
int vm_malloc(struct vm *vm, uint64_t gpa, size_t len);
void *vm_gpa2hva(struct vm *vm, uint64_t gpa, uint64_t len);
int vm_gpabase2memseg(struct vm *vm, uint64_t gpabase,
	struct vm_memory_segment *seg);
int vm_get_memobj(struct vm *vm, uint64_t gpa, size_t len, uint64_t *offset,
	void **object);
bool vm_mem_allocated(struct vm *vm, uint64_t gpa);
int vm_get_register(struct vm *vm, int vcpu, int reg, uint64_t *retval);
int vm_set_register(struct vm *vm, int vcpu, int reg, uint64_t val);
int vm_get_seg_desc(struct vm *vm, int vcpu, int reg,
	struct seg_desc *ret_desc);
int vm_set_seg_desc(struct vm *vm, int vcpu, int reg, struct seg_desc *desc);
int vm_run(struct vm *vm, int vcpu, struct vm_exit *vm_exit);
int vm_suspend(struct vm *vm, enum vm_suspend_how how);
int vm_inject_nmi(struct vm *vm, int vcpu);
int vm_nmi_pending(struct vm *vm, int vcpuid);
void vm_nmi_clear(struct vm *vm, int vcpuid);
int vm_inject_extint(struct vm *vm, int vcpu);
int vm_extint_pending(struct vm *vm, int vcpuid);
void vm_extint_clear(struct vm *vm, int vcpuid);
struct vlapic *vm_lapic(struct vm *vm, int cpu);
struct vioapic *vm_ioapic(struct vm *vm);
struct vhpet *vm_hpet(struct vm *vm);
int vm_get_capability(struct vm *vm, int vcpu, int type, int *val);
int vm_set_capability(struct vm *vm, int vcpu, int type, int val);
int vm_get_x2apic_state(struct vm *vm, int vcpu, enum x2apic_state *state);
int vm_set_x2apic_state(struct vm *vm, int vcpu, enum x2apic_state state);
int vm_apicid2vcpuid(struct vm *vm, int apicid);
int vm_activate_cpu(struct vm *vm, int vcpu);
struct vm_exit *vm_exitinfo(struct vm *vm, int vcpuid);
void vm_exit_suspended(struct vm *vm, int vcpuid, uint64_t rip);
void vm_exit_rendezvous(struct vm *vm, int vcpuid, uint64_t rip);
void vm_vcpu_dump(struct vm *vm, int vcpuid);

/*
 * Rendezvous all vcpus specified in 'dest' and execute 'func(arg)'.
 * The rendezvous 'func(arg)' is not allowed to do anything that will
 * cause the thread to be put to sleep.
 *
 * If the rendezvous is being initiated from a vcpu context then the
 * 'vcpuid' must refer to that vcpu, otherwise it should be set to -1.
 *
 * The caller cannot hold any locks when initiating the rendezvous.
 *
 * The implementation of this API may cause vcpus other than those specified
 * by 'dest' to be stalled. The caller should not rely on any vcpus making
 * forward progress when the rendezvous is in progress.
 */
typedef void (*vm_rendezvous_func_t)(struct vm *vm, int vcpuid, void *arg);
void vm_smp_rendezvous(struct vm *vm, int vcpuid, cpuset_t dest,
    vm_rendezvous_func_t func, void *arg);
cpuset_t vm_active_cpus(struct vm *vm);
cpuset_t vm_suspended_cpus(struct vm *vm);

static __inline int
vcpu_rendezvous_pending(void *rendezvous_cookie)
{

	return (*(uintptr_t *)rendezvous_cookie != 0);
}

static __inline int
vcpu_suspended(void *suspend_cookie)
{

	return (*(int *)suspend_cookie);
}

enum vcpu_state {
	VCPU_IDLE,
	VCPU_FROZEN,
	VCPU_RUNNING,
	VCPU_SLEEPING,
};

int vcpu_set_state(struct vm *vm, int vcpu, enum vcpu_state state,
    bool from_idle);
enum vcpu_state vcpu_get_state(struct vm *vm, int vcpu);

static int __inline
vcpu_is_running(struct vm *vm, int vcpu)
{
	return (vcpu_get_state(vm, vcpu) == VCPU_RUNNING);
}

void *vcpu_stats(struct vm *vm, int vcpu);
void vcpu_notify_event(struct vm *vm, int vcpuid, bool lapic_intr);
struct vatpic *vm_atpic(struct vm *vm);
struct vatpit *vm_atpit(struct vm *vm);
struct vpmtmr *vm_pmtmr(struct vm *vm);
struct vrtc *vm_rtc(struct vm *vm);

/*
 * Inject exception 'vector' into the guest vcpu. This function returns 0 on
 * success and non-zero on failure.
 *
 * Wrapper functions like 'vm_inject_gp()' should be preferred to calling
 * this function directly because they enforce the trap-like or fault-like
 * behavior of an exception.
 *
 * This function should only be called in the context of the thread that is
 * executing this vcpu.
 */
int vm_inject_exception(struct vm *vm, int vcpuid, int vector, int err_valid,
    uint32_t errcode, int restart_instruction);

/*
 * This function is called after a VM-exit that occurred during exception or
 * interrupt delivery through the IDT. The format of 'intinfo' is described
 * in Figure 15-1, "EXITINTINFO for All Intercepts", APM, Vol 2.
 *
 * If a VM-exit handler completes the event delivery successfully then it
 * should call vm_exit_intinfo() to extinguish the pending event. For e.g.,
 * if the task switch emulation is triggered via a task gate then it should
 * call this function with 'intinfo=0' to indicate that the external event
 * is not pending anymore.
 *
 * Return value is 0 on success and non-zero on failure.
 */
int vm_exit_intinfo(struct vm *vm, int vcpuid, uint64_t intinfo);

/*
 * This function is called before every VM-entry to retrieve a pending
 * event that should be injected into the guest. This function combines
 * nested events into a double or triple fault.
 *
 * Returns 0 if there are no events that need to be injected into the guest
 * and non-zero otherwise.
 */
int vm_entry_intinfo(struct vm *vm, int vcpuid, uint64_t *info);

int vm_get_intinfo(struct vm *vm, int vcpuid, uint64_t *info1, uint64_t *info2);

enum vm_reg_name vm_segment_name(int seg_encoding);

struct vm_copyinfo {
	uint64_t	gpa;
	size_t		len;
	void		*hva;
};

/*
 * Set up 'copyinfo[]' to copy to/from guest linear address space starting
 * at 'gla' and 'len' bytes long. The 'prot' should be set to PROT_READ for
 * a copyin or PROT_WRITE for a copyout.
 *
 * retval	is_fault	Intepretation
 *   0		   0		Success
 *   0		   1		An exception was injected into the guest
 * EFAULT	  N/A		Unrecoverable error
 *
 * The 'copyinfo[]' can be passed to 'vm_copyin()' or 'vm_copyout()' only if
 * the return value is 0. The 'copyinfo[]' resources should be freed by calling
 * 'vm_copy_teardown()' after the copy is done.
 */
int vm_copy_setup(struct vm *vm, int vcpuid, struct vm_guest_paging *paging,
    uint64_t gla, size_t len, int prot, struct vm_copyinfo *copyinfo,
    int num_copyinfo, int *is_fault);
void vm_copy_teardown(struct vm *vm, int vcpuid, struct vm_copyinfo *copyinfo,
    int num_copyinfo);
void vm_copyin(struct vm *vm, int vcpuid, struct vm_copyinfo *copyinfo,
    void *kaddr, size_t len);
void vm_copyout(struct vm *vm, int vcpuid, const void *kaddr,
    struct vm_copyinfo *copyinfo, size_t len);

int vcpu_trace_exceptions(void);

/* APIs to inject faults into the guest */
void vm_inject_fault(void *vm, int vcpuid, int vector, int errcode_valid,
    int errcode);

static __inline void
vm_inject_ud(void *vm, int vcpuid)
{
	vm_inject_fault(vm, vcpuid, IDT_UD, 0, 0);
}

static __inline void
vm_inject_gp(void *vm, int vcpuid)
{
	vm_inject_fault(vm, vcpuid, IDT_GP, 1, 0);
}

static __inline void
vm_inject_ac(void *vm, int vcpuid, int errcode)
{
	vm_inject_fault(vm, vcpuid, IDT_AC, 1, errcode);
}

static __inline void
vm_inject_ss(void *vm, int vcpuid, int errcode)
{
	vm_inject_fault(vm, vcpuid, IDT_SS, 1, errcode);
}

void vm_inject_pf(void *vm, int vcpuid, int error_code, uint64_t cr2);

int vm_restart_instruction(void *vm, int vcpuid);
