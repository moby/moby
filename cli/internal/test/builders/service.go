package builders

import (
	"github.com/docker/docker/api/types/swarm"
)

// Service creates a service with default values.
// Any number of service builder functions can be passed to augment it.
// Currently, only ServiceName is implemented
func Service(builders ...func(*swarm.Service)) *swarm.Service {
	service := &swarm.Service{
		ID: "serviceID",
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: "defaultServiceName",
			},
		},
	}

	for _, builder := range builders {
		builder(service)
	}

	return service
}

// ServiceName sets the service name
func ServiceName(name string) func(*swarm.Service) {
	return func(service *swarm.Service) {
		service.Spec.Annotations.Name = name
	}
}
