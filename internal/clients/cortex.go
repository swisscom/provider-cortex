package clients

import (
	"context"
	"encoding/json"
	"fmt"

	cortexClient "github.com/cortexproject/cortex-tools/pkg/client"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/swisscom/provider-cortex/apis/v1alpha1"
)

// Error strings.
const (
	errTrackProviderConfigUsage  = "cannot track ProviderConfig usage"
	errGetProviderConfig         = "cannot get referenced ProviderConfig"
	errUnmarshalCredentialSecret = "cannot unmarshal the data in credentials secret"
	errGetCredentials            = "cannot get credentials"
)

// Credentials Secret content
const (
	CredentialsKeyUsername = "username"
	CredentialsKeyPassword = "password"
)

type Config struct {
	cortexClient.Config
}

// NewClient creates new Cortex Client with provided Cortex Configurations.
func NewClient(config Config) *cortexClient.CortexClient {
	c, err := cortexClient.New(cortexClient.Config{
		Address: config.Address,
		ID:      config.ID,
	})

	if err != nil {
		fmt.Printf("Could not initialize cortex client: %v", err)
	}
	return c
}

// GetConfig constructs a Config that can be used to authenticate to Cortex
func GetConfig(ctx context.Context, c client.Client, mg resource.Managed) (*Config, error) {
	switch {
	case mg.GetProviderConfigReference() != nil:
		return UseProviderConfig(ctx, c, mg)
	default:
		return nil, errors.New("providerConfigRef is not given")
	}
}

// UseProviderConfig to produce a config that can be used to authenticate to Cortex.
func UseProviderConfig(ctx context.Context, c client.Client, mg resource.Managed) (*Config, error) {
	pc := &v1alpha1.ProviderConfig{}
	if err := c.Get(ctx, types.NamespacedName{Name: mg.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetProviderConfig)
	}

	t := resource.NewProviderConfigUsageTracker(c, &v1alpha1.ProviderConfigUsage{})
	if err := t.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackProviderConfigUsage)
	}

	data, err := resource.CommonCredentialExtractor(ctx, pc.Spec.Credentials.Source, c, pc.Spec.Credentials.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCredentials)
	}

	m := map[string]string{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, errors.Wrap(err, errUnmarshalCredentialSecret)
	}

	return &Config{cortexClient.Config{Address: pc.Spec.Address, User: m[CredentialsKeyUsername], Key: m[CredentialsKeyPassword]}}, nil
	// return &Config{cortexClient.Config{ID: pc.Spec.TenantID, Address: pc.Spec.Address, User: m[CredentialsKeyUsername], Key: m[CredentialsKeyPassword]}}, nil
}
