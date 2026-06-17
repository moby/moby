package metadata

const (
	// CheckpointAnnotationEngine specifies the name of the container engine (e.g., podman, cri-o).
	CheckpointAnnotationEngine = "org.criu.checkpoint.engine.name"

	// CheckpointAnnotationEngineVersion specifies the version of the container engine.
	CheckpointAnnotationEngineVersion = "org.criu.checkpoint.engine.version"

	// CheckpointAnnotationName specifies the name of the container associated with the checkpoint.
	CheckpointAnnotationName = "org.criu.checkpoint.container.name"

	// CheckpointAnnotationPod specifies the name of the pod associated with the checkpoint.
	CheckpointAnnotationPod = "org.criu.checkpoint.pod.name"

	// CheckpointAnnotationPodID specifies the ID of the pod associated with the checkpoint.
	CheckpointAnnotationPodID = "org.criu.checkpoint.pod.id"

	// CheckpointAnnotationNamespace specifies the namespace of the pod associated with the checkpoint.
	CheckpointAnnotationNamespace = "org.criu.checkpoint.pod.namespace"

	// CheckpointAnnotationRootfsImageName specifies the name of the root filesystem image associated with the checkpoint.
	CheckpointAnnotationRootfsImageName = "org.criu.checkpoint.rootfsImageName"

	// CheckpointAnnotationRootfsImageUserRequested specifies the name of the root filesystem image requested by the user.
	CheckpointAnnotationRootfsImageUserRequested = "org.criu.checkpoint.rootfsImageUserRequested"

	// CheckpointAnnotationRootfsImageSha specifies the SHA hash of the root filesystem image associated with the checkpoint.
	CheckpointAnnotationRootfsImageSha = "org.criu.checkpoint.rootfsImageSha"

	// CheckpointAnnotationRootfsImageID specifies the ID of the root filesystem image associated with the checkpoint.
	CheckpointAnnotationRootfsImageID = "org.criu.checkpoint.rootfsImageID"

	// CheckpointAnnotationRawImageName specifies the original unprocessed name of the image used to create the container.
	CheckpointAnnotationRawImageName = "org.criu.checkpoint.rawImageName"

	// CheckpointAnnotationRuntimeName specifies the runtime used on the host where the checkpoint was created.
	CheckpointAnnotationRuntimeName = "org.criu.checkpoint.runtime.name"

	// CheckpointAnnotationRuntimeVersion specifies the version of the runtime used on the host where the checkpoint was created.
	CheckpointAnnotationRuntimeVersion = "org.criu.checkpoint.runtime.version"

	// CheckpointAnnotationCriuVersion specifies the version of CRIU used on the host where the checkpoint was created.
	CheckpointAnnotationCriuVersion = "org.criu.checkpoint.criu.version"

	// CheckpointAnnotationConmonVersion specifies the version of conmon used on the host where the checkpoint was created.
	CheckpointAnnotationConmonVersion = "org.criu.checkpoint.conmon.version"

	// CheckpointAnnotationHostArch specifies the CPU architecture of the host where the checkpoint was created.
	CheckpointAnnotationHostArch = "org.criu.checkpoint.host.arch"

	// CheckpointAnnotationHostKernel specifies the kernel version used by the host where the checkpoint was created.
	CheckpointAnnotationHostKernel = "org.criu.checkpoint.host.kernel"

	// CheckpointAnnotationCgroupVersion specifies the cgroup version used by the host where the checkpoint was created.
	CheckpointAnnotationCgroupVersion = "org.criu.checkpoint.cgroup.version"

	// CheckpointAnnotationDistributionVersion specifies the name of the host distribution on which the checkpoint was created.
	// This annotation is particularly useful because some distributions may include non-upstream patches
	// that can cause CRIU (Checkpoint/Restore in Userspace) to fail.
	CheckpointAnnotationDistributionName = "org.criu.checkpoint.distribution.name"

	// CheckpointAnnotationDistributionVersion specifies the version of the host distribution on which the checkpoint was created.
	CheckpointAnnotationDistributionVersion = "org.criu.checkpoint.distribution.version"
)
