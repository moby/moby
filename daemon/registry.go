package daemon

// AddInsecureRegistry add insecure registry in the runtime
func (daemon *Daemon) AddInsecureRegistry(insecureRegistries []string) error {

	return daemon.RegistryService.AddInsecureRegistryService(insecureRegistries)
}
