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
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VariableSource defines where variables for a deployment come from
type VariableSource struct {
	ConfigMap *ConfigMapVariableSource `json:"configMap,omitempty"`
	Secret    *SecretVariableSource    `json:"secret,omitempty"`

	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// ConfigMapVariableSource ties a VariableSource to a ConfigMap
type ConfigMapVariableSource struct {
	Name    string            `json:"name"`
	MapKeys map[string]string `json:"mapKeys,omitempty"`
}

// SecretVariableSource ties a VariableSource to a Secret
type SecretVariableSource struct {
	Name    string            `json:"name"`
	MapKeys map[string]string `json:"mapKeys,omitempty"`
}

// BOSHDeploymentSpec defines the desired state of BOSHDeployment
type BOSHDeploymentSpec struct {
	Repo       string `json:"repo"`
	Ref        string `json:"ref"`
	Entrypoint string `json:"entrypoint"`

	Director string `json:"director,omitempty"`

	Ops  []string         `json:"ops,omitempty"`
	Vars []VariableSource `json:"vars,omitempty"`
}

// BOSHDeploymentStatus defines the observed state of BOSHDeployment
type BOSHDeploymentStatus struct {
	Ready bool   `json:"ready"`
	State string `json:"state"`
}

// +kubebuilder:object:root=true

// BOSHDeployment is the Schema for the boshdeployments API
// +kubebuilder:resource:path=boshdeployments,scope=Namespaced,shortName=bosh
type BOSHDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec         BOSHDeploymentSpec   `json:"spec,omitempty"`
	Dependencies DependencySpecs      `json:"dependencies,omitempty"`
	Status       BOSHDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BOSHDeploymentList contains a list of BOSHDeployment
type BOSHDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BOSHDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BOSHDeployment{}, &BOSHDeploymentList{})
}

func (bd *BOSHDeployment) StateVolumeName() string {
	return fmt.Sprintf("%s-state", bd.Name)
}

func (bd *BOSHDeployment) StateConfigMapName() string {
	return fmt.Sprintf("%s-state", bd.Name)
}

func (bd *BOSHDeployment) SecretsName() string {
	return fmt.Sprintf("%s-secrets", bd.Name)
}

func (bd *BOSHDeployment) JobName(verb string) string {
	if bd.Spec.Director != "" {
		return fmt.Sprintf("%s-%s-via-%s", verb, bd.Name, bd.Spec.Director)
	} else {
		return fmt.Sprintf("%s-%s-bosh", verb, bd.Name)
	}
}

func (bd *BOSHDeployment) StateVolume() *corev1.PersistentVolumeClaim {
	mode := corev1.PersistentVolumeFilesystem
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: bd.Namespace,
			Name:      bd.StateVolumeName(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			VolumeMode: &mode,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					"storage": resource.MustParse("1Mi"),
				},
			},
		},
	}
}

func (bd *BOSHDeployment) DeployJob() *batchv1.Job {
	return bd.job("deploy")
}

func (bd *BOSHDeployment) TeardownJob() *batchv1.Job {
	return bd.job("teardown")
}

func (bd *BOSHDeployment) job(verb string) *batchv1.Job {
	vars := []corev1.EnvVar{
		corev1.EnvVar{
			Name: "POD_NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "metadata.namespace",
				},
			},
		},
		corev1.EnvVar{
			Name:  "UPSTREAM_REPO",
			Value: bd.Spec.Repo,
		},
		corev1.EnvVar{
			Name:  "UPSTREAM_REF",
			Value: bd.Spec.Ref,
		},
		corev1.EnvVar{
			Name:  "UPSTREAM_ENTRYPOINT",
			Value: bd.Spec.Entrypoint,
		},
		corev1.EnvVar{
			Name:  "CREDS_SECRET_NAME",
			Value: bd.SecretsName(),
		},
		corev1.EnvVar{
			Name:  "CREDS_STATE_FILE_CONFIG_MAP",
			Value: bd.StateConfigMapName(),
		},
		corev1.EnvVar{
			Name:  "GLUON_director_name", // FIXME
			Value: bd.Name,
		},
	}

	if bd.Spec.Director != "" {
		secretName := fmt.Sprintf("%s-secrets", bd.Spec.Director)

		vars = append(vars, corev1.EnvVar{
			Name: "BOSH_ENVIRONMENT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: "endpoint",
				},
			},
		})
		vars = append(vars, corev1.EnvVar{
			Name: "BOSH_CLIENT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: "username",
				},
			},
		})
		vars = append(vars, corev1.EnvVar{
			Name: "BOSH_CLIENT_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: "password",
				},
			},
		})
		vars = append(vars, corev1.EnvVar{
			Name: "BOSH_CA_CERT",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: "ca",
				},
			},
		})

		// track original BOSHDeployment spec.Name as the deployment
		vars = append(vars, corev1.EnvVar{
			Name:  "BOSH_DEPLOYMENT",
			Value: bd.Name,
		})
	}

	// append ops files to command
	command := make([]string, len(bd.Spec.Ops)*2+1)
	command[0] = verb
	for i, op := range bd.Spec.Ops {
		file := op
		if !strings.HasSuffix(file, ".yml") && !strings.HasSuffix(file, ".yaml") {
			file = fmt.Sprintf("%s.yml", op)
		}

		command[1+i*2] = "-o"
		command[1+i*2+1] = file
	}

	volumes := []corev1.Volume{}
	mounts := []corev1.VolumeMount{}
	if bd.Spec.Director == "" {
		volumes = append(volumes, corev1.Volume{
			Name: "state",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: bd.StateVolumeName(),
				},
			},
		})
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "state",
			MountPath: "/bosh/state",
		})
	}

	// create the Job resource, in all of its glory
	var one int32 = 1
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: bd.Namespace,
			Name:      bd.JobName(verb),
		},
		Spec: batchv1.JobSpec{
			Parallelism:  &one,
			Completions:  &one,
			BackoffLimit: &one,
			//TTLSecondsAfterFinished
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Volumes:       volumes,
					Containers: []corev1.Container{
						corev1.Container{
							Name:            "deploy",
							Image:           GluonImage,
							ImagePullPolicy: GluonPullPolicy,
							VolumeMounts:    mounts,
							Command:         command,
							Env:             vars,
						},
					},
				},
			},
		},
	}
}
