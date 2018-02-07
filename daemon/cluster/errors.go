package cluster // import "github.com/docker/docker/daemon/cluster"

const (
	// errNoSwarm is returned on leaving a cluster that was never initialized
	errNoSwarm notAvailableError = "This node is not part of a swarm"

	// errSwarmExists is returned on initialize or join request for a cluster that has already been activated
	errSwarmExists notAvailableError = "This node is already part of a swarm. Use \"docker swarm leave\" to leave this swarm and join another one."

	// errSwarmJoinTimeoutReached is returned when cluster join could not complete before timeout was reached.
	errSwarmJoinTimeoutReached notAvailableError = "Timeout was reached before node joined. The attempt to join the swarm will continue in the background. Use the \"docker info\" command to see the current swarm status of your node."

	// errSwarmLocked is returned if the swarm is encrypted and needs a key to unlock it.
	errSwarmLocked notAvailableError = "Swarm is encrypted and needs to be unlocked before it can be used. Please use \"docker swarm unlock\" to unlock it."

	// errSwarmCertificatesExpired is returned if docker was not started for the whole validity period and they had no chance to renew automatically.
	errSwarmCertificatesExpired notAvailableError = "Swarm certificates have expired. To replace them, leave the swarm and join again."

	// errSwarmNotManager is returned if the node is not a swarm manager.
	errSwarmNotManager notAvailableError = "This node is not a swarm manager. Worker nodes can't be used to view or modify cluster state. Please run this command on a manager node or promote the current node to a manager."
)

type notAllowedError string

func (e notAllowedError) Error() string {
	return string(e)
}

func (e notAllowedError) Forbidden() {}

type notAvailableError string

func (e notAvailableError) Error() string {
	return string(e)
}

func (e notAvailableError) Unavailable() {}

type configError string

func (e configError) Error() string {
	return string(e)
}

func (e configError) InvalidParameter() {}

type invalidUnlockKey struct{}

func (invalidUnlockKey) Error() string {
	return "swarm could not be unlocked: invalid key provided"
}

func (invalidUnlockKey) Unauthorized() {}

type notLockedError struct{}

func (notLockedError) Error() string {
	return "swarm is not locked"
}

func (notLockedError) Conflict() {}
