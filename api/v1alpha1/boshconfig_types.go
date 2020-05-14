/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BOSHConfigSpec defines the desired state of BOSHConfig
type BOSHConfigSpec struct {
	Director string `json:"director"`

	Type   string `json:"type"`
	Config string `json:"config"`
}

// BOSHConfigStatus defines the observed state of BOSHConfig
type BOSHConfigStatus struct {
	Ready bool   `json:"ready"`
	State string `json:"state"`
}

// +kubebuilder:object:root=true

// BOSHConfig is the Schema for the boshconfigs API
// +kubebuilder:resource:path=boshconfigs,scope=Namespaced,shortName=bcc
type BOSHConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec         BOSHConfigSpec   `json:"spec,omitempty"`
	Dependencies DependencySpecs  `json:"dependencies,omitempty"`
	Status       BOSHConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BOSHConfigList contains a list of BOSHConfig
type BOSHConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BOSHConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BOSHConfig{}, &BOSHConfigList{})
}

func (bc *BOSHConfig) JobName(director *BOSHDeployment) string {
	return fmt.Sprintf("update-config-%s-on-%s", bc.Name, director.Name)
}

func (bc *BOSHConfig) Job(director *BOSHDeployment) *batchv1.Job {
	secret := director.SecretsName()

	command := []string{
		"bosh",
		"-n",
		fmt.Sprintf("update-%s-config", bc.Spec.Type),
	}
	if bc.Spec.Type != "cloud" {
		command = append(command, "--name")
		command = append(command, bc.Name)
	}
	command = append(command, "/bosh/config/config.yml")

	var one int32 = 1
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: bc.Namespace,
			Name:      bc.JobName(director),
		},
		Spec: batchv1.JobSpec{
			Parallelism:  &one,
			Completions:  &one,
			BackoffLimit: &one,
			//TTLSecondsAfterFinished
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes: []corev1.Volume{
						corev1.Volume{
							Name: "config",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: bc.Name,
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						corev1.Container{
							Name:            "update-config",
							Image:           GluonImage,
							ImagePullPolicy: GluonPullPolicy,
							Command:         command,
							VolumeMounts: []corev1.VolumeMount{
								corev1.VolumeMount{
									Name:      "config",
									MountPath: "/bosh/config",
									ReadOnly:  true,
								},
							},
							Env: []corev1.EnvVar{
								corev1.EnvVar{
									Name: "BOSH_ENVIRONMENT",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secret,
											},
											Key: "endpoint",
										},
									},
								},
								corev1.EnvVar{
									Name: "BOSH_CLIENT",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secret,
											},
											Key: "username",
										},
									},
								},
								corev1.EnvVar{
									Name: "BOSH_CLIENT_SECRET",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secret,
											},
											Key: "password",
										},
									},
								},
								corev1.EnvVar{
									Name: "BOSH_CA_CERT",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: secret,
											},
											Key: "ca",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
