/*
Gluon - BOSH / CF Orchestration via Kuberenetes API(s)

Copyright (c) 2020 James Hunt

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to
deal in the Software without restriction, including without limitation the
rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
sell copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software..

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
IN THE SOFTWARE.
*/

package controllers

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1alpha1 "github.com/starkandwayne/gluon-controller/api/v1alpha1"
)

// BOSHConfigReconciler reconciles a BOSHConfig object
type BOSHConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gluon.starkandwayne.com,resources=boshconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gluon.starkandwayne.com,resources=boshconfigs/status,verbs=get;update;patch

func (r *BOSHConfigReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("boshconfig", req.NamespacedName)

	// fetch the BOSHConfig instance
	instance := &v1alpha1.BOSHConfig{}
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
	if ok, info, err := instance.Dependencies.Resolved(r.Client, req.Namespace); !ok {
		if err != nil {
			log.Info("failed to determine if dependencies are resolved", "dependency", info, "error", err)
		} else {
			log.Info("dependencies not yet resolved", "dependency", info)
		}
		return instance.Dependencies.Requeue(), err
	}

	// create the ConfigMap for this BOSHConfig
	log.Info("checking for backing config map", "configmap", instance.Name)
	config := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name}, config)
	if err == nil {
		log.Info("updating backing config map", "configmap", instance.Name)
		config.Data["config.yml"] = instance.Spec.Config

		err = r.Client.Update(ctx, config)
		if err != nil {
			return ctrl.Result{}, err
		}

	} else if errors.IsNotFound(err) {
		log.Info("creating backing config map", "configmap", instance.Name)
		config = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: instance.Namespace,
				Name:      instance.Name,
			},
			Data: map[string]string{
				"config.yml": instance.Spec.Config,
			},
		}

		if err := controllerutil.SetControllerReference(instance, config, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		err = r.Client.Create(ctx, config)
		if err != nil {
			return ctrl.Result{}, err
		}

	} else {
		return ctrl.Result{}, err
	}

	// retrieve our upstream director
	director := &v1alpha1.BOSHDeployment{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: instance.Spec.Director}, director)
	if err != nil {
		if errors.IsNotFound(err) {
			// director was there once, but is gone now
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	job := &batchv1.Job{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: instance.JobName(director)}, job)
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
	}

	return ctrl.Result{}, nil
}

func (r *BOSHConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.BOSHConfig{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
