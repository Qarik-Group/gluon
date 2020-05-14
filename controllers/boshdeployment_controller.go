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
	"fmt"

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

const Finalizer = "boshdeployment.gluon.starkandwayne.com"

// BOSHDeploymentReconciler reconciles a BOSHDeployment object
type BOSHDeploymentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gluon.starkandwayne.com,resources=boshdeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gluon.starkandwayne.com,resources=boshdeployments/status,verbs=get;update;patch

func (r *BOSHDeploymentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("boshdeployment", req.NamespacedName)

	// fetch the BOSHDeployment instance
	instance := &v1alpha1.BOSHDeployment{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// that's ok, maybe someone got cold feet and deleted it.
			return ctrl.Result{}, nil
		}
		// something else went wrong...
		return ctrl.Result{}, err
	}

	/*
		// register finalizer if desired
		if instance.ObjectMeta.DeletionTimestamp.IsZero() {
			// we are not being deleted, so if we do not have our finalizer,
			// then add it and update.
			if !HasFinalizer(instance, Finalizer) {
				instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, Finalizer)
				if err := r.Update(context.Background(), instance); err != nil {
					return ctrl.Result{}, err
				}
			}
		} else {
			// we are being deleted; is our finalizer still listed?
			if HasFinalizer(instance, Finalizer) {
				// create the Job resource, in all of its glory
				found := &batchv1.Job{}
				err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: instance.JobName("teardown")}, found)
				if err != nil && errors.IsNotFound(err) {
					job, err := instance.TeardownJob()
					if err != nil {
						return ctrl.Result{}, err
					}

					// rather than set an ownership record, just set a TTL
					var week int32 = 86400 * 7
					job.Spec.TTLSecondsAfterFinished = &week

					err = r.Client.Create(ctx, job)
					if err != nil {
						return ctrl.Result{}, err
					}

					// move the pvc ownership over to the teardown job
					pvc := &corev1.PersistentVolumeClaim{}
					err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: instance.StateVolumeName()}, pvc)
					if err == nil {
						if err := controllerutil.SetControllerReference(job, pvc, r.Scheme); err != nil {
							return ctrl.Result{}, err
						}
					}

				} else if err != nil {
					return ctrl.Result{}, err
				}
			}

			// remove our finalizer from the list and update it.
			instance.ObjectMeta.Finalizers = controllerutil.RemoveFinalizer(instance, Finalizer)
			if err := r.Update(context.Background(), instance); err != nil {
				return ctrl.Result{}, err
			}

			return ctrl.Result{}, nil
		}
	*/

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

	// first we make a volume for our state files / creds / vars
	log.Info("checking for persistent state volume", "pvc", instance.StateVolumeName())
	stateVolume := &corev1.PersistentVolumeClaim{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: instance.StateVolumeName()}, stateVolume)
	if err != nil && errors.IsNotFound(err) {
		log.Info("creating persistent volume claim", "pvc", instance.StateVolumeName())
		stateVolume = instance.StateVolume()
		if err := controllerutil.SetControllerReference(instance, stateVolume, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err = r.Client.Create(ctx, stateVolume); err != nil {
			return ctrl.Result{}, err
		}

		// pvc created.
		// (but we still have more work to do!)

	} else if err != nil {
		return ctrl.Result{}, err
	}

	// then we look for the deployment job
	log.Info("checking for deployment job", "job", instance.JobName("deploy"))
	job := &batchv1.Job{}
	err = r.Client.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: instance.JobName("deploy")}, job)
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

		// deployment job not found; create it.
		log.Info("creating deployment job", "job", instance.JobName("deploy"))
		job = instance.DeployJob()
		if err := r.ResolveVariableSources(instance, job); err != nil {
			return ctrl.Result{}, err
		}
		if err := controllerutil.SetControllerReference(instance, job, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Client.Create(ctx, job); err != nil {
			return ctrl.Result{}, err
		}

		// job created.
		return ctrl.Result{}, nil
	}

	// job exists; no need to requeue the reconciliation
	return ctrl.Result{}, nil
}

func (r *BOSHDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.BOSHDeployment{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

func HasFinalizer(o metav1.Object, finalizer string) bool {
	f := o.GetFinalizers()
	for _, e := range f {
		if e == finalizer {
			return true
		}
	}
	return false
}

func (r *BOSHDeploymentReconciler) ResolveVariableSources(bd *v1alpha1.BOSHDeployment, job *batchv1.Job) error {
	ctx := context.Background()

	type ref struct {
		secret    string
		configmap string
		key       string
		literal   string
	}

	// set up variable source references
	refs := make(map[string]ref)
	for _, src := range bd.Spec.Vars {
		// name/value literal
		if src.Name != "" {
			var rf ref
			rf.literal = src.Value
			refs[src.Name] = rf
			continue
		}

		// configMap ref
		if src.ConfigMap != nil {
			cm := &corev1.ConfigMap{}
			err := r.Client.Get(ctx, types.NamespacedName{Namespace: bd.Namespace, Name: src.ConfigMap.Name}, cm)
			if err != nil {
				return err
			}

			if src.ConfigMap.MapKeys != nil {
				// map just the keys to their variables
				for k, variable := range src.ConfigMap.MapKeys {
					var rf ref
					rf.configmap = src.ConfigMap.Name
					rf.key = k
					refs[variable] = rf
				}
			} else {
				// take everything as a 1:1 key->var relation
				for k := range cm.Data {
					var rf ref
					rf.configmap = src.ConfigMap.Name
					rf.key = k
					refs[k] = rf
				}
			}
			continue
		}

		// secret ref
		if src.Secret != nil {
			secret := &corev1.Secret{}
			err := r.Client.Get(ctx, types.NamespacedName{Namespace: bd.Namespace, Name: src.Secret.Name}, secret)
			if err != nil {
				return err
			}

			if src.Secret.MapKeys != nil {
				// map just the keys to their variables
				for k, variable := range src.Secret.MapKeys {
					var rf ref
					rf.secret = src.Secret.Name
					rf.key = k
					refs[variable] = rf
				}
			} else {
				// take everything as a 1:1 key->var relation
				for k := range secret.Data {
					var rf ref
					rf.secret = src.Secret.Name
					rf.key = k
					refs[k] = rf
				}
			}
			continue
		}
	}

	// resolve final var references
	vars := job.Spec.Template.Spec.Containers[0].Env

	for k, rf := range refs {
		var ev corev1.EnvVar
		ev.Name = fmt.Sprintf("GLUON_%s", k)

		if rf.literal != "" {
			ev.Value = rf.literal

		} else if rf.configmap != "" {
			ev.ValueFrom = &corev1.EnvVarSource{
				ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: rf.configmap,
					},
					Key: rf.key,
				},
			}

		} else if rf.secret != "" {
			ev.ValueFrom = &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: rf.secret,
					},
					Key: rf.key,
				},
			}
		}

		vars = append(vars, ev)
	}

	job.Spec.Template.Spec.Containers[0].Env = vars
	return nil
}
