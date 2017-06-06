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

#include <stdio.h>

#ifdef XHYVE_CONFIG_TRACE
#define vmmtrace printf
#else
#define vmmtrace if (0) printf
#endif

struct vm;
extern const char *vm_name(struct vm *vm);

#define	VCPU_CTR0(vm, vcpuid, format)					\
vmmtrace("vm %s[%d]: " format "\n", vm_name((vm)), (vcpuid))

#define	VCPU_CTR1(vm, vcpuid, format, p1)				\
vmmtrace("vm %s[%d]: " format "\n", vm_name((vm)), (vcpuid), (p1))

#define	VCPU_CTR2(vm, vcpuid, format, p1, p2)				\
vmmtrace("vm %s[%d]: " format "\n", vm_name((vm)), (vcpuid), (p1), (p2))

#define	VCPU_CTR3(vm, vcpuid, format, p1, p2, p3)			\
vmmtrace("vm %s[%d]: " format "\n", vm_name((vm)), (vcpuid), (p1), (p2), (p3))

#define	VCPU_CTR4(vm, vcpuid, format, p1, p2, p3, p4)			\
vmmtrace("vm %s[%d]: " format "\n", vm_name((vm)), (vcpuid),		\
    (p1), (p2), (p3), (p4))

#define	VM_CTR0(vm, format)						\
vmmtrace("vm %s: " format "\n", vm_name((vm)))

#define	VM_CTR1(vm, format, p1)						\
vmmtrace("vm %s: " format "\n", vm_name((vm)), (p1))

#define	VM_CTR2(vm, format, p1, p2)					\
vmmtrace("vm %s: " format "\n", vm_name((vm)), (p1), (p2))

#define	VM_CTR3(vm, format, p1, p2, p3)					\
vmmtrace("vm %s: " format "\n", vm_name((vm)), (p1), (p2), (p3))

#define	VM_CTR4(vm, format, p1, p2, p3, p4)				\
vmmtrace("vm %s: " format "\n", vm_name((vm)), (p1), (p2), (p3), (p4))
