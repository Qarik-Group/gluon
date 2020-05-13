package v1alpha1

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type DependencySpec struct {
	Stemcell   *string `json:"stemcell,omitempty"`
	Deployment *string `json:"deployment,omitempty"`
	Config     *string `json:"config,omitempty"`
	Status     string  `json:"status"`
}

type DependencySpecs struct {
	RetryAfter   int              `json:"retryAfter,omitempty"`
	Dependencies []DependencySpec `json:"dependsOn,omitempty"`
}

func (ds DependencySpec) Resolved(c client.Client, ns string) (bool, string, error) {
	var (
		ready bool
		state string
	)

	what := "(unrecognized type)"
	if ds.Stemcell != nil {
		what = fmt.Sprintf("stemcell %s", *ds.Stemcell)
		sc := &BOSHStemcell{}
		err := c.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: *ds.Stemcell}, sc)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, what, nil
			}
			return false, what, err
		}

		ready = sc.Status.Ready
		state = sc.Status.State

	} else if ds.Deployment != nil {
		what = fmt.Sprintf("deployment %s", *ds.Deployment)
		dep := &BOSHDeployment{}
		err := c.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: *ds.Deployment}, dep)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, what, nil
			}
			return false, what, err
		}

		ready = dep.Status.Ready
		state = dep.Status.State

	} else if ds.Config != nil {
		what = fmt.Sprintf("config %s", *ds.Config)
		cfg := &BOSHConfig{}
		err := c.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: *ds.Config}, cfg)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, what, nil
			}
			return false, what, err
		}

		ready = cfg.Status.Ready
		state = cfg.Status.State

	} else {
		return false, what, fmt.Errorf("unrecognized object type") // FIXME validating webhook please
	}

	return ready && (ds.Status == "" || state == ds.Status), what, nil
}

func (dss DependencySpecs) Resolved(c client.Client, ns string) (bool, string, error) {
	for _, spec := range dss.Dependencies {
		if ok, desc, err := spec.Resolved(c, ns); !ok || err != nil {
			return ok, desc, err
		}
	}
	return true, "", nil
}

func (dss DependencySpecs) Requeue() ctrl.Result {
	return ctrl.Result{RequeueAfter: time.Duration(dss.RetryAfter) * time.Second}
}
