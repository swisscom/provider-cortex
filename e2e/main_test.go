//go:build e2e

package e2e

import (
	"os"
	"testing"

	runtime "k8s.io/apimachinery/pkg/runtime"

	xpv1alpha1 "github.com/crossplane/crossplane/apis/pkg/v1alpha1"
	"github.com/swisscom/provider-cortex/apis"
	"sigs.k8s.io/e2e-framework/pkg/env"

	"github.com/crossplane-contrib/xp-testing/pkg/images"
	"github.com/crossplane-contrib/xp-testing/pkg/logging"
	"github.com/crossplane-contrib/xp-testing/pkg/setup"
)

var testenv env.Environment

func TestMain(m *testing.M) {
	var verbosity = 4
	logging.EnableVerboseLogging(&verbosity)
	testenv = env.NewParallel()

	key := "crossplane/provider-cortex"
	imgs := images.GetImagesFromEnvironmentOrPanic(key, &key)
	clusterSetup := setup.ClusterSetup{
		Name:   "cortex",
		Images: imgs,
		ControllerConfig: &xpv1alpha1.ControllerConfig{
			Spec: xpv1alpha1.ControllerConfigSpec{
				Image: &imgs.Package,
				// Raise sync interval to speed up tests
				// add debug output, in case necessary for debugging in e.g. CI
				Args: []string{"--debug", "--sync=5s"},
			},
		},
		SecretData:        nil,
		AddToSchemaFuncs:  []func(s *runtime.Scheme) error{apis.AddToScheme},
		CrossplaneVersion: "1.13.2",
	}

	clusterSetup.Configure(testenv)
	testenv.Setup(installCortex("v1.15.2"))
	os.Exit(testenv.Run(m))
}
