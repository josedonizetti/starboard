package trivy_test

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/aquasecurity/starboard/pkg/apis/aquasecurity/v1alpha1"
	"github.com/aquasecurity/starboard/pkg/ext"
	"github.com/aquasecurity/starboard/pkg/plugin/trivy"
	"github.com/aquasecurity/starboard/pkg/starboard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var (
	fixedTime  = time.Now()
	fixedClock = ext.NewFixedClock(fixedTime)
)

func TestScanner_GetScanJobSpec(t *testing.T) {

	testCases := []struct {
		name string

		config       starboard.ConfigData
		workloadSpec corev1.PodSpec

		expectedSecrets []corev1.Secret
		expectedJobSpec corev1.PodSpec
	}{
		{
			name: "Standalone mode without insecure registry",
			config: starboard.ConfigData{
				"trivy.imageRef": "docker.io/aquasec/trivy:0.14.0",
				"trivy.mode":     string(starboard.Standalone),
			},
			workloadSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:1.16",
					},
				},
			},
			expectedJobSpec: corev1.PodSpec{
				Affinity:                     starboard.LinuxNodeAffinity(),
				RestartPolicy:                corev1.RestartPolicyNever,
				AutomountServiceAccountToken: pointer.BoolPtr(false),
				Volumes: []corev1.Volume{
					{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{
								Medium: corev1.StorageMediumDefault,
							},
						},
					},
				},
				InitContainers: []corev1.Container{
					{
						Name:                     "00000000-0000-0000-0000-000000000001",
						Image:                    "docker.io/aquasec/trivy:0.14.0",
						ImagePullPolicy:          corev1.PullIfNotPresent,
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Env: []corev1.EnvVar{
							{
								Name: "HTTP_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "HTTPS_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpsProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "NO_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.noProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},

							{
								Name: "GITHUB_TOKEN",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.SecretName,
										},
										Key:      "trivy.githubToken",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
						},
						Command: []string{
							"trivy",
						},
						Args: []string{
							"--download-db-only",
							"--cache-dir", "/var/lib/trivy",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100M"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500M"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "data",
								MountPath: "/var/lib/trivy",
								ReadOnly:  false,
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:                     "nginx",
						Image:                    "docker.io/aquasec/trivy:0.14.0",
						ImagePullPolicy:          corev1.PullIfNotPresent,
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Env: []corev1.EnvVar{
							{
								Name: "TRIVY_SEVERITY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.severity",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_IGNORE_UNFIXED",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.ignoreUnfixed",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SKIP_FILES",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.skipFiles",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SKIP_DIRS",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.skipDirs",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "HTTP_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "HTTPS_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpsProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "NO_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.noProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
						},
						Command: []string{
							"trivy",
						},
						Args: []string{
							"--skip-update",
							"--cache-dir", "/var/lib/trivy",
							"--quiet",
							"--format", "json",
							"nginx:1.16",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100M"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500M"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "data",
								ReadOnly:  false,
								MountPath: "/var/lib/trivy",
							},
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged:               pointer.BoolPtr(false),
							AllowPrivilegeEscalation: pointer.BoolPtr(false),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"all"},
							},
							ReadOnlyRootFilesystem: pointer.BoolPtr(true),
						},
					},
				},
				SecurityContext: &corev1.PodSecurityContext{},
			},
		},
		{
			name: "Standalone mode with insecure registry",
			config: starboard.ConfigData{
				"trivy.imageRef":                     "docker.io/aquasec/trivy:0.14.0",
				"trivy.mode":                         string(starboard.Standalone),
				"trivy.insecureRegistry.pocRegistry": "poc.myregistry.harbor.com.pl",
			},
			workloadSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "poc.myregistry.harbor.com.pl/nginx:1.16",
					},
				},
			},
			expectedJobSpec: corev1.PodSpec{
				Affinity:                     starboard.LinuxNodeAffinity(),
				RestartPolicy:                corev1.RestartPolicyNever,
				AutomountServiceAccountToken: pointer.BoolPtr(false),
				Volumes: []corev1.Volume{
					{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{
								Medium: corev1.StorageMediumDefault,
							},
						},
					},
				},
				InitContainers: []corev1.Container{
					{
						Name:                     "00000000-0000-0000-0000-000000000001",
						Image:                    "docker.io/aquasec/trivy:0.14.0",
						ImagePullPolicy:          corev1.PullIfNotPresent,
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Env: []corev1.EnvVar{
							{
								Name: "HTTP_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "HTTPS_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpsProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "NO_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.noProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "GITHUB_TOKEN",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.SecretName,
										},
										Key:      "trivy.githubToken",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
						},
						Command: []string{
							"trivy",
						},
						Args: []string{
							"--download-db-only",
							"--cache-dir", "/var/lib/trivy",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100M"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500M"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "data",
								MountPath: "/var/lib/trivy",
								ReadOnly:  false,
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:                     "nginx",
						Image:                    "docker.io/aquasec/trivy:0.14.0",
						ImagePullPolicy:          corev1.PullIfNotPresent,
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Env: []corev1.EnvVar{
							{
								Name: "TRIVY_SEVERITY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.severity",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_IGNORE_UNFIXED",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.ignoreUnfixed",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SKIP_FILES",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.skipFiles",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SKIP_DIRS",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.skipDirs",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "HTTP_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "HTTPS_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpsProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "NO_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.noProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name:  "TRIVY_INSECURE",
								Value: "true",
							},
						},
						Command: []string{
							"trivy",
						},
						Args: []string{
							"--skip-update",
							"--cache-dir", "/var/lib/trivy",
							"--quiet",
							"--format", "json",
							"poc.myregistry.harbor.com.pl/nginx:1.16",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100M"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500M"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "data",
								ReadOnly:  false,
								MountPath: "/var/lib/trivy",
							},
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged:               pointer.BoolPtr(false),
							AllowPrivilegeEscalation: pointer.BoolPtr(false),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"all"},
							},
							ReadOnlyRootFilesystem: pointer.BoolPtr(true),
						},
					},
				},
				SecurityContext: &corev1.PodSecurityContext{},
			},
		},
		{
			name: "Standalone mode with trivyignore file",
			config: starboard.ConfigData{
				"trivy.imageRef": "docker.io/aquasec/trivy:0.14.0",
				"trivy.mode":     string(starboard.Standalone),
				"trivy.ignoreFile": `# Accept the risk
CVE-2018-14618

# No impact in our settings
CVE-2019-1543`,
			},
			workloadSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:1.16",
					},
				},
			},
			expectedJobSpec: corev1.PodSpec{
				Affinity:                     starboard.LinuxNodeAffinity(),
				RestartPolicy:                corev1.RestartPolicyNever,
				AutomountServiceAccountToken: pointer.BoolPtr(false),
				Volumes: []corev1.Volume{
					{
						Name: "data",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{
								Medium: corev1.StorageMediumDefault,
							},
						},
					},
					{
						Name: "ignorefile",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: starboard.ConfigMapName,
								},
								Items: []corev1.KeyToPath{
									{
										Key:  "trivy.ignoreFile",
										Path: ".trivyignore",
									},
								},
							},
						},
					},
				},
				InitContainers: []corev1.Container{
					{
						Name:                     "00000000-0000-0000-0000-000000000001",
						Image:                    "docker.io/aquasec/trivy:0.14.0",
						ImagePullPolicy:          corev1.PullIfNotPresent,
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Env: []corev1.EnvVar{
							{
								Name: "HTTP_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "HTTPS_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpsProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "NO_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.noProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},

							{
								Name: "GITHUB_TOKEN",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.SecretName,
										},
										Key:      "trivy.githubToken",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
						},
						Command: []string{
							"trivy",
						},
						Args: []string{
							"--download-db-only",
							"--cache-dir", "/var/lib/trivy",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100M"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500M"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "data",
								MountPath: "/var/lib/trivy",
								ReadOnly:  false,
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:                     "nginx",
						Image:                    "docker.io/aquasec/trivy:0.14.0",
						ImagePullPolicy:          corev1.PullIfNotPresent,
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Env: []corev1.EnvVar{
							{
								Name: "TRIVY_SEVERITY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.severity",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_IGNORE_UNFIXED",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.ignoreUnfixed",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SKIP_FILES",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.skipFiles",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SKIP_DIRS",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.skipDirs",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "HTTP_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "HTTPS_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpsProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "NO_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.noProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name:  "TRIVY_IGNOREFILE",
								Value: "/tmp/trivy/.trivyignore",
							},
						},
						Command: []string{
							"trivy",
						},
						Args: []string{
							"--skip-update",
							"--cache-dir", "/var/lib/trivy",
							"--quiet",
							"--format", "json",
							"nginx:1.16",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100M"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500M"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "data",
								ReadOnly:  false,
								MountPath: "/var/lib/trivy",
							},
							{
								Name:      "ignorefile",
								MountPath: "/tmp/trivy/.trivyignore",
								SubPath:   ".trivyignore",
							},
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged:               pointer.BoolPtr(false),
							AllowPrivilegeEscalation: pointer.BoolPtr(false),
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{"all"},
							},
							ReadOnlyRootFilesystem: pointer.BoolPtr(true),
						},
					},
				},
				SecurityContext: &corev1.PodSecurityContext{},
			},
		},
		{
			name: "ClientServer mode without insecure registry",
			config: starboard.ConfigData{
				"trivy.imageRef":  "docker.io/aquasec/trivy:0.14.0",
				"trivy.mode":      string(starboard.ClientServer),
				"trivy.serverURL": "http://trivy.trivy:4954",
			},
			workloadSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:1.16",
					},
				},
			},
			expectedJobSpec: corev1.PodSpec{
				RestartPolicy:                corev1.RestartPolicyNever,
				AutomountServiceAccountToken: pointer.BoolPtr(false),
				Containers: []corev1.Container{
					{
						Name:                     "nginx",
						Image:                    "docker.io/aquasec/trivy:0.14.0",
						ImagePullPolicy:          corev1.PullIfNotPresent,
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Env: []corev1.EnvVar{
							{
								Name: "HTTP_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "HTTPS_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpsProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "NO_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.noProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SEVERITY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.severity",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_IGNORE_UNFIXED",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.ignoreUnfixed",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SKIP_FILES",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.skipFiles",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SKIP_DIRS",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.skipDirs",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_TOKEN_HEADER",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.SecretName,
										},
										Key:      "trivy.serverTokenHeader",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_TOKEN",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.SecretName,
										},
										Key:      "trivy.serverToken",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_CUSTOM_HEADERS",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.SecretName,
										},
										Key:      "trivy.serverCustomHeaders",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
						},
						Command: []string{
							"trivy",
						},
						Args: []string{
							"--quiet",
							"client",
							"--format",
							"json",
							"--remote",
							"http://trivy.trivy:4954",
							"nginx:1.16",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100M"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500M"),
							},
						},
					},
				},
			},
		},
		{
			name: "ClientServer mode with insecure registry",
			config: starboard.ConfigData{
				"trivy.imageRef":                     "docker.io/aquasec/trivy:0.14.0",
				"trivy.mode":                         string(starboard.ClientServer),
				"trivy.serverURL":                    "http://trivy.trivy:4954",
				"trivy.insecureRegistry.pocRegistry": "poc.myregistry.harbor.com.pl",
			},
			workloadSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "poc.myregistry.harbor.com.pl/nginx:1.16",
					},
				},
			},
			expectedJobSpec: corev1.PodSpec{
				RestartPolicy:                corev1.RestartPolicyNever,
				AutomountServiceAccountToken: pointer.BoolPtr(false),
				Containers: []corev1.Container{
					{
						Name:                     "nginx",
						Image:                    "docker.io/aquasec/trivy:0.14.0",
						ImagePullPolicy:          corev1.PullIfNotPresent,
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Env: []corev1.EnvVar{
							{
								Name: "HTTP_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "HTTPS_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpsProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "NO_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.noProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SEVERITY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.severity",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_IGNORE_UNFIXED",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.ignoreUnfixed",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SKIP_FILES",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.skipFiles",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SKIP_DIRS",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.skipDirs",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_TOKEN_HEADER",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.SecretName,
										},
										Key:      "trivy.serverTokenHeader",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_TOKEN",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.SecretName,
										},
										Key:      "trivy.serverToken",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_CUSTOM_HEADERS",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.SecretName,
										},
										Key:      "trivy.serverCustomHeaders",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name:  "TRIVY_INSECURE",
								Value: "true",
							},
						},
						Command: []string{
							"trivy",
						},
						Args: []string{
							"--quiet",
							"client",
							"--format",
							"json",
							"--remote",
							"http://trivy.trivy:4954",
							"poc.myregistry.harbor.com.pl/nginx:1.16",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100M"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500M"),
							},
						},
					},
				},
			},
		},
		{
			name: "ClientServer mode with trivyignore file",
			config: starboard.ConfigData{
				"trivy.imageRef":  "docker.io/aquasec/trivy:0.14.0",
				"trivy.mode":      string(starboard.ClientServer),
				"trivy.serverURL": "http://trivy.trivy:4954",
				"trivy.ignoreFile": `# Accept the risk
CVE-2018-14618

# No impact in our settings
CVE-2019-1543`,
			},
			workloadSpec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "nginx",
						Image: "nginx:1.16",
					},
				},
			},
			expectedJobSpec: corev1.PodSpec{
				RestartPolicy:                corev1.RestartPolicyNever,
				AutomountServiceAccountToken: pointer.BoolPtr(false),
				Volumes: []corev1.Volume{
					{
						Name: "ignorefile",
						VolumeSource: corev1.VolumeSource{
							ConfigMap: &corev1.ConfigMapVolumeSource{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: starboard.ConfigMapName,
								},
								Items: []corev1.KeyToPath{
									{
										Key:  "trivy.ignoreFile",
										Path: ".trivyignore",
									},
								},
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:                     "nginx",
						Image:                    "docker.io/aquasec/trivy:0.14.0",
						ImagePullPolicy:          corev1.PullIfNotPresent,
						TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
						Env: []corev1.EnvVar{
							{
								Name: "HTTP_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "HTTPS_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.httpsProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "NO_PROXY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.noProxy",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SEVERITY",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.severity",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_IGNORE_UNFIXED",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.ignoreUnfixed",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SKIP_FILES",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.skipFiles",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_SKIP_DIRS",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.ConfigMapName,
										},
										Key:      "trivy.skipDirs",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_TOKEN_HEADER",
								ValueFrom: &corev1.EnvVarSource{
									ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.SecretName,
										},
										Key:      "trivy.serverTokenHeader",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_TOKEN",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.SecretName,
										},
										Key:      "trivy.serverToken",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name: "TRIVY_CUSTOM_HEADERS",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: starboard.SecretName,
										},
										Key:      "trivy.serverCustomHeaders",
										Optional: pointer.BoolPtr(true),
									},
								},
							},
							{
								Name:  "TRIVY_IGNOREFILE",
								Value: "/tmp/trivy/.trivyignore",
							},
						},
						Command: []string{
							"trivy",
						},
						Args: []string{
							"--quiet",
							"client",
							"--format",
							"json",
							"--remote",
							"http://trivy.trivy:4954",
							"nginx:1.16",
						},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("100m"),
								corev1.ResourceMemory: resource.MustParse("100M"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("500M"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "ignorefile",
								MountPath: "/tmp/trivy/.trivyignore",
								SubPath:   ".trivyignore",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pluginContext := starboard.NewPluginContext().
				WithName(string(starboard.Trivy)).
				WithNamespace("starboard-ns").
				WithServiceAccountName("starboard-sa").
				Get()
			instance := trivy.NewPlugin(fixedClock, ext.NewSimpleIDGenerator(), tc.config)
			jobSpec, secrets, err := instance.GetScanJobSpec(pluginContext, tc.workloadSpec, nil)
			require.NoError(t, err)
			assert.Empty(t, secrets)
			assert.Equal(t, tc.expectedJobSpec, jobSpec)
		})
	}

}

var (
	sampleReportAsString = `[{
		"Target": "alpine:3.10.2 (alpine 3.10.2)",
		"Type": "alpine",
		"Vulnerabilities": [
			{
				"VulnerabilityID": "CVE-2019-1549",
				"PkgName": "openssl",
				"InstalledVersion": "1.1.1c-r0",
				"FixedVersion": "1.1.1d-r0",
				"Title": "openssl: information disclosure in fork()",
				"Description": "Usually this long long description of CVE-2019-1549",
				"Severity": "MEDIUM",
				"PrimaryURL": "https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2019-1549",
				"References": [
					"https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2019-1549"
				]
			},
			{
				"VulnerabilityID": "CVE-2019-1547",
				"PkgName": "openssl",
				"InstalledVersion": "1.1.1c-r0",
				"FixedVersion": "1.1.1d-r0",
				"Title": "openssl: side-channel weak encryption vulnerability",
				"Severity": "LOW",
				"PrimaryURL": "https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2019-1547",
				"References": [
					"https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2019-1547"
				]
			}
		]
	}]`

	sampleReport = v1alpha1.VulnerabilityScanResult{
		UpdateTimestamp: metav1.NewTime(fixedTime),
		Scanner: v1alpha1.Scanner{
			Name:    "Trivy",
			Vendor:  "Aqua Security",
			Version: "0.9.1",
		},
		Registry: v1alpha1.Registry{
			Server: "index.docker.io",
		},
		Artifact: v1alpha1.Artifact{
			Repository: "library/alpine",
			Tag:        "3.10.2",
		},
		Summary: v1alpha1.VulnerabilitySummary{
			CriticalCount: 0,
			MediumCount:   1,
			LowCount:      1,
			NoneCount:     0,
			UnknownCount:  0,
		},
		Vulnerabilities: []v1alpha1.Vulnerability{
			{
				VulnerabilityID:  "CVE-2019-1549",
				Resource:         "openssl",
				InstalledVersion: "1.1.1c-r0",
				FixedVersion:     "1.1.1d-r0",
				Severity:         v1alpha1.SeverityMedium,
				Title:            "openssl: information disclosure in fork()",
				PrimaryLink:      "https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2019-1549",
				Links:            []string{},
			},
			{
				VulnerabilityID:  "CVE-2019-1547",
				Resource:         "openssl",
				InstalledVersion: "1.1.1c-r0",
				FixedVersion:     "1.1.1d-r0",
				Severity:         v1alpha1.SeverityLow,
				Title:            "openssl: side-channel weak encryption vulnerability",
				PrimaryLink:      "https://cve.mitre.org/cgi-bin/cvename.cgi?name=CVE-2019-1547",
				Links:            []string{},
			},
		},
	}
)

func TestScanner_ParseVulnerabilityReportData(t *testing.T) {
	config := starboard.ConfigData{
		"trivy.imageRef": "aquasec/trivy:0.9.1",
	}

	testCases := []struct {
		name           string
		imageRef       string
		input          string
		expectedError  error
		expectedReport v1alpha1.VulnerabilityScanResult
	}{
		{
			name:           "Should convert vulnerability report in JSON format when input is quiet",
			imageRef:       "alpine:3.10.2",
			input:          sampleReportAsString,
			expectedError:  nil,
			expectedReport: sampleReport,
		},
		{
			name:          "Should convert vulnerability report in JSON format when OS is not detected",
			imageRef:      "core.harbor.domain/library/nginx@sha256:d20aa6d1cae56fd17cd458f4807e0de462caf2336f0b70b5eeb69fcaaf30dd9c",
			input:         `null`,
			expectedError: nil,
			expectedReport: v1alpha1.VulnerabilityScanResult{
				UpdateTimestamp: metav1.NewTime(fixedTime),
				Scanner: v1alpha1.Scanner{
					Name:    "Trivy",
					Vendor:  "Aqua Security",
					Version: "0.9.1",
				},
				Registry: v1alpha1.Registry{
					Server: "core.harbor.domain",
				},
				Artifact: v1alpha1.Artifact{
					Repository: "library/nginx",
					Digest:     "sha256:d20aa6d1cae56fd17cd458f4807e0de462caf2336f0b70b5eeb69fcaaf30dd9c",
				},
				Summary: v1alpha1.VulnerabilitySummary{
					CriticalCount: 0,
					HighCount:     0,
					MediumCount:   0,
					LowCount:      0,
					NoneCount:     0,
					UnknownCount:  0,
				},
				Vulnerabilities: []v1alpha1.Vulnerability{},
			},
		},
		{
			name:          "Should return error when image reference cannot be parsed",
			imageRef:      ":",
			input:         "null",
			expectedError: errors.New("could not parse reference: :"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			instance := trivy.NewPlugin(fixedClock, ext.NewSimpleIDGenerator(), config)
			report, err := instance.ParseVulnerabilityReportData(tc.imageRef, io.NopCloser(strings.NewReader(tc.input)))
			switch {
			case tc.expectedError == nil:
				require.NoError(t, err)
				assert.Equal(t, tc.expectedReport, report)
			default:
				assert.EqualError(t, err, tc.expectedError.Error())
			}
		})
	}

}

func TestGetScoreFromCVSS(t *testing.T) {
	testCases := []struct {
		name          string
		cvss          map[string]*trivy.CVSS
		expectedScore *float64
	}{
		{
			name: "Should return vendor score when vendor v3 score exist",
			cvss: map[string]*trivy.CVSS{
				"nvd": {
					V3Score: pointer.Float64Ptr(8.1),
				},
				"redhat": {
					V3Score: pointer.Float64Ptr(8.3),
				},
			},
			expectedScore: pointer.Float64Ptr(8.3),
		},
		{
			name: "Should return nvd score when vendor v3 score is nil",
			cvss: map[string]*trivy.CVSS{
				"nvd": {
					V3Score: pointer.Float64Ptr(8.1),
				},
				"redhat": {
					V3Score: nil,
				},
			},
			expectedScore: pointer.Float64Ptr(8.1),
		},
		{
			name: "Should return nvd score when vendor doesn't exist",
			cvss: map[string]*trivy.CVSS{
				"nvd": {
					V3Score: pointer.Float64Ptr(8.1),
				},
			},
			expectedScore: pointer.Float64Ptr(8.1),
		},
		{
			name: "Should return nil when vendor and nvd both v3 scores are nil",
			cvss: map[string]*trivy.CVSS{
				"nvd": {
					V3Score: nil,
				},
				"redhat": {
					V3Score: nil,
				},
			},
			expectedScore: nil,
		},
		{
			name:          "Should return nil when cvss doesn't exist",
			cvss:          map[string]*trivy.CVSS{},
			expectedScore: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			score := trivy.GetScoreFromCVSS(tc.cvss)
			assert.Equal(t, tc.expectedScore, score)
		})
	}
}
