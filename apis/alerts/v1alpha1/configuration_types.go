/*
Copyright 2022 The Crossplane Authors.

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
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
)

// AlertManagerConfigurationParameters are the configurable fields of an AlertManagerConfiguration.
type AlertManagerConfigurationParameters struct {
	// Custom notification template definitions
	// +optional
	TemplateFiles map[string]string `json:"template_files,omitempty"`

	// Contains the alert manager configuration. This uses the same structure as
	// an alert manager config file in standalone Prometheus.
	// https://prometheus.io/docs/alerting/latest/configuration/
	AlertmanagerConfig string `json:"alertmanager_config"`
}

// AlertManagerConfigurationObservation are the observable fields of an AlertManagerConfiguration.
type AlertManagerConfigurationObservation struct {
	Status    string `json:"status,omitempty"`
	Data      string `json:"data,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
	Error     string `json:"error,omitempty"`
}

// A AlertManagerConfigurationSpec defines the desired state of an AlertManagerConfiguration.
type AlertManagerConfigurationSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       AlertManagerConfigurationParameters `json:"forProvider"`
}

// A AlertManagerConfigurationStatus represents the observed state of an AlertManagerConfiguration.
type AlertManagerConfigurationStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          AlertManagerConfigurationObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// An AlertManagerConfiguration is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cortex}
type AlertManagerConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AlertManagerConfigurationSpec   `json:"spec"`
	Status AlertManagerConfigurationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AlertManagerConfigurationList contains a list of AlertManagerConfiguration
type AlertManagerConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AlertManagerConfiguration `json:"items"`
}

// AlertManagerConfiguration type metadata.
var (
	AlertManagerConfigurationKind             = reflect.TypeOf(AlertManagerConfiguration{}).Name()
	AlertManagerConfigurationGroupKind        = schema.GroupKind{Group: Group, Kind: AlertManagerConfigurationKind}.String()
	AlertManagerConfigurationKindAPIVersion   = AlertManagerConfigurationKind + "." + SchemeGroupVersion.String()
	AlertManagerConfigurationGroupVersionKind = SchemeGroupVersion.WithKind(AlertManagerConfigurationKind)
)

func init() {
	SchemeBuilder.Register(&AlertManagerConfiguration{}, &AlertManagerConfigurationList{})
}
