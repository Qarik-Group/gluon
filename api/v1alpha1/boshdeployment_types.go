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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// VariableSource defines where variables for a deployment come from
type VariableSource struct {
	ConfigMap ConfigMapVariableSource `json:"configMap"`
	Secret    SecretVariableSource    `json:"secret"`

	Name  string `json:"name"`
	Value string `json:"value"`
}

// ConfigMapVariableSource ties a VariableSource to a ConfigMap
type ConfigMapVariableSource struct {
	Name string `json:"name"`
}

// SecretVariableSource ties a VariableSource to a Secret
type SecretVariableSource struct {
	Name string `json:"name"`
}

// BOSHDeploymentSpec defines the desired state of BOSHDeployment
type BOSHDeploymentSpec struct {
	Repo       string           `json:"repo"`
	Ref        string           `json:"ref"`
	Entrypoint string           `json:"entrypoint"`
	Ops        []string         `json:"ops"`
	Vars       []VariableSource `json:"vars"`
}

// BOSHDeploymentStatus defines the observed state of BOSHDeployment
type BOSHDeploymentStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// BOSHDeployment is the Schema for the boshdeployments API
type BOSHDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BOSHDeploymentSpec   `json:"spec,omitempty"`
	Status BOSHDeploymentStatus `json:"status,omitempty"`
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
