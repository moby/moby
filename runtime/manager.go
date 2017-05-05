package runtime

import (
	"sync"

	"github.com/docker/docker/api/types/runtime"
	getter "github.com/docker/docker/pkg/plugingetter"
)

const RuntimeDriverName = "RuntimeDriver"

type RuntimeManager struct {
	sync.RWMutex
	cache          map[string]*RuntimeDriver
	defaultRuntime string
	plugingetter   getter.PluginGetter
}

func NewRuntimeManager() *RuntimeManager {
	return &RuntimeManager{
		cache: make(map[string]*RuntimeDriver),
	}
}

func (rtmgr *RuntimeManager) RegisterPluginGetter(plugingetter getter.PluginGetter) {
	rtmgr.plugingetter = plugingetter
}

// FillCache populates the runtime manager cache with the managed runtimes.
func (rtmgr *RuntimeManager) FillCache() error {
	if rtmgr.plugingetter == nil {
		return ErrRuntimeMissingPluginGetter
	}

	runtimePlugins := rtmgr.plugingetter.GetAllManagedPluginsByCap(RuntimeDriverName)

	for _, plugin := range runtimePlugins {
		runtime, err := runtimeDriverFromPlugin(plugin)
		if err != nil {
			continue
		}

		_, err = rtmgr.GetRuntime(runtime.name())
		if err != nil {
			continue
		}
	}

	return nil
}

func (rtmgr *RuntimeManager) RegisterConfiguredRuntime(name string, rt runtime.Runtime) error {
	rtmgr.Lock()
	defer rtmgr.Unlock()

	if _, ok := rtmgr.cache[name]; ok {
		return ErrRuntimeExists(name)
	}

	rtmgr.cache[name] = newRuntimeDriver(name, rt, nil)

	return nil
}

func (rtmgr *RuntimeManager) Lookup(name string) (*RuntimeDriver, error) {
	rtmgr.RLock()
	defer rtmgr.RUnlock()
	if rt, ok := rtmgr.cache[name]; ok {
		if rt.plugin == nil {
			// This is a configured runtime
			return rt, nil
		}

		// Verify from the plugin store that it is still there
		_, err := rtmgr.plugingetter.Get(name, RuntimeDriverName, getter.Lookup)
		if err != nil {
			rtmgr.ReleaseRuntime(name)
			return nil, ErrRuntimeNotFound(name)
		}

		return rt, nil
	}

	return nil, ErrRuntimeNotFound(name)
}


func (rtmgr *RuntimeManager) GetRuntime(name string) (*RuntimeDriver, error) {
	rtmgr.RLock()
	defer rtmgr.RUnlock()
	if rt, ok := rtmgr.cache[name]; ok {
		if rt.plugin == nil {
			// This is a configured runtime
			return rt, nil
		}

		// Verify from the plugin store that it is still there
		_, err := rtmgr.plugingetter.Get(name, RuntimeDriverName, getter.Lookup)
		if err != nil {
			rtmgr.ReleaseRuntime(name)
			return nil, ErrRuntimeNotFound(name)
		}

		return rt, nil
	}

	// This is not a cached runtime, let's search through the plugin store
	rtplugin, err := rtmgr.plugingetter.Get(name, RuntimeDriverName, getter.Acquire)
	if err != nil {
		return nil, ErrRuntimeNotFound(name)
	}

	// Create and initialize a new runtime driver
	newRuntimeDriver, err := runtimeDriverFromPlugin(rtplugin)
	if err != nil {
		return nil, err
	}

	// Add the runtime to our cache
	rtmgr.cache[name] = newRuntimeDriver

	return newRuntimeDriver, nil
}

func (rtmgr *RuntimeManager) ReleaseRuntime(name string) error {
	rtmgr.RLock()
	defer rtmgr.RUnlock()
	rt, ok := rtmgr.cache[name]
	if !ok {
		return ErrRuntimeNotFound(name)
	}

	delete(rtmgr.cache, rt.info.Name)

	if rt.plugin != nil {
		// Releasethe plugin store
		_, _ = rtmgr.plugingetter.Get(name, RuntimeDriverName, getter.Release)
	}

	return nil
}

// GetRuntimePathAndArgs returns the runtime path and arguments for a given
// runtime name
func (rtmgr *RuntimeManager) GetRuntimePathAndArgs(name string) (string, []string, error) {
	rt, err := rtmgr.GetRuntime(name)
	if err != nil {
		return "", nil, err
	}

	return rt.info.Runtime.Path, rt.info.Runtime.Args, nil
}

// GetDefaultRuntimeName returns the current default runtime
func (rtmgr *RuntimeManager) GetDefaultRuntimeName() string {
	rtmgr.RLock()
	rt := rtmgr.defaultRuntime
	rtmgr.RUnlock()

	return rt
}

func (rtmgr *RuntimeManager) GetAllRuntimes() []RuntimeDriver {
	rtmgr.Lock()
	defer rtmgr.Unlock()

	var runtimes []RuntimeDriver

	for name, r := range rtmgr.cache {
		if r.plugin != nil {
			_, err := rtmgr.plugingetter.Get(name, RuntimeDriverName, getter.Lookup)
			if err != nil {
				// This cache entry is missing, let's delete it
				if name == rtmgr.defaultRuntime {
					rtmgr.defaultRuntime = ""
				}

				delete(rtmgr.cache, name)

				continue
			}
		}

		runtimes = append(runtimes, *r)
	}

	return runtimes
}

func (rtmgr *RuntimeManager) SetDefaultRuntime(name string) error {
	rtmgr.Lock()
	defer rtmgr.Unlock()

	if name == rtmgr.defaultRuntime {
		return nil
	}

	if _, ok := rtmgr.cache[name]; !ok {
		plugin, err := rtmgr.plugingetter.Get(name, RuntimeDriverName, getter.Lookup)
		if err != nil {
			return ErrRuntimeNotFound(name)
		}

		// We cache the new runtime
		driver, err := runtimeDriverFromPlugin(plugin)
		if err != nil {
			return err
		}

		rtmgr.cache[name] = driver
	}

	// TODO verify that the default runtime is still there.
	defaultRuntime, ok := rtmgr.cache[rtmgr.defaultRuntime]
	if ok {
		defaultRuntime.setDefaultRuntime(false)
	}

	// New default
	rtmgr.defaultRuntime = name
	rtmgr.cache[rtmgr.defaultRuntime].setDefaultRuntime(true)

	return nil
}

func (rtmgr *RuntimeManager) GetPluginList() []string {
	if rtmgr.plugingetter == nil {
		return nil
	}

	var pluginList []string

	runtimePlugins := rtmgr.plugingetter.GetAllManagedPluginsByCap(RuntimeDriverName)

	for _, plugin := range runtimePlugins {
		pluginList = append(pluginList, plugin.Name())
	}

	return pluginList
}
