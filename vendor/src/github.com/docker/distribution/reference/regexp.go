package reference

import "regexp"

var (
	// nameSubComponentRegexp defines the part of the name which must be
	// begin and end with an alphanumeric character. These characters can
	// be separated by any number of dashes.
	nameSubComponentRegexp = regexp.MustCompile(`[a-z0-9]+(?:[-]+[a-z0-9]+)*`)

	// nameComponentRegexp restricts registry path component names to
	// start with at least one letter or number, with following parts able to
	// be separated by one period, underscore or double underscore.
	nameComponentRegexp = regexp.MustCompile(nameSubComponentRegexp.String() + `(?:(?:[._]|__)` + nameSubComponentRegexp.String() + `)*`)

	nameRegexp = regexp.MustCompile(`(?:` + nameComponentRegexp.String() + `/)*` + nameComponentRegexp.String())

	hostnameComponentRegexp = regexp.MustCompile(`(?:[a-z0-9]|[a-z0-9][a-z0-9-]*[a-z0-9])`)

	// hostnameComponentRegexp restricts the registry hostname component of a repository name to
	// start with a component as defined by hostnameRegexp and followed by an optional port.
	hostnameRegexp = regexp.MustCompile(`(?:` + hostnameComponentRegexp.String() + `\.)*` + hostnameComponentRegexp.String() + `(?::[0-9]+)?`)

	// TagRegexp matches valid tag names. From docker/docker:graph/tags.go.
	TagRegexp = regexp.MustCompile(`[\w][\w.-]{0,127}`)

	// anchoredTagRegexp matches valid tag names, anchored at the start and
	// end of the matched string.
	anchoredTagRegexp = regexp.MustCompile(`^` + TagRegexp.String() + `$`)

	// DigestRegexp matches valid digests.
	DigestRegexp = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*[:][[:xdigit:]]{32,}`)

	// anchoredDigestRegexp matches valid digests, anchored at the start and
	// end of the matched string.
	anchoredDigestRegexp = regexp.MustCompile(`^` + DigestRegexp.String() + `$`)

	// NameRegexp is the format for the name component of references. The
	// regexp has capturing groups for the hostname and name part omitting
	// the seperating forward slash from either.
	NameRegexp = regexp.MustCompile(`(?:` + hostnameRegexp.String() + `/)?` + nameRegexp.String())

	// ReferenceRegexp is the full supported format of a reference. The
	// regexp has capturing groups for name, tag, and digest components.
	ReferenceRegexp = regexp.MustCompile(`^((?:` + hostnameRegexp.String() + `/)?` + nameRegexp.String() + `)(?:[:](` + TagRegexp.String() + `))?(?:[@](` + DigestRegexp.String() + `))?$`)

	// anchoredNameRegexp is used to parse a name value, capturing hostname
	anchoredNameRegexp = regexp.MustCompile(`^(?:(` + hostnameRegexp.String() + `)/)?(` + nameRegexp.String() + `)$`)
)
