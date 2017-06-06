/*-
 * Copyright (c) 2013 Hudson River Trading LLC
 * Written by: John H. Baldwin <jhb@FreeBSD.org>
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
 * THIS SOFTWARE IS PROVIDED BY THE AUTHOR AND CONTRIBUTORS ``AS IS'' AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED.  IN NO EVENT SHALL THE AUTHOR OR CONTRIBUTORS BE LIABLE
 * FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 * OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
 * LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
 * OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
 * SUCH DAMAGE.
 */

#include <stdint.h>
#include <assert.h>
#include <errno.h>
#include <pthread.h>
#include <signal.h>
#include <xhyve/support/misc.h>
#include <xhyve/vmm/vmm_api.h>
#include <xhyve/acpi.h>
#include <xhyve/inout.h>
#include <xhyve/mevent.h>
#include <xhyve/pci_irq.h>
#include <xhyve/pci_lpc.h>

static pthread_mutex_t pm_lock = PTHREAD_MUTEX_INITIALIZER;
static struct mevent *power_button;
static sig_t old_power_handler;

/*
 * Reset Control register at I/O port 0xcf9.  Bit 2 forces a system
 * reset when it transitions from 0 to 1.  Bit 1 selects the type of
 * reset to attempt: 0 selects a "soft" reset, and 1 selects a "hard"
 * reset.
 */
static int
reset_handler(UNUSED int vcpu, int in, UNUSED int port, int bytes,
	uint32_t *eax, UNUSED void *arg)
{
	int error;

	static uint8_t reset_control;

	if (bytes != 1)
		return (-1);
	if (in)
		*eax = reset_control;
	else {
		reset_control = (uint8_t) *eax;

		/* Treat hard and soft resets the same. */
		if (reset_control & 0x4) {
			error = xh_vm_suspend(VM_SUSPEND_RESET);
			assert(error == 0 || errno == EALREADY);
		}
	}
	return (0);
}
INOUT_PORT(reset_reg, 0xCF9, IOPORT_F_INOUT, reset_handler);

/*
 * ACPI's SCI is a level-triggered interrupt.
 */
static int sci_active;

static void
sci_assert(void)
{
	if (sci_active)
		return;
	xh_vm_isa_assert_irq(SCI_INT, SCI_INT);
	sci_active = 1;
}

static void
sci_deassert(void)
{
	if (!sci_active)
		return;
	xh_vm_isa_deassert_irq(SCI_INT, SCI_INT);
	sci_active = 0;
}

/*
 * Power Management 1 Event Registers
 *
 * The only power management event supported is a power button upon
 * receiving SIGTERM.
 */
static uint16_t pm1_enable, pm1_status;

#define	PM1_TMR_STS		0x0001
#define	PM1_BM_STS		0x0010
#define	PM1_GBL_STS		0x0020
#define	PM1_PWRBTN_STS		0x0100
#define	PM1_SLPBTN_STS		0x0200
#define	PM1_RTC_STS		0x0400
#define	PM1_WAK_STS		0x8000

#define	PM1_TMR_EN		0x0001
#define	PM1_GBL_EN		0x0020
#define	PM1_PWRBTN_EN		0x0100
#define	PM1_SLPBTN_EN		0x0200
#define	PM1_RTC_EN		0x0400

static void
sci_update(void)
{
	int need_sci;

	/* See if the SCI should be active or not. */
	need_sci = 0;
	if ((pm1_enable & PM1_TMR_EN) && (pm1_status & PM1_TMR_STS))
		need_sci = 1;
	if ((pm1_enable & PM1_GBL_EN) && (pm1_status & PM1_GBL_STS))
		need_sci = 1;
	if ((pm1_enable & PM1_PWRBTN_EN) && (pm1_status & PM1_PWRBTN_STS))
		need_sci = 1;
	if ((pm1_enable & PM1_SLPBTN_EN) && (pm1_status & PM1_SLPBTN_STS))
		need_sci = 1;
	if ((pm1_enable & PM1_RTC_EN) && (pm1_status & PM1_RTC_STS))
		need_sci = 1;
	if (need_sci)
		sci_assert();
	else
		sci_deassert();
}

static int
pm1_status_handler(UNUSED int vcpu, int in, UNUSED int port, int bytes,
	uint32_t *eax, UNUSED void *arg)
{

	if (bytes != 2)
		return (-1);

	pthread_mutex_lock(&pm_lock);
	if (in)
		*eax = pm1_status;
	else {
		/*
		 * Writes are only permitted to clear certain bits by
		 * writing 1 to those flags.
		 */
		pm1_status &= ~(*eax & (PM1_WAK_STS | PM1_RTC_STS |
		    PM1_SLPBTN_STS | PM1_PWRBTN_STS | PM1_BM_STS));
		sci_update();
	}
	pthread_mutex_unlock(&pm_lock);
	return (0);
}

static int
pm1_enable_handler(UNUSED int vcpu, int in, UNUSED int port, int bytes,
	uint32_t *eax, UNUSED void *arg)
{

	if (bytes != 2)
		return (-1);

	pthread_mutex_lock(&pm_lock);
	if (in)
		*eax = pm1_enable;
	else {
		/*
		 * Only permit certain bits to be set.  We never use
		 * the global lock, but ACPI-CA whines profusely if it
		 * can't set GBL_EN.
		 */
		pm1_enable = *eax & (PM1_PWRBTN_EN | PM1_GBL_EN);
		sci_update();
	}
	pthread_mutex_unlock(&pm_lock);
	return (0);
}
INOUT_PORT(pm1_status, PM1A_EVT_ADDR, IOPORT_F_INOUT, pm1_status_handler);
INOUT_PORT(pm1_enable, PM1A_EVT_ADDR2, IOPORT_F_INOUT, pm1_enable_handler);

void
push_power_button(void)
{
	pthread_mutex_lock(&pm_lock);
	if (!(pm1_status & PM1_PWRBTN_STS)) {
		pm1_status |= PM1_PWRBTN_STS;
		sci_update();
	}
	pthread_mutex_unlock(&pm_lock);
}

static void
power_button_handler(UNUSED int signal, UNUSED enum ev_type type,
	UNUSED void *arg)
{
	push_power_button();
}

/*
 * Power Management 1 Control Register
 *
 * This is mostly unimplemented except that we wish to handle writes that
 * set SPL_EN to handle S5 (soft power off).
 */
static uint16_t pm1_control;

#define	PM1_SCI_EN	0x0001
#define	PM1_SLP_TYP	0x1c00
#define	PM1_SLP_EN	0x2000
#define	PM1_ALWAYS_ZERO	0xc003

static int
pm1_control_handler(UNUSED int vcpu, int in, UNUSED int port, int bytes,
	uint32_t *eax, UNUSED void *arg)
{
	int error;

	if (bytes != 2)
		return (-1);
	if (in)
		*eax = pm1_control;
	else {
		/*
		 * Various bits are write-only or reserved, so force them
		 * to zero in pm1_control.  Always preserve SCI_EN as OSPM
		 * can never change it.
		 */
		pm1_control = (uint16_t) ((pm1_control & PM1_SCI_EN) |
			(*eax & ~((unsigned) (PM1_SLP_EN | PM1_ALWAYS_ZERO))));

		/*
		 * If SLP_EN is set, check for S5.  Bhyve's _S5_ method
		 * says that '5' should be stored in SLP_TYP for S5.
		 */
		if (*eax & PM1_SLP_EN) {
			if ((pm1_control & PM1_SLP_TYP) >> 10 == 5) {
				error = xh_vm_suspend(VM_SUSPEND_POWEROFF);
				assert(error == 0 || errno == EALREADY);
			}
		}
	}
	return (0);
}
INOUT_PORT(pm1_control, PM1A_CNT_ADDR, IOPORT_F_INOUT, pm1_control_handler);
SYSRES_IO(PM1A_EVT_ADDR, 8);

/*
 * ACPI SMI Command Register
 *
 * This write-only register is used to enable and disable ACPI.
 */
static int
smi_cmd_handler(UNUSED int vcpu, int in, UNUSED int port, int bytes,
	uint32_t *eax, UNUSED void *arg)
{
	assert(!in);
	if (bytes != 1)
		return (-1);

	pthread_mutex_lock(&pm_lock);
	switch (*eax) {
	case BHYVE_ACPI_ENABLE:
		pm1_control |= PM1_SCI_EN;
		if (power_button == NULL) {
			power_button = mevent_add(SIGTERM, EVF_SIGNAL,
				power_button_handler, NULL);
			old_power_handler = signal(SIGTERM, SIG_IGN);
		}
		break;
	case BHYVE_ACPI_DISABLE:
		pm1_control &= ~PM1_SCI_EN;
		if (power_button != NULL) {
			mevent_delete(power_button);
			power_button = NULL;
			signal(SIGTERM, old_power_handler);
		}
		break;
	}
	pthread_mutex_unlock(&pm_lock);
	return (0);
}
INOUT_PORT(smi_cmd, SMI_CMD, IOPORT_F_OUT, smi_cmd_handler);
SYSRES_IO(SMI_CMD, 1);

void
sci_init(void)
{
	/*
	 * Mark ACPI's SCI as level trigger and bump its use count
	 * in the PIRQ router.
	 */
	pci_irq_use(SCI_INT);
	xh_vm_isa_set_irq_trigger(SCI_INT, LEVEL_TRIGGER);
}
