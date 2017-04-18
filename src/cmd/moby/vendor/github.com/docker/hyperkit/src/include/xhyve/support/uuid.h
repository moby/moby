/*-
 * Copyright (c) 2002,2005 Marcel Moolenaar
 * Copyright (c) 2002 Hiten Mahesh Pandya
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
 *
 * $FreeBSD$
 */

#pragma once

#include <stdint.h>
#include <stdio.h>
#include <string.h>

#define	_UUID_NODE_LEN 6

struct uuid {
	uint32_t time_low;
	uint16_t time_mid;
	uint16_t time_hi_and_version;
	uint8_t clock_seq_hi_and_reserved;
	uint8_t clock_seq_low;
	uint8_t node[_UUID_NODE_LEN];
};

typedef struct uuid uuid_internal_t;

/*
 * This implementation mostly conforms to the DCE 1.1 specification.
 * See Also:
 *	uuidgen(1), uuidgen(2), uuid(3)
 */

/* Status codes returned by the functions. */
#define	uuid_s_ok			0
#define	uuid_s_bad_version		1
#define	uuid_s_invalid_string_uuid	2
#define	uuid_s_no_memory		3

/*
 * uuid_create_nil() - create a nil UUID.
 * See also:
 *	http://www.opengroup.org/onlinepubs/009629399/uuid_create_nil.htm
 */
static inline void
uuid_create_nil(uuid_t *u, uint32_t *status)
{
	if (status)
		*status = uuid_s_ok;

	bzero(u, sizeof(*u));
}

static inline void
uuid_enc_le(void *buf, uuid_t *uuid)
{
	uuid_internal_t *u = (uuid_internal_t *) ((void *) uuid);
	uint8_t *p = buf;
	int i;

	memcpy(p, &u->time_low, 4);
	memcpy(p, &u->time_mid, 2);
	memcpy(p, &u->time_hi_and_version, 2);
	p[8] = u->clock_seq_hi_and_reserved;
	p[9] = u->clock_seq_low;
	for (i = 0; i < _UUID_NODE_LEN; i++)
		p[10 + i] = u->node[i];
}

/*
 * uuid_from_string() - convert a string representation of an UUID into
 *			a binary representation.
 * See also:
 *	http://www.opengroup.org/onlinepubs/009629399/uuid_from_string.htm
 *
 * NOTE: The sequence field is in big-endian, while the time fields are in
 *	 native byte order.
 */
static inline void
uuid_from_string(const char *s, uuid_t *uuid, uint32_t *status)
{
	uuid_internal_t *u = (uuid_internal_t *) ((void *) uuid);
	int n;

	/* Short-circuit 2 special cases: NULL pointer and empty string. */
	if (s == NULL || *s == '\0') {
		uuid_create_nil(((uuid_t *) u), status);
		return;
	}

	/* Assume the worst. */
	if (status != NULL)
		*status = uuid_s_invalid_string_uuid;

	/* The UUID string representation has a fixed length. */
	if (strlen(s) != 36)
		return;

	/*
	 * We only work with "new" UUIDs. New UUIDs have the form:
	 *	01234567-89ab-cdef-0123-456789abcdef
	 * The so called "old" UUIDs, which we don't support, have the form:
	 *	0123456789ab.cd.ef.01.23.45.67.89.ab
	 */
	if (s[8] != '-')
		return;

	n = sscanf(s,
	    "%8x-%4hx-%4hx-%2hhx%2hhx-%2hhx%2hhx%2hhx%2hhx%2hhx%2hhx",
	    &u->time_low, &u->time_mid, &u->time_hi_and_version,
	    &u->clock_seq_hi_and_reserved, &u->clock_seq_low, &u->node[0],
	    &u->node[1], &u->node[2], &u->node[3], &u->node[4], &u->node[5]);

	/* Make sure we have all conversions. */
	if (n != 11)
		return;

	/* We have a successful scan. Check semantics... */
	n = u->clock_seq_hi_and_reserved;
	if ((n & 0x80) != 0x00 &&			/* variant 0? */
	    (n & 0xc0) != 0x80 &&			/* variant 1? */
	    (n & 0xe0) != 0xc0) {			/* variant 2? */
		if (status != NULL)
			*status = uuid_s_bad_version;
	} else {
		if (status != NULL)
			*status = uuid_s_ok;
	}
}
