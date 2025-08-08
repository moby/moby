package logger

import (
	"errors"
	"io"
)

type client interface {
	Call(string, any, any) error
	Stream(string, any) (io.ReadCloser, error)
}

type logPluginProxy struct {
	client
}

type logPluginProxyStartLoggingRequest struct {
	File string
	Info Info
}

type logPluginProxyStartLoggingResponse struct {
	Err string
}

func (pp *logPluginProxy) StartLogging(file string, info Info) (err error) {
	var (
		req logPluginProxyStartLoggingRequest
		ret logPluginProxyStartLoggingResponse
	)

	req.File = file
	req.Info = info
	if err = pp.Call("LogDriver.StartLogging", req, &ret); err != nil {
		return err
	}

	if ret.Err != "" {
		return errors.New(ret.Err)
	}

	return nil
}

type logPluginProxyStopLoggingRequest struct {
	File string
}

type logPluginProxyStopLoggingResponse struct {
	Err string
}

func (pp *logPluginProxy) StopLogging(file string) (err error) {
	var (
		req logPluginProxyStopLoggingRequest
		ret logPluginProxyStopLoggingResponse
	)

	req.File = file
	if err = pp.Call("LogDriver.StopLogging", req, &ret); err != nil {
		return err
	}

	if ret.Err != "" {
		return errors.New(ret.Err)
	}

	return nil
}

type logPluginProxyCapabilitiesResponse struct {
	Cap Capability
	Err string
}

func (pp *logPluginProxy) Capabilities() (Capability, error) {
	var ret logPluginProxyCapabilitiesResponse
	if err := pp.Call("LogDriver.Capabilities", nil, &ret); err != nil {
		return Capability{}, err
	}

	if ret.Err != "" {
		return Capability{}, errors.New(ret.Err)
	}

	return ret.Cap, nil
}

type logPluginProxyReadLogsRequest struct {
	Info   Info
	Config ReadConfig
}

func (pp *logPluginProxy) ReadLogs(info Info, config ReadConfig) (stream io.ReadCloser, _ error) {
	return pp.Stream("LogDriver.ReadLogs", logPluginProxyReadLogsRequest{
		Info:   info,
		Config: config,
	})
}
