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

package rulegroup

import (
	"context"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/pkg/rulefmt"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cortexClient "github.com/cortexproject/cortex-tools/pkg/client"
	"github.com/cortexproject/cortex-tools/pkg/rules/rwrulefmt"
	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-cortex/apis/rules/v1alpha1"
	apisv1alpha1 "github.com/crossplane/provider-cortex/apis/v1alpha1"
	xpClient "github.com/crossplane/provider-cortex/internal/clients"
	"github.com/crossplane/provider-cortex/internal/features"
)

const (
	errNotRuleGroup      = "managed resource is not a RuleGroup custom resource"
	errRuleGroupNotFound = "requested resource not found"
	errTrackPCUsage      = "cannot track ProviderConfig usage"
	errGetPC             = "cannot get ProviderConfig"
	errGetCreds          = "cannot get credentials"

	errNewClient = "cannot create new Service"
)

// Setup adds a controller that reconciles RuleGroup managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.RuleGroupGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.RuleGroupGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: xpClient.NewClient}),
		// managed.NewNameAsExternalName(c)
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		// WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.RuleGroup{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(config cortexClient.Config) *cortexClient.CortexClient
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.RuleGroup)
	if !ok {
		return nil, errors.New(errNotRuleGroup)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	config, err := xpClient.GetConfig(ctx, c.kube, cr)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{service: c.newServiceFn(*config)}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	// A 'client' used to connect to the external resource API
	service *cortexClient.CortexClient
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.RuleGroup)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotRuleGroup)
	}

	// if meta.GetExternalName(cr) == "" {
	// 	return managed.ExternalObservation{
	// 		ResourceExists: false,
	// 	}, nil
	// }

	observedRuleGroup, err := c.service.GetRuleGroup(ctx, cr.Spec.ForProvider.Namespace, meta.GetExternalName(cr))
	if err != nil {
		switch {
		case isErrRuleGroupNotFound(err):
			return managed.ExternalObservation{}, nil
		default:
			return managed.ExternalObservation{}, err
		}
	}

	if observedRuleGroup == nil {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		// Return false when the external resource does not exist. This lets
		// the managed resource reconciler know that it needs to call Create to
		// (re)create the resource, or that it has successfully been deleted.
		ResourceExists: true,

		// Return false when the external resource exists, but it not up to date
		// with the desired managed resource state. This lets the managed
		// resource reconciler know that it needs to call Update.
		ResourceUpToDate: isUpToDate(cr, observedRuleGroup),

		// Return any details that may be required to connect to the external
		// resource. These will be stored as the connection secret.
		// ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.RuleGroup)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotRuleGroup)
	}

	rns := []rulefmt.RuleNode{}

	// iterate through group rules
	for _, rule := range cr.Spec.ForProvider.Rules {
		rn, err := generateRuleNode(rule)
		if err != nil {
			return managed.ExternalCreation{}, err
		}

		rns = append(rns, *rn)
	}

	var interval model.Duration
	var err error

	if cr.Spec.ForProvider.Interval != nil {
		interval, err = model.ParseDuration(*cr.Spec.ForProvider.Interval)
		if err != nil {
			return managed.ExternalCreation{}, err
		}
	}

	rw := rwrulefmt.RuleGroup{
		RuleGroup: rulefmt.RuleGroup{
			Name:     cr.GetName(),
			Interval: interval,
			// Limit: cr.Spec.ForProvider.Limit,
			Rules: rns,
		},
	}

	err = c.service.CreateRuleGroup(ctx, cr.Spec.ForProvider.Namespace, rw)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.RuleGroup)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotRuleGroup)
	}

	rns := []rulefmt.RuleNode{}

	// iterate through group rules
	for _, rule := range cr.Spec.ForProvider.Rules {
		rn, err := generateRuleNode(rule)
		if err != nil {
			return managed.ExternalUpdate{}, err
		}

		rns = append(rns, *rn)
	}

	var interval model.Duration
	var err error

	if cr.Spec.ForProvider.Interval != nil {
		interval, err = model.ParseDuration(*cr.Spec.ForProvider.Interval)
		if err != nil {
			return managed.ExternalUpdate{}, err
		}
	}

	rw := rwrulefmt.RuleGroup{
		RuleGroup: rulefmt.RuleGroup{
			Name:     cr.GetName(),
			Interval: interval,
			// Limit: cr.Spec.ForProvider.Limit,
			Rules: rns,
		},
	}

	err = c.service.CreateRuleGroup(ctx, cr.Spec.ForProvider.Namespace, rw)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	return managed.ExternalUpdate{
		// Optionally return any details that may be required to connect to the
		// external resource. These will be stored as the connection secret.
		// ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.RuleGroup)
	if !ok {
		return errors.New(errNotRuleGroup)
	}

	err := c.service.DeleteRuleGroup(ctx, cr.Spec.ForProvider.Namespace, meta.GetExternalName(cr))

	return errors.Wrap(err, "")
}

func isUpToDate(cr *v1alpha1.RuleGroup, observedRuleGroup *rwrulefmt.RuleGroup) bool {
	if cr == nil || observedRuleGroup == nil {
		return false
	}

	var interval model.Duration
	var err error

	if cr.Spec.ForProvider.Interval != nil {
		interval, err = model.ParseDuration(*cr.Spec.ForProvider.Interval)
		if err != nil {
			return false
		}
	}

	if interval != observedRuleGroup.Interval {
		return false
	}

	var recordRule rulefmt.RuleNode
	var alertRule rulefmt.RuleNode

	// cortex rules
	for _, rule := range observedRuleGroup.Rules {
		if !rule.Record.IsZero() {
			recordRule = rule
		}
		if !rule.Alert.IsZero() {
			alertRule = rule
		}
	}

	// iterate through kubernetes rules and compare
	for _, rule := range cr.Spec.ForProvider.Rules {
		rn, err := generateRuleNode(rule)
		if err != nil {
			return false
		}

		if !rn.Alert.IsZero() {
			if !cmp.Equal(rn.Alert.Value, alertRule.Alert.Value) {
				return false
			}
		}

		if !rn.Record.IsZero() {
			if !cmp.Equal(rn.Record.Value, recordRule.Record.Value) {
				return false
			}
		}
	}

	return true
}

// generates a Cortex RuleNode from a Kubernetes RuleNode
func generateRuleNode(specRuleNode v1alpha1.RuleNode) (*rulefmt.RuleNode, error) {
	rn := rulefmt.RuleNode{}

	if specRuleNode.Record != nil {
		yn := yaml.Node{}
		err := yaml.Unmarshal([]byte(*specRuleNode.Record), &yn)
		if err != nil {
			return nil, err
		}
		// we are interested in the ScalarNode
		rn.Record = *yn.Content[0]
	}
	if specRuleNode.Alert != nil {
		yn := yaml.Node{}
		err := yaml.Unmarshal([]byte(*specRuleNode.Alert), &yn)
		if err != nil {
			return nil, err
		}
		rn.Alert = *yn.Content[0]
	}
	yn := yaml.Node{}
	err := yaml.Unmarshal([]byte(specRuleNode.Expr), &yn)
	if err != nil {
		return nil, err
	}
	rn.Expr = *yn.Content[0]
	if specRuleNode.For != nil {
		rn.For, err = model.ParseDuration(*specRuleNode.For)
		if err != nil {
			return nil, err
		}
	}
	if len(specRuleNode.Labels) != 0 {
		rn.Labels = specRuleNode.Labels
	}
	if len(specRuleNode.Annotations) != 0 {
		rn.Annotations = specRuleNode.Annotations
	}

	return &rn, nil
}

func isErrRuleGroupNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), errRuleGroupNotFound)
}
