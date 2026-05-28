package instructions

import (
	"strconv"
	"strings"

	"github.com/moby/buildkit/util/suggest"
	"github.com/pkg/errors"
	"github.com/tonistiigi/go-csvvalue"
)

var devicesKey = "dockerfile/run/devices"

func init() {
	parseRunPreHooks = append(parseRunPreHooks, runDevicePreHook)
	parseRunPostHooks = append(parseRunPostHooks, runDevicePostHook)
}

func runDevicePreHook(cmd *RunCommand, req parseRequest) error {
	st := &deviceState{}
	st.flag = req.flags.AddStrings("device")
	cmd.setExternalValue(devicesKey, st)
	return nil
}

func runDevicePostHook(cmd *RunCommand, req parseRequest) error {
	return setDeviceState(cmd)
}

func setDeviceState(cmd *RunCommand) error {
	st := getDeviceState(cmd)
	if st == nil {
		return errors.Errorf("no device state")
	}
	devices := make([]*Device, len(st.flag.StringValues))
	for i, str := range st.flag.StringValues {
		d, err := ParseDevice(str)
		if err != nil {
			return err
		}
		devices[i] = d
	}
	st.devices = devices
	return nil
}

func getDeviceState(cmd *RunCommand) *deviceState {
	v := cmd.getExternalValue(devicesKey)
	if v == nil {
		return nil
	}
	return v.(*deviceState)
}

func GetDevices(cmd *RunCommand) []*Device {
	return getDeviceState(cmd).devices
}

type deviceState struct {
	flag    *Flag
	devices []*Device
}

type Device struct {
	Name     string
	Required bool
}

func ParseDevice(val string) (*Device, error) {
	fields, err := csvvalue.Fields(val, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse csv devices")
	}

	d := &Device{}

	for _, field := range fields {
		key, value, ok := strings.Cut(field, "=")
		key = strings.ToLower(key)

		if !ok {
			switch key {
			case "required":
				d.Required = true
				continue
			default:
				if d.Name == "" {
					d.Name = field
					continue
				}
				// any other option requires a value.
				return nil, errors.Errorf("invalid field '%s' must be a key=value pair", field)
			}
		}

		switch key {
		case "name":
			if d.Name != "" {
				return nil, errors.Errorf("device name already set to %s", d.Name)
			}
			d.Name = value
		case "required":
			d.Required, err = strconv.ParseBool(value)
			if err != nil {
				return nil, errors.Errorf("invalid value for %s: %s", key, value)
			}
		default:
			if d.Name == "" {
				d.Name = field
				continue
			}
			allKeys := []string{"name", "required"}
			return nil, suggest.WrapError(errors.Errorf("unexpected key '%s' in '%s'", key, field), key, allKeys, true)
		}
	}

	return d, nil
}
