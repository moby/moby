// +build dfrunmount dfextall

package instructions

import (
	"encoding/csv"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

const MountTypeBind = "bind"
const MountTypeCache = "cache"
const MountTypeTmpfs = "tmpfs"

var allowedMountTypes = map[string]struct{}{
	MountTypeBind:  {},
	MountTypeCache: {},
	MountTypeTmpfs: {},
}

type mountsKeyT string

var mountsKey = mountsKeyT("dockerfile/run/mounts")

func init() {
	parseRunPreHooks = append(parseRunPreHooks, runMountPreHook)
	parseRunPostHooks = append(parseRunPostHooks, runMountPostHook)
}

func isValidMountType(s string) bool {
	_, ok := allowedMountTypes[s]
	return ok
}

func runMountPreHook(cmd *RunCommand, req parseRequest) error {
	st := &mountState{}
	st.flag = req.flags.AddStrings("mount")
	cmd.setExternalValue(mountsKey, st)
	return nil
}

func runMountPostHook(cmd *RunCommand, req parseRequest) error {
	st := getMountState(cmd)
	if st == nil {
		return errors.Errorf("no mount state")
	}
	var mounts []*Mount
	for _, str := range st.flag.StringValues {
		m, err := parseMount(str)
		if err != nil {
			return err
		}
		mounts = append(mounts, m)
	}
	st.mounts = mounts
	return nil
}

func getMountState(cmd *RunCommand) *mountState {
	v := cmd.getExternalValue(mountsKey)
	if v == nil {
		return nil
	}
	return v.(*mountState)
}

func GetMounts(cmd *RunCommand) []*Mount {
	return getMountState(cmd).mounts
}

type mountState struct {
	flag   *Flag
	mounts []*Mount
}

type Mount struct {
	Type     string
	From     string
	Source   string
	Target   string
	ReadOnly bool
	CacheID  string
}

func parseMount(value string) (*Mount, error) {
	csvReader := csv.NewReader(strings.NewReader(value))
	fields, err := csvReader.Read()
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse csv mounts")
	}

	m := &Mount{Type: MountTypeBind}

	roAuto := true

	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		key := strings.ToLower(parts[0])

		if len(parts) == 1 {
			switch key {
			case "readonly", "ro":
				m.ReadOnly = true
				roAuto = false
				continue
			case "readwrite", "rw":
				m.ReadOnly = false
				roAuto = false
				continue
			}
		}

		if len(parts) != 2 {
			return nil, errors.Errorf("invalid field '%s' must be a key=value pair", field)
		}

		value := parts[1]
		switch key {
		case "type":
			if !isValidMountType(strings.ToLower(value)) {
				return nil, errors.Errorf("invalid mount type %q", value)
			}
			m.Type = strings.ToLower(value)
		case "from":
			m.From = value
		case "source", "src":
			m.Source = value
		case "target", "dst", "destination":
			m.Target = value
		case "readonly", "ro":
			m.ReadOnly, err = strconv.ParseBool(value)
			if err != nil {
				return nil, errors.Errorf("invalid value for %s: %s", key, value)
			}
			roAuto = false
		case "readwrite", "rw":
			rw, err := strconv.ParseBool(value)
			if err != nil {
				return nil, errors.Errorf("invalid value for %s: %s", key, value)
			}
			m.ReadOnly = !rw
			roAuto = false
		case "id":
			m.CacheID = value
		default:
			return nil, errors.Errorf("unexpected key '%s' in '%s'", key, field)
		}
	}

	if roAuto {
		if m.Type == MountTypeCache {
			m.ReadOnly = false
		} else {
			m.ReadOnly = true
		}
	}

	return m, nil
}
