#include <stdlib.h>
#include <string.h>

#ifdef ESCAPE_TEST
#  include <assert.h>
#  define test_assert(arg) assert(arg)
#else
#  define test_assert(arg)
#endif

#define DEL '\x7f'

/*
 * Poor man version of itoa with base=16 and input number from 0 to 15,
 * represented by a char. Converts it to a single hex digit ('0' to 'f').
 */
static char hex(char i)
{
	test_assert(i >= 0 && i < 16);

	if (i >= 0 && i < 10) {
		return '0' + i;
	}
	if (i >= 10 && i < 16) {
		return 'a' + i - 10;
	}
	return '?';
}

/*
 * Given the character, tells how many _extra_ characters are needed
 * to JSON-escape it. If 0 is returned, the character does not need to
 * be escaped.
 */
static int need_escape(char c)
{
	switch (c) {
	case '\\':
	case '"':
	case '\b':
	case '\n':
	case '\r':
	case '\t':
	case '\f':
		return 1;
	case DEL:		// -> \u007f
		return 5;
	default:
		if (c > 0 && c < ' ') {
			// ASCII decimal 01 to 31 -> \u00xx
			return 5;
		}
		return 0;
	}
}

/*
 * Escape the string so it can be used as a JSON string (per RFC4627,
 * section 2.5 minimal requirements, plus the DEL (0x7f) character).
 *
 * It is expected that the argument is a string allocated via malloc.
 * In case no escaping is needed, the original string is returned as is;
 * otherwise, the original string is free'd, and the newly allocated
 * escaped string is returned. Thus, in any case, the value returned
 * need to be free'd by the caller.
 */
char *escape_json_string(char *s)
{
	int i, j, len;
	char *c, *out;

	/*
	 * First, check if escaping is at all needed -- if not, we can avoid
	 * malloc and return the argument as is.  While at it, count how much
	 * extra space is required.
	 *
	 * XXX: the counting code must be in sync with the escaping code
	 * (checked by test_assert()s below).
	 */
	for (i = j = 0; s[i] != '\0'; i++) {
		j += need_escape(s[i]);
	}
	if (j == 0) {
		// nothing to escape
		return s;
	}

	len = i + j + 1;
	out = malloc(len);
	if (!out) {
		free(s);
		// As malloc failed, strdup can fail, too, so in the worst case
		// scenario NULL will be returned from here.
		return strdup("escape_json_string: out of memory");
	}
	for (c = s, j = 0; *c != '\0'; c++) {
		switch (*c) {
		case '"':
		case '\\':
			test_assert(need_escape(*c) == 1);
			out[j++] = '\\';
			out[j++] = *c;
			continue;
		}
		if ((*c < 0 || *c >= ' ') && (*c != DEL)) {
			// no escape needed
			test_assert(need_escape(*c) == 0);
			out[j++] = *c;
			continue;
		}
		out[j++] = '\\';
		switch (*c) {
		case '\b':
			out[j++] = 'b';
			break;
		case '\n':
			out[j++] = 'n';
			break;
		case '\r':
			out[j++] = 'r';
			break;
		case '\t':
			out[j++] = 't';
			break;
		case '\f':
			out[j++] = 'f';
			break;
		default:
			test_assert(need_escape(*c) == 5);
			out[j++] = 'u';
			out[j++] = '0';
			out[j++] = '0';
			out[j++] = hex(*c >> 4);
			out[j++] = hex(*c & 0x0f);
		}
	}
	test_assert(j + 1 == len);
	out[j] = '\0';

	free(s);
	return out;
}
