/*
Copyright 2024.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WeaviateSpec defines the desired state of Weaviate
type WeaviateSpec struct {
	Replicas int32 `json:"replicas,omitempty"`

	// TODO: maybe gRPCService.<>
	EnableGRPCService bool `json:"enableGRPCService,omitempty"`

	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
}

// WeaviateStatus defines the observed state of Weaviate
type WeaviateStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	Ready      bool               `json:"ready"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Weaviate is the Schema for the weaviates API
type Weaviate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WeaviateSpec   `json:"spec,omitempty"`
	Status WeaviateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WeaviateList contains a list of Weaviate
type WeaviateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Weaviate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Weaviate{}, &WeaviateList{})
}
