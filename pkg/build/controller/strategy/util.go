package strategy

import (
	"os"
	"path"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

// setupDockerSocket configures the pod to support the host's Docker socket
func setupDockerSocket(podSpec *kapi.Pod) {
	dockerSocketVolume := kapi.Volume{
		Name: "docker-socket",
		Source: &kapi.VolumeSource{
			HostDir: &kapi.HostDir{
				Path: "/var/run/docker.sock",
			},
		},
	}

	dockerSocketVolumeMount := kapi.VolumeMount{
		Name:      "docker-socket",
		MountPath: "/var/run/docker.sock",
	}

	podSpec.Spec.Volumes = append(podSpec.Spec.Volumes,
		dockerSocketVolume)
	podSpec.Spec.Containers[0].VolumeMounts =
		append(podSpec.Spec.Containers[0].VolumeMounts,
			dockerSocketVolumeMount)
}

// setupDockerConfig configures the path to .dockercfg which contains registry credentials
func setupDockerConfig(podSpec *kapi.Pod) {
	dockerConfig := path.Join(os.Getenv("HOME"), ".dockercfg")
	if _, err := os.Stat(dockerConfig); os.IsNotExist(err) {
		return
	}
	dockerConfigVolume := kapi.Volume{
		Name: "docker-cfg",
		Source: &kapi.VolumeSource{
			HostDir: &kapi.HostDir{
				Path: dockerConfig,
			},
		},
	}

	dockerConfigVolumeMount := kapi.VolumeMount{
		Name:      "docker-cfg",
		ReadOnly:  true,
		MountPath: "/root/.dockercfg",
	}

	podSpec.Spec.Volumes = append(podSpec.Spec.Volumes,
		dockerConfigVolume)
	podSpec.Spec.Containers[0].VolumeMounts =
		append(podSpec.Spec.Containers[0].VolumeMounts,
			dockerConfigVolumeMount)
}
