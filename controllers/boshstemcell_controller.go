/*


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

package controllers

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/starkandwayne/gluon-controller/api/v1alpha1"
)

// BOSHStemcellReconciler reconciles a BOSHStemcell object
type BOSHStemcellReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gluon.starkandwayne.com,resources=boshstemcells,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gluon.starkandwayne.com,resources=boshstemcells/status,verbs=get;update;patch

func (r *BOSHStemcellReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("boshstemcell", req.NamespacedName)

	// fetch the BOSHStemcell instance
	instance := &v1alpha1.BOSHStemcell{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// that's ok, maybe someone got cold feet and deleted it.
			return ctrl.Result{}, nil
		}
		// something else went wrong...
		return ctrl.Result{}, err
	}

	// check to see if our dependencies are resolved
	log.Info("checking dependencies")
	if ok, info, err := instance.Dependencies.Resolved(r.Client, instance.Namespace); !ok {
		if err != nil {
			log.Info("failed to determine if dependencies are resolved", "dependency", info, "error", err)
		} else {
			log.Info("dependencies not yet resolved", "dependency", info)
		}
		return instance.Dependencies.Requeue(), err
	}

	director := &v1alpha1.BOSHDeployment{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: instance.Spec.Director}, director)
	if err != nil {
		if errors.IsNotFound(err) {
			// director was there once, but is gone now
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	job := &batchv1.Job{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: instance.JobName(director)}, job)
	if err == nil {
		// job exists; we may have gotten a reconcile request based on our watch(es)
		instance.Status.Ready, instance.Status.State = v1alpha1.DetermineReadiness(job)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}

	} else if !errors.IsNotFound(err) {
		return ctrl.Result{}, err

	} else {
		instance.Status.Ready, instance.Status.State = v1alpha1.DetermineReadiness(nil)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}

		// create the Job resource, in all of its glory
		job := instance.Job(director)
		if err := controllerutil.SetControllerReference(instance, job, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err = r.Client.Create(ctx, job); err != nil {
			return ctrl.Result{}, err
		}

		// job created.
	}

	return ctrl.Result{}, nil
}

func (r *BOSHStemcellReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.BOSHStemcell{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
