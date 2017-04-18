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

#include <time.h>
#include <assert.h>
#include <xhyve/vmm/vmm_api.h>
#include <xhyve/acpi.h>
#include <xhyve/pci_lpc.h>
#include <xhyve/rtc.h>

#define	IO_RTC 0x70

#define	RTC_LMEM_LSB 0x34
#define	RTC_LMEM_MSB 0x35
#define	RTC_HMEM_LSB 0x5b
#define	RTC_HMEM_SB 0x5c
#define	RTC_HMEM_MSB 0x5d

#define m_64KB (64*1024)
#define	m_16MB (16*1024*1024)

/*
 * Returns the current RTC time as number of seconds since 00:00:00 Jan 1, 1970
 */
static time_t
rtc_time(int use_localtime)
{
	struct tm tm;
	time_t t;

	time(&t);
	if (use_localtime) {
		localtime_r(&t, &tm);
		t = timegm(&tm);
	}
	return (t);
}

void
rtc_init(int use_localtime)
{
	size_t himem;
	size_t lomem;
	int err;

	/* XXX init diag/reset code/equipment/checksum ? */

	/*
	 * Report guest memory size in nvram cells as required by UEFI.
	 * Little-endian encoding.
	 * 0x34/0x35 - 64KB chunks above 16MB, below 4GB
	 * 0x5b/0x5c/0x5d - 64KB chunks above 4GB
	 */
	lomem = (xh_vm_get_lowmem_size() - m_16MB) / m_64KB;
	err = xh_vm_rtc_write(RTC_LMEM_LSB, ((uint8_t) lomem));
	assert(err == 0);
	err = xh_vm_rtc_write(RTC_LMEM_MSB, ((uint8_t) (lomem >> 8)));
	assert(err == 0);

	himem = xh_vm_get_highmem_size() / m_64KB;
	err = xh_vm_rtc_write(RTC_HMEM_LSB, ((uint8_t) himem));
	assert(err == 0);
	err = xh_vm_rtc_write(RTC_HMEM_SB, ((uint8_t) (himem >> 8)));
	assert(err == 0);
	err = xh_vm_rtc_write(RTC_HMEM_MSB, ((uint8_t) (himem >> 16)));
	assert(err == 0);

	err = xh_vm_rtc_settime(rtc_time(use_localtime));
	assert(err == 0);
}

static void
rtc_dsdt(void)
{
	dsdt_line("");
	dsdt_line("Device (RTC)");
	dsdt_line("{");
	dsdt_line("  Name (_HID, EisaId (\"PNP0B00\"))");
	dsdt_line("  Name (_CRS, ResourceTemplate ()");
	dsdt_line("  {");
	dsdt_indent(2);
	dsdt_fixed_ioport(IO_RTC, 2);
	dsdt_fixed_irq(8);
	dsdt_unindent(2);
	dsdt_line("  })");
	dsdt_line("}");
}
LPC_DSDT(rtc_dsdt);

/*
 * Reserve the extended RTC I/O ports although they are not emulated at this
 * time.
 */
SYSRES_IO(0x72, 6);
