package clients

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	cortexClient "github.com/cortexproject/cortex-tools/pkg/client"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/crossplane/provider-cortex/apis/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewClient creates new Cortex Client with provided Cortex Configurations.
func NewClient(config cortexClient.Config) *cortexClient.CortexClient {
	client, err := cortexClient.New(config)

	if err != nil {
		fmt.Printf("Could not initialize cortex client: %v", err)
	}
	return client
}

// GetConfig constructs a Config that can be used to authenticate to Cortex
func GetConfig(ctx context.Context, c client.Client, mg resource.Managed) (*cortexClient.Config, error) {
	switch {
	case mg.GetProviderConfigReference() != nil:
		return UseProviderConfig(ctx, c, mg)
	default:
		return nil, errors.New("providerConfigRef is not given")
	}
}

// UseProviderConfig to produce a config that can be used to authenticate to Cortex.
func UseProviderConfig(ctx context.Context, c client.Client, mg resource.Managed) (*cortexClient.Config, error) {
	pc := &v1alpha1.ProviderConfig{}
	if err := c.Get(ctx, types.NamespacedName{Name: mg.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, "cannot get referenced Provider")
	}

	t := resource.NewProviderConfigUsageTracker(c, &v1alpha1.ProviderConfigUsage{})
	if err := t.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, "cannot track ProviderConfig usage")
	}

	return &cortexClient.Config{ID: pc.Spec.TenantID, Address: pc.Spec.Address}, nil

	// switch s := pc.Spec.Credentials.Source; s { //nolint:exhaustive
	// case xpv1.CredentialsSourceSecret:
	// 	csr := pc.Spec.Credentials.SecretRef
	// 	if csr == nil {
	// 		return nil, errors.New("no credentials secret referenced")
	// 	}
	// 	s := &corev1.Secret{}
	// 	if err := c.Get(ctx, types.NamespacedName{Namespace: csr.Namespace, Name: csr.Name}, s); err != nil {
	// 		return nil, errors.Wrap(err, "cannot get credentials secret")
	// 	}
	// 	return &cortexClient.Config{ID: pc.Spec.TenantID, Address: pc.Spec.Address, Password: string(s.Data[csr.Key])}, nil
	// default:
	// 	return nil, errors.Errorf("credentials source %s is not currently supported", s)
	// }
}
