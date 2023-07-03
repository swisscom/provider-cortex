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

// RuleGroupParameters are the configurable fields of a RuleGroup.
type RuleGroupParameters struct {
	// The ruler API uses the concept of a “namespace” when creating rule groups.
	// This is a stand in for the name of the rule file in Prometheus and rule
	// groups must be named uniquely within a namespace.
	// This property is required.
	// +immutable
	Namespace string `json:"namespace"`

	// How often rules in the group are evaluated.
	// +optional
	Interval *string `json:"interval,omitempty"`

	// Limit the number of alerts an alerting rule and series a recording
	// rule can produce.
	// +optional
	// Limit    *int           `json:"limit,omitempty"`

	// Recording and alerting rules exist in a rule group. Rules within a group
	// are run sequentially at a regular interval, with the same evaluation
	// time.
	// https://prometheus.io/docs/prometheus/latest/configuration/recording_rules/#recording-rules
	// This property is required.
	Rules []RuleNode `json:"rules"`
}

type RuleNode struct {
	// The name of the time series to output to. Must be a valid metric name.
	// Either 'Record' or 'Alert' is required
	// +optional
	Record *string `json:"record,omitempty"`

	// The name of the alert. Must be a valid label value.
	// Either 'Record' or 'Alert' is required
	// +optional
	Alert *string `json:"alert,omitempty"`

	// The PromQL expression to evaluate. Every evaluation cycle this is
	// evaluated at the current time, and the result recorded as a new set of
	// time series with the metric name as given by 'record', or if an 'alert'
	// is provided all resultant time series become pending/firing alerts.
	// This property is required.
	Expr string `json:"expr"`

	// Alerts are considered firing once they have been returned for this long.
	// Alerts which have not yet fired for long enough are considered pending.
	// +optional
	For *string `json:"for,omitempty"`

	// Labels to add or overwrite
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations to add to each alert.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// How long an alert will continue firing after the condition that triggered it
	// has cleared.
	// KeepFiringFor model.Duration    `json:"keep_firing_for,omitempty"`
}

// RuleGroupObservation are the observable fields of a RuleGroup.
type RuleGroupObservation struct {
	Status    string `json:"status,omitempty"`
	Data      string `json:"data,omitempty"`
	ErrorType string `json:"errorType,omitempty"`
	Error     string `json:"error,omitempty"`
}

// A RuleGroupSpec defines the desired state of a RuleGroup.
type RuleGroupSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       RuleGroupParameters `json:"forProvider"`
}

// A RuleGroupStatus represents the observed state of a RuleGroup.
type RuleGroupStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          RuleGroupObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A RuleGroup is an example API type.
// +kubebuilder:printcolumn:name="READY",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="EXTERNAL-NAME",type="string",JSONPath=".metadata.annotations.crossplane\\.io/external-name"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster,categories={crossplane,managed,cortex}
type RuleGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RuleGroupSpec   `json:"spec"`
	Status RuleGroupStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RuleGroupList contains a list of RuleGroup
type RuleGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RuleGroup `json:"items"`
}

// RuleGroup type metadata.
var (
	RuleGroupKind             = reflect.TypeOf(RuleGroup{}).Name()
	RuleGroupGroupKind        = schema.GroupKind{Group: Group, Kind: RuleGroupKind}.String()
	RuleGroupKindAPIVersion   = RuleGroupKind + "." + SchemeGroupVersion.String()
	RuleGroupGroupVersionKind = SchemeGroupVersion.WithKind(RuleGroupKind)
)

func init() {
	SchemeBuilder.Register(&RuleGroup{}, &RuleGroupList{})
}
