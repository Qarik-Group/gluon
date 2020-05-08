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
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	boshv1alpha1 "github.com/starkandwayne/gluon-controller/api/v1alpha1"
)

// BOSHDeploymentReconciler reconciles a BOSHDeployment object
type BOSHDeploymentReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=gluon.starkandwayne.com,resources=boshdeployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gluon.starkandwayne.com,resources=boshdeployments/status,verbs=get;update;patch

func (r *BOSHDeploymentReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("boshdeployment", req.NamespacedName)

	// fetch the BOSHDeployment instance
	instance := &boshv1alpha1.BOSHDeployment{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// that's ok, maybe someone got cold feet and deleted it.
			return ctrl.Result{}, nil
		}
		// something else went wrong...
		return ctrl.Result{}, err
	}

	jobName := fmt.Sprintf("deploy-%s-bosh", req.Name)
	if instance.Spec.Director != "" {
		jobName = fmt.Sprintf("deploy-%s-to-%s", req.Name, instance.Spec.Director)
	}
	stateVolumeName := fmt.Sprintf("%s-state", req.Name)
	stateConfigMapName := fmt.Sprintf("%s-state", req.Name)
	secretName := fmt.Sprintf("%s-secrets", req.Name)

	// first we make a volume for our state files / creds / vars
	stateVolume := &corev1.PersistentVolumeClaim{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Namespace: req.Namespace, Name: stateVolumeName}, stateVolume)
	if err != nil && errors.IsNotFound(err) {
		r.Log.Info("creating persistent volume claim", "namespace", req.Namespace, "pvc", stateVolumeName)
		mode := corev1.PersistentVolumeFilesystem
		stateVolume = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: req.Namespace,
				Name:      stateVolumeName,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				VolumeMode: &mode,
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						"storage": resource.MustParse("1Mi"),
					},
				},
			},
		}

		if err := controllerutil.SetControllerReference(instance, stateVolume, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}

		err = r.Client.Create(context.TODO(), stateVolume)
		if err != nil {
			return ctrl.Result{}, err
		}

		// pvc created.
		// (but we still have more work to do!)

	} else if err != nil {
		return ctrl.Result{}, err
	}

	found := &batchv1.Job{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Namespace: req.Namespace, Name: jobName}, found)
	if err != nil && errors.IsNotFound(err) {
		// deployment job not found; create it.
		r.Log.Info("creating deployment job", "namespace", req.Namespace, "job", jobName)

		vars := []corev1.EnvVar{
			corev1.EnvVar{
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						APIVersion: "v1",
						FieldPath:  "metadata.namespace",
					},
				},
			},
			corev1.EnvVar{
				Name:  "UPSTREAM_REPO",
				Value: instance.Spec.Repo,
			},
			corev1.EnvVar{
				Name:  "UPSTREAM_REF",
				Value: instance.Spec.Ref,
			},
			corev1.EnvVar{
				Name:  "UPSTREAM_ENTRYPOINT",
				Value: instance.Spec.Entrypoint,
			},
			corev1.EnvVar{
				Name:  "CREDS_SECRET_NAME",
				Value: secretName,
			},
			corev1.EnvVar{
				Name:  "CREDS_STATE_FILE_CONFIG_MAP",
				Value: stateConfigMapName,
			},
			corev1.EnvVar{
				Name:  "GLUON_director_name", // FIXME
				Value: req.Name,
			},
		}

		if instance.Spec.Director != "" {
			directorSecretName := fmt.Sprintf("%s-secrets", instance.Spec.Director)
			vars = append(vars, corev1.EnvVar{
				Name: "BOSH_ENVIRONMENT",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: directorSecretName,
						},
						Key: "endpoint",
					},
				},
			})
			vars = append(vars, corev1.EnvVar{
				Name: "BOSH_CLIENT",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: directorSecretName,
						},
						Key: "username",
					},
				},
			})
			vars = append(vars, corev1.EnvVar{
				Name: "BOSH_CLIENT_SECRET",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: directorSecretName,
						},
						Key: "password",
					},
				},
			})
			vars = append(vars, corev1.EnvVar{
				Name: "BOSH_CA_CERT",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: directorSecretName,
						},
						Key: "ca",
					},
				},
			})

			// track original BOSHDeployment spec.Name as the deployment
			vars = append(vars, corev1.EnvVar{
				Name:  "BOSH_DEPLOYMENT",
				Value: req.Name,
			})
		}

		// set up variable source references
		type secRef struct {
			secret string
			key    string
		}
		secRefs := make(map[string]secRef)
		cmRefs := make(map[string]string)
		for _, src := range instance.Spec.Vars {
			// name/value literal
			if src.Name != "" {
				vars = append(vars, corev1.EnvVar{
					Name:  fmt.Sprintf("GLUON_%s", src.Name),
					Value: src.Value,
				})
				continue
			}

			// configMap ref
			if src.ConfigMap.Name != "" {
				cm := &corev1.ConfigMap{}
				err = r.Client.Get(context.TODO(), types.NamespacedName{Namespace: req.Namespace, Name: src.ConfigMap.Name}, cm)
				if err != nil {
					return ctrl.Result{}, err
				}

				for k := range cm.Data {
					cmRefs[k] = src.ConfigMap.Name
				}
				continue
			}

			// secret ref
			if src.Secret.Name != "" {
				secret := &corev1.Secret{}
				err = r.Client.Get(context.TODO(), types.NamespacedName{Namespace: req.Namespace, Name: src.Secret.Name}, secret)
				if err != nil {
					return ctrl.Result{}, err
				}

				if src.Secret.MapKeys != nil {
					// map just the keys to their variables
					for k, variable := range src.Secret.MapKeys {
						secRefs[variable] = secRef{
							secret: src.Secret.Name,
							key:    k,
						}
					}
				} else {
					// take everything as a 1:1 key->var relation
					for k := range secret.Data {
						secRefs[k] = secRef{
							secret: src.Secret.Name,
							key:    k,
						}
					}
				}
				continue
			}
		}

		// resolve final ConfigMap var references
		for k, src := range cmRefs {
			vars = append(vars, corev1.EnvVar{
				Name: fmt.Sprintf("GLUON_%s", k),
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: src,
						},
						Key: k,
					},
				},
			})
		}

		// resolve final Secret var references
		for k, src := range secRefs {
			vars = append(vars, corev1.EnvVar{
				Name: fmt.Sprintf("GLUON_%s", k),
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: src.secret,
						},
						Key: src.key,
					},
				},
			})
		}

		// append ops files to command
		command := make([]string, len(instance.Spec.Ops)*2+1)
		command[0] = "deploy"
		for i, op := range instance.Spec.Ops {
			file := op
			if !strings.HasSuffix(file, ".yml") && !strings.HasSuffix(file, ".yaml") {
				file = fmt.Sprintf("%s.yml", op)
			}

			command[1+i*2] = "-o"
			command[1+i*2+1] = file
		}

		// create the Job resource, in all of its glory
		var one int32 = 1
		job := &batchv1.Job{
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
								Name: "state",
								VolumeSource: corev1.VolumeSource{
									PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
										ClaimName: stateVolumeName,
									},
								},
							},
						},
						Containers: []corev1.Container{
							corev1.Container{
								Name:            "deploy",
								Image:           "starkandwayne/bosh-create-env:latest",
								ImagePullPolicy: corev1.PullAlways,
								VolumeMounts: []corev1.VolumeMount{
									corev1.VolumeMount{
										Name:      "state",
										MountPath: "/bosh/state",
									},
								},
								Command: command,
								Env:     vars,
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
		return ctrl.Result{}, nil

	} else if err != nil {
		return ctrl.Result{}, err
	}

	// job exists; no need to requeue the reconciliation
	return ctrl.Result{}, nil
}

func (r *BOSHDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&boshv1alpha1.BOSHDeployment{}).
		Complete(r)
}
