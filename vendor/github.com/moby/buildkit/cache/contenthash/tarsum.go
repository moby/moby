package contenthash

import (
	"archive/tar"
	"io"
	"sort"
	"strconv"
	"strings"
)

// WriteV1TarsumHeaders writes a tar header to a writer in V1 tarsum format.
func WriteV1TarsumHeaders(h *tar.Header, w io.Writer) {
	for _, elem := range v1TarHeaderSelect(h) {
		w.Write([]byte(elem[0] + elem[1]))
	}
}

// Functions below are from docker legacy tarsum implementation.
// There is no valid technical reason to continue using them.

func v0TarHeaderSelect(h *tar.Header) (orderedHeaders [][2]string) {
	return [][2]string{
		{"name", h.Name},
		{"mode", strconv.FormatInt(h.Mode, 10)},
		{"uid", strconv.Itoa(h.Uid)},
		{"gid", strconv.Itoa(h.Gid)},
		{"size", strconv.FormatInt(h.Size, 10)},
		{"mtime", strconv.FormatInt(h.ModTime.UTC().Unix(), 10)},
		{"typeflag", string([]byte{h.Typeflag})},
		{"linkname", h.Linkname},
		{"uname", h.Uname},
		{"gname", h.Gname},
		{"devmajor", strconv.FormatInt(h.Devmajor, 10)},
		{"devminor", strconv.FormatInt(h.Devminor, 10)},
	}
}

func v1TarHeaderSelect(h *tar.Header) (orderedHeaders [][2]string) {
	pax := h.PAXRecords
	if len(h.Xattrs) > 0 { //nolint deprecated
		if pax == nil {
			pax = map[string]string{}
			for k, v := range h.Xattrs { //nolint deprecated
				pax["SCHILY.xattr."+k] = v
			}
		}
	}

	// Get extended attributes.
	xAttrKeys := make([]string, len(h.PAXRecords))
	for k := range pax {
		if strings.HasPrefix(k, "SCHILY.xattr.") {
			k = strings.TrimPrefix(k, "SCHILY.xattr.")
			if k == "security.capability" || !strings.HasPrefix(k, "security.") && !strings.HasPrefix(k, "system.") {
				xAttrKeys = append(xAttrKeys, k)
			}
		}
	}
	sort.Strings(xAttrKeys)

	// Make the slice with enough capacity to hold the 11 basic headers
	// we want from the v0 selector plus however many xattrs we have.
	orderedHeaders = make([][2]string, 0, 11+len(xAttrKeys))

	// Copy all headers from v0 excluding the 'mtime' header (the 5th element).
	v0headers := v0TarHeaderSelect(h)
	orderedHeaders = append(orderedHeaders, v0headers[0:5]...)
	orderedHeaders = append(orderedHeaders, v0headers[6:]...)

	// Finally, append the sorted xattrs.
	for _, k := range xAttrKeys {
		orderedHeaders = append(orderedHeaders, [2]string{k, h.PAXRecords["SCHILY.xattr."+k]})
	}

	return
}
