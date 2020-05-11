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

// BOSHConfigReconciler reconciles a BOSHConfig object
type BOSHConfigReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gluon.starkandwayne.com,resources=boshconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gluon.starkandwayne.com,resources=boshconfigs/status,verbs=get;update;patch

func (r *BOSHConfigReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("boshconfig", req.NamespacedName)

	// fetch the BOSHConfig instance
	instance := &v1alpha1.BOSHConfig{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// that's ok, maybe someone got cold feet and deleted it.
			return ctrl.Result{}, nil
		}
		// something else went wrong...
		return ctrl.Result{}, err
	}

	// create the ConfigMap for this BOSHConfig
	config := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), req.NamespacedName, config)
	if err != nil {
		if errors.IsNotFound(err) {
			config = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: req.Namespace,
					Name:      req.Name,
				},
				Data: map[string]string{
					"config.yml": instance.Spec.Config,
				},
			}

			if err := controllerutil.SetControllerReference(instance, config, r.Scheme); err != nil {
				return ctrl.Result{}, err
			}

			err = r.Client.Create(context.TODO(), config)
			if err != nil {
				return ctrl.Result{}, err
			}

			// configmap created.

		} else {
			return ctrl.Result{}, err
		}
	}

	command := []string{
		"bosh",
		"update-config",
		"-n",
		"--name",
		req.Name,
		"--type",
		instance.Spec.Type,
		"/bosh/config/config.yml",
	}

	directors := &v1alpha1.BOSHDeploymentList{}
	err = r.Client.List(context.TODO(), directors, client.InNamespace(req.Namespace), client.MatchingLabels(instance.Spec.ApplyTo))
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, director := range directors.Items {
		jobName := fmt.Sprintf("update-config-%s-on-%s", req.Name, director.Name)
		directorSecretName := fmt.Sprintf("%s-secrets", director.Name)

		job := &batchv1.Job{}
		err = r.Client.Get(context.TODO(), types.NamespacedName{Namespace: req.Namespace, Name: jobName}, job)
		if err == nil || !errors.IsNotFound(err) {
			// already got a job; don't need another one
			// (or something bad happened, which we cannot handle right now)
			continue
		}

		// create the Job resource, in all of its glory
		var one int32 = 1
		job = &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: req.Namespace,
				Name:      jobName,
			},
			Spec: batchv1.JobSpec{
				Parallelism:  &one,
				Completions:  &one,
				BackoffLimit: &one,
				//TTLSecondsAfterFinished
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Volumes: []corev1.Volume{
							corev1.Volume{
								Name: "config",
								VolumeSource: corev1.VolumeSource{
									ConfigMap: &corev1.ConfigMapVolumeSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: req.Name,
										},
									},
								},
							},
						},
						Containers: []corev1.Container{
							corev1.Container{
								Name:            "update-config",
								Image:           "starkandwayne/bosh-create-env:latest",
								ImagePullPolicy: corev1.PullAlways,
								Command:         command,
								VolumeMounts: []corev1.VolumeMount{
									corev1.VolumeMount{
										Name:      "config",
										MountPath: "/bosh/config",
										ReadOnly:  true,
									},
								},
								Env: []corev1.EnvVar{
									corev1.EnvVar{
										Name: "BOSH_ENVIRONMENT",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: directorSecretName,
												},
												Key: "endpoint",
											},
										},
									},
									corev1.EnvVar{
										Name: "BOSH_CLIENT",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: directorSecretName,
												},
												Key: "username",
											},
										},
									},
									corev1.EnvVar{
										Name: "BOSH_CLIENT_SECRET",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: directorSecretName,
												},
												Key: "password",
											},
										},
									},
									corev1.EnvVar{
										Name: "BOSH_CA_CERT",
										ValueFrom: &corev1.EnvVarSource{
											SecretKeyRef: &corev1.SecretKeySelector{
												LocalObjectReference: corev1.LocalObjectReference{
													Name: directorSecretName,
												},
												Key: "ca",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}
		if err := controllerutil.SetControllerReference(instance, job, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		err = r.Client.Create(context.TODO(), job)
		if err != nil {
			return ctrl.Result{}, err
		}

		// job created.
	}

	return ctrl.Result{}, nil
}

func (r *BOSHConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.BOSHConfig{}).
		Complete(r)
}
