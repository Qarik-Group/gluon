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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BOSHStemcellSpec defines the desired state of BOSHStemcell
type BOSHStemcellSpec struct {
	UploadTo map[string]string `json:"uploadTo"`

	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	URL     string `json:"url"`
	SHA1    string `json:"sha1"`
	Fix     bool   `json:"fix,omitempty"`
}

// BOSHStemcellStatus defines the observed state of BOSHStemcell
type BOSHStemcellStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true

// BOSHStemcell is the Schema for the boshstemcells API
type BOSHStemcell struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BOSHStemcellSpec   `json:"spec,omitempty"`
	Status BOSHStemcellStatus `json:"status,omitempty"`
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
