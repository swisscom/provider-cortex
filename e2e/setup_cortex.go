package e2e

import (
	"context"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/crossplane-contrib/xp-testing/pkg/xpenvfuncs"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

const (
	cortexNamespace       = "default"
	bucketCreatorPodLabel = "app=minio-bucket-creator"
)

func installCortex(cortexVersion string) env.Func {
	return xpenvfuncs.Compose(
		xpenvfuncs.IgnoreMatchedErr(envfuncs.CreateNamespace(cortexNamespace), errors.IsAlreadyExists),
		installCortexManifests(cortexVersion),
		waitForCortexToBeAvailable(cortexNamespace, bucketCreatorPodLabel),
	)
}

func waitForCortexToBeAvailable(namespace, cortexPodLabel string) env.Func {
	return func(ctx context.Context, config *envconf.Config) (context.Context, error) {
		res := config.Client().Resources()
		res = res.WithNamespace(namespace)

		c := conditions.New(res)
		var pods v1.PodList

		err := res.List(ctx, &pods, resources.WithLabelSelector(bucketCreatorPodLabel))
		if err != nil {
			return nil, err
		}
		klog.V(4).Info("Waiting for Cortex to become available")
		for i := range pods.Items {
			err := wait.For(
				c.PodPhaseMatch(&pods.Items[i], v1.PodSucceeded),
				wait.WithTimeout(8*time.Minute), wait.WithImmediate())
			if err != nil {
				return nil, err
			}

		}
		klog.V(4).Info("Cortex has become available")
		return ctx, nil
	}
}

func installCortexManifests(cortexVersion string) env.Func {
	return func(ctx context.Context, config *envconf.Config) (context.Context, error) {
		manifest, err := downloadManifest()
		if err != nil {
			return ctx, err
		}
		r, err := resources.New(config.Client().RESTConfig())
		if err != nil {
			return ctx, err
		}
		err = decoder.DecodeEach(
			ctx,
			strings.NewReader(manifest),
			decoder.IgnoreErrorHandler(decoder.CreateHandler(r), errors.IsAlreadyExists),
			decoder.MutateNamespace(cortexNamespace),
		)

		if err != nil {
			return ctx, err
		}
		return ctx, nil
	}
}

func downloadManifest() (string, error) {
	url := "https://raw.githubusercontent.com/janwillies/cortex-cue/main/generated.yaml"
	req, err := http.NewRequestWithContext(context.TODO(), http.MethodGet, url, nil)

	if err != nil {
		return "", err
	}

	client := http.DefaultClient
	res, err := client.Do(req)

	if err != nil {
		return "", err
	}
	defer func() {
		err := res.Body.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()
	d, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", err
	}
	manifests := string(d)
	return manifests, nil
}
