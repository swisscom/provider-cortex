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

package alertmanager

import (
	"context"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/connection"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane/provider-cortex/apis/alerts/v1alpha1"
	apisv1alpha1 "github.com/crossplane/provider-cortex/apis/v1alpha1"
	xpClient "github.com/crossplane/provider-cortex/internal/clients"
	"github.com/crossplane/provider-cortex/internal/clients/alertmanager"
	"github.com/crossplane/provider-cortex/internal/features"
)

const (
	errNotConfiguration      = "managed resource is not a AlertManagerConfiguration custom resource"
	errConfigurationNotFound = "requested resource not found"
	errTrackPCUsage          = "cannot track ProviderConfig usage"
	errGetPC                 = "cannot get ProviderConfig"
	errGetCreds              = "cannot get credentials"

	errNewClient = "cannot create new Service"
)

// Setup adds a controller that reconciles RuleGroup managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.AlertManagerConfigurationGroupKind)

	cps := []managed.ConnectionPublisher{managed.NewAPISecretPublisher(mgr.GetClient(), mgr.GetScheme())}
	if o.Features.Enabled(features.EnableAlphaExternalSecretStores) {
		cps = append(cps, connection.NewDetailsManager(mgr.GetClient(), apisv1alpha1.StoreConfigGroupVersionKind))
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.AlertManagerConfigurationGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: newAlertManagerClient}),
		// managed.NewNameAsExternalName(c)
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithConnectionPublishers(cps...))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		// WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.AlertManagerConfiguration{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(config xpClient.Config) alertmanager.AlertManagerClient
}

func newAlertManagerClient(config xpClient.Config) alertmanager.AlertManagerClient {
	return xpClient.NewClient(config)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.AlertManagerConfiguration)
	if !ok {
		return nil, errors.New(errNotConfiguration)
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
	service alertmanager.AlertManagerClient
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.AlertManagerConfiguration)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotConfiguration)
	}

	// if meta.GetExternalName(cr) == "" {
	// 	return managed.ExternalObservation{
	// 		ResourceExists: false,
	// 	}, nil
	// }

	alertmanagerConfig, templateFiles, err := c.service.GetAlertmanagerConfig(ctx)
	if err != nil {
		switch {
		case isErrConfigurationNotFound(err):
			return managed.ExternalObservation{}, nil
		default:
			return managed.ExternalObservation{}, err
		}
	}

	if alertmanagerConfig == "" {
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
		ResourceUpToDate: isUpToDate(cr, alertmanagerConfig, templateFiles),

		// Return any details that may be required to connect to the external
		// resource. These will be stored as the connection secret.
		// ConnectionDetails: managed.ConnectionDetails{},
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.AlertManagerConfiguration)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotConfiguration)
	}

	err := c.service.CreateAlertmanagerConfig(ctx, cr.Spec.ForProvider.AlertmanagerConfig, cr.Spec.ForProvider.TemplateFiles)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.AlertManagerConfiguration)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotConfiguration)
	}

	err := c.service.CreateAlertmanagerConfig(ctx, cr.Spec.ForProvider.AlertmanagerConfig, cr.Spec.ForProvider.TemplateFiles)
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
	_, ok := mg.(*v1alpha1.AlertManagerConfiguration)
	if !ok {
		return errors.New(errNotConfiguration)
	}

	err := c.service.DeleteAlermanagerConfig(ctx)

	return errors.Wrap(err, "")
}

func isUpToDate(cr *v1alpha1.AlertManagerConfiguration, alertmanagerConfig string, templateFiles map[string]string) bool {
	if cr == nil || alertmanagerConfig == "" {
		return false
	}

	if cr.Spec.ForProvider.AlertmanagerConfig != alertmanagerConfig {
		return false
	}

	if len(templateFiles) != len(cr.Spec.ForProvider.TemplateFiles) {
		return false
	}

	for k := range templateFiles {
		if templateFiles[k] != cr.Spec.ForProvider.TemplateFiles[k] {
			return false
		}
	}

	return true
}

func isErrConfigurationNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), errConfigurationNotFound)
}
