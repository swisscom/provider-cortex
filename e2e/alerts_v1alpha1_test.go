//go:build e2e

package e2e

import (
	"fmt"
	"testing"

	"github.com/crossplane-contrib/xp-testing/pkg/resources"
	"sigs.k8s.io/e2e-framework/pkg/features"

	"github.com/swisscom/provider-cortex/apis/alerts/v1alpha1"
)

func Test_AlertManagerConfiguration_v1alpha1(t *testing.T) {

	resource := resources.NewResourceTestConfig(&v1alpha1.AlertManagerConfiguration{}, "AlertManagerConfiguration")

	fB := features.New(fmt.Sprintf("%v", resource.Kind))
	fB.WithLabel("kind", resource.Kind)
	fB.Setup(resource.Setup)
	fB.Assess("create", resource.AssessCreate)
	fB.Assess("delete", resource.AssessDelete)

	testenv.Test(t, fB.Feature())

}
