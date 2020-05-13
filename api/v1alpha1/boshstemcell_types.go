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

// BOSHStemcellSpec defines the desired state of BOSHStemcell
type BOSHStemcellSpec struct {
	Director string `json:"director"`

	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	URL     string `json:"url"`
	SHA1    string `json:"sha1"`
	Fix     bool   `json:"fix,omitempty"`
}

// BOSHStemcellStatus defines the observed state of BOSHStemcell
type BOSHStemcellStatus struct {
	Ready bool   `json:"ready"`
	State string `json:"state"`
}

// +kubebuilder:object:root=true

// BOSHStemcell is the Schema for the boshstemcells API
// +kubebuilder:resource:path=boshstemcells,scope=Namespaced,shortName=stemcell;bsc
type BOSHStemcell struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec         BOSHStemcellSpec   `json:"spec,omitempty"`
	Dependencies DependencySpecs    `json:"dependencies,omitempty"`
	Status       BOSHStemcellStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BOSHStemcellList contains a list of BOSHStemcell
type BOSHStemcellList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BOSHStemcell `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BOSHStemcell{}, &BOSHStemcellList{})
}

func (bs *BOSHStemcell) JobName(director *BOSHDeployment) string {
	return fmt.Sprintf("upload-%s-to-%s", bs.Name, director.Name)
}

func (bs *BOSHStemcell) Job(director *BOSHDeployment) *batchv1.Job {
	secret := director.SecretsName()

	command := []string{
		"bosh",
		"upload-stemcell",
		bs.Spec.URL,
		"--sha1",
		bs.Spec.SHA1,
	}
	if bs.Spec.Name != "" {
		command = append(command, "--name")
		command = append(command, bs.Spec.Name)
	}
	if bs.Spec.Version != "" {
		command = append(command, "--version")
		command = append(command, bs.Spec.Version)
	}
	if bs.Spec.Fix {
		command = append(command, "--fix")
	}

	var one int32 = 1
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: bs.ObjectMeta.Namespace,
			Name:      bs.JobName(director),
		},
		Spec: batchv1.JobSpec{
			Parallelism:  &one,
			Completions:  &one,
			BackoffLimit: &one,
			//TTLSecondsAfterFinished
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						corev1.Container{
							Name:            "upload",
							Image:           "starkandwayne/bosh-create-env:latest",
							ImagePullPolicy: corev1.PullAlways,
							Command:         command,
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
