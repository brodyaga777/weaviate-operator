/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"github.com/brodyaga777/weaviate-operator/internal/pkg/k8sutils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	dbv1alpha1 "github.com/brodyaga777/weaviate-operator/api/v1alpha1"
)

// WeaviateReconciler reconciles a Weaviate object
type WeaviateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=db.weaviate.io,resources=weaviates,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=db.weaviate.io,resources=weaviates/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=db.weaviate.io,resources=weaviates/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Weaviate object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *WeaviateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// extract reconciled resource
	var db dbv1alpha1.Weaviate
	err := r.Get(ctx, req.NamespacedName, &db)
	if err != nil {
		if errors.IsNotFound(err) {
			// was deleted
			return reconcile.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	logger.Info("reconciling weaviate instance", "name", db.GetName())

	// TODO: handle deletion

	// process gRPC service
	logger.Info(
		"reconciling gRPC service",
		"instance", db.Name,
	)
	gSvc, err := r.reconcileGRPCService(ctx, db)
	if err != nil {
		logger.Error(err, "failed to reconcile gRPC service")
		return ctrl.Result{}, err
	}
	logger.Info(
		"reconciled gRPC service",
		"instance", db.Name,
		"service", gSvc,
	)

	// process service configuration
	// TODO: separate
	err = r.reconcileServices(ctx, db)
	if err != nil {
		logger.Error(err, "failed to reconcile weaviate services")
		return ctrl.Result{}, err
	}

	logger.Info("reconciling secret", "instance", db.Name)
	err = r.reconcileSecret(ctx, db)
	if err != nil {
		logger.Error(err, "failed to reconcile weaviate secret")
		return ctrl.Result{}, err
	}
	logger.Info("secret configuration reconciled", "instance", db.Name)

	// process statefulset configuration
	sts, err := r.reconcileStatefulSet(ctx, db)
	if err != nil {
		logger.Error(err, "failed to reconcile weaviate sts")
		return ctrl.Result{}, err
	}

	// mark weaviate as ready
	if sts.Status.ReadyReplicas == db.Spec.Replicas {
		db.Status.Ready = true
		err = r.Update(ctx, &db)
		if err != nil {
			logger.Error(err, "failed to updated weaviate status", "instance", db.Name)
		}
	}

	logger.Info("reconciled statefulset", "instance", sts.ObjectMeta.Name)

	logger.Info("reconciling completed for weaviate instance", "name", db.GetName())

	return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 15}, nil

}

func (r *WeaviateReconciler) reconcileSecret(ctx context.Context, db dbv1alpha1.Weaviate) error {
	logger := log.FromContext(ctx)

	secret := &corev1.Secret{}
	err := r.Client.Get(
		ctx,
		types.NamespacedName{
			Name:      fmt.Sprintf("%s-cluster-api-basic-auth", db.Name),
			Namespace: db.Namespace,
		},
		secret,
	)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		logger.Info("weaviate secret does not exist, provisioning", "instance", db.Name)

		secret, err = r.createSecret(ctx, db)
		if err != nil {
			logger.Error(err, "failed to provision secret")
			return err
		}
		logger.Info("secret provisioned", "instance", db.Name)
	}

	return nil
}

// createSecret TODO: handle user provisioned secrets
func (r *WeaviateReconciler) createSecret(ctx context.Context, db dbv1alpha1.Weaviate) (*corev1.Secret, error) {

	username := "usertest"
	password := "passwordTest"

	secretData := make(map[string][]byte)

	secretData["username"] = []byte(username)
	secretData["password"] = []byte(password)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-cluster-api-basic-auth", db.Name),
			Namespace: db.Namespace,
		},
		Data:       secretData,
		StringData: nil,
		Type:       v1.SecretTypeBasicAuth,
	}

	err := r.Client.Create(ctx, secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func (r *WeaviateReconciler) reconcileStatefulSet(ctx context.Context, db dbv1alpha1.Weaviate) (*appsv1.StatefulSet, error) {
	logger := log.FromContext(ctx)

	// get sts
	sts := &appsv1.StatefulSet{}
	err := r.Client.Get(
		ctx,
		types.NamespacedName{Name: db.Name, Namespace: db.Namespace},
		sts,
	)
	if errors.IsNotFound(err) {
		logger.Info("weaviate instance does not exist, provisioning", "name", db.GetName())

		// TODO: provision
		wv, err := r.createStatefulSet(ctx, db)
		if err != nil {
			logger.Error(err, "errprovision")
			return nil, err
		}
		println(wv)
	}

	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "failed to retrieve stateful set")
		return nil, err
	}

	// TODO: implement update func? first easily start from service

	err = r.Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: db.Name}, sts)
	if err != nil {
		logger.Error(err, "failed to retrieve weaviate statefulset", "instance", db.Name)
		return nil, err
	}

	return sts, nil
}

func (r *WeaviateReconciler) reconcileGRPCService(ctx context.Context, db dbv1alpha1.Weaviate) (*corev1.Service, error) {
	logger := log.FromContext(ctx)

	svc := &corev1.Service{}

	err := r.Client.Get(
		ctx,
		types.NamespacedName{Name: fmt.Sprintf("%s-grpc", db.Name), Namespace: db.Namespace},
		svc,
	)
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}

	// TODO: think, ok, how will you will handle update?
	if errors.IsNotFound(err) {
		logger.Info("grpc service does not exist, provisioning", "name", db.GetName())

		// reconcile grpc service
		// TODO: createOrUpdate
		svc, err = r.createGRPCService(ctx, db)
		if err != nil {
			return nil, err
		}
	}

	// TODO: ygbt like implement update

	return svc, err
}

func (r *WeaviateReconciler) reconcileServices(ctx context.Context, db dbv1alpha1.Weaviate) error {
	logger := log.FromContext(ctx)

	// get svc
	svc := &corev1.Service{}
	hSvc := &corev1.Service{}

	err := r.Client.Get(
		ctx,
		types.NamespacedName{Name: db.Name, Namespace: db.Namespace},
		svc,
	)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	// TODO: think, ok, how will you will handle update?
	if errors.IsNotFound(err) {
		logger.Info("service does not exist, provisioning", "name", db.GetName())

		// reconcile core service
		svc, err = r.createService(ctx, db)
		if err != nil {
			return err
		}

	}

	err = r.Client.Get(
		ctx,
		types.NamespacedName{Name: fmt.Sprintf("%s-headless", db.Name), Namespace: db.Namespace},
		hSvc,
	)

	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		logger.Info("headless service does not exist, provisioning", "name", db.GetName())

		// reconcile headless service
		hSvc, err = r.createHeadlessService(ctx, db)
		if err != nil {
			return err
		}
		println(hSvc)
	}

	return nil
}

func (r *WeaviateReconciler) createHeadlessService(ctx context.Context, db dbv1alpha1.Weaviate) (*corev1.Service, error) {
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-headless", db.Name),
			Namespace: db.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: db.APIVersion,
					Kind:       db.Kind,
					Name:       db.Name,
					UID:        db.UID,
				},
			},
			Labels: map[string]string{
				"app.kubernetes.io/name":       "weaviate",
				"app.kubernetes.io/managed-by": "weaviate-operator",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": db.Name,
			},
			Type: v1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Port:       80,
					TargetPort: intstr.FromInt32(7000),
				},
			},
			PublishNotReadyAddresses: true,
			ClusterIP:                "None",
		},
	}

	err := r.Client.Create(ctx, &svc)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (r *WeaviateReconciler) createGRPCService(ctx context.Context, db dbv1alpha1.Weaviate) (*corev1.Service, error) {
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-grpc", db.Name),
			Namespace: db.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: db.APIVersion,
					Kind:       db.Kind,
					Name:       db.Name,
					UID:        db.UID,
				},
			},
			Labels: map[string]string{
				"app.kubernetes.io/name":       "weaviate",
				"app.kubernetes.io/managed-by": "weaviate-operator",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": db.Name,
			},
			Type: v1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "grpc",
					Port:       50051,
					TargetPort: intstr.FromInt32(50051),
				},
			},
		},
	}

	err := r.Client.Create(ctx, &svc)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (r *WeaviateReconciler) createService(ctx context.Context, db dbv1alpha1.Weaviate) (*corev1.Service, error) {

	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      db.GetName(),
			Namespace: db.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: db.APIVersion,
					Kind:       db.Kind,
					Name:       db.Name,
					UID:        db.UID,
				},
			},
			Labels: map[string]string{
				"app.kubernetes.io/name":       "weaviate",
				"app.kubernetes.io/managed-by": "weaviate-operator",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": db.Name,
			},
			Type: v1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt32(8080),
				},
			},
		},
	}

	err := r.Client.Create(ctx, &svc)
	if err != nil {
		return nil, err
	}

	return nil, nil

}

func (r *WeaviateReconciler) createStatefulSet(ctx context.Context, db dbv1alpha1.Weaviate) (*appsv1.StatefulSet, error) {

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      db.Name,
			Namespace: db.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: db.APIVersion,
					Kind:       db.Kind,
					Name:       db.Name,
					UID:        db.UID,
				},
			},
			// TODO: set constants
			Labels: map[string]string{
				"app.kubernetes.io/component":  "weaviate",
				"app.kubernetes.io/instance":   db.Name,
				"app.kubernetes.io/name":       "weaviate",
				"app.kubernetes.io/version":    "v0.0.1",
				"app.kubernetes.io/part-of":    "weaviate",
				"app.kubernetes.io/managed-by": "weaviate-operator",
				"app":                          db.Name,
				"name":                         db.Name,
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &db.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": db.Name,
					//"app.kubernetes.io/part-of": "weaviate",
					//"app.kubernetes.io/name":    "weaviate",
				},
			},
			PodManagementPolicy: appsv1.ParallelPodManagement,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":                          db.Name,
						"app.kubernetes.io/name":       "weaviate",
						"app.kubernetes.io/managed-by": "weaviate-operator",
					},
				},

				Spec: v1.PodSpec{
					//Volumes: []v1.Volume{
					//	{
					//		Name:         "",
					//		VolumeSource: v1.VolumeSource{},
					//	},
					//},
					Containers: []corev1.Container{

						{
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      fmt.Sprintf("%s-data", db.Name),
									MountPath: "/var/lib/weaviate",
								},
							},

							Name: "weaviate",
							//Image: "cr.weaviate.io/semitechnologies/weaviate:1.25.0",
							Image: "cr.weaviate.io/semitechnologies/weaviate:1.26.0-preview0-42c1f4d",
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/v1/.well-known/ready",
										Port: intstr.IntOrString{IntVal: 8080},
										//Host:        "",
										//Scheme:      "",
										//HTTPHeaders: nil,
									},
								},
								FailureThreshold:    30,
								InitialDelaySeconds: 3,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      5,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/v1/.well-known/live",
										Port: intstr.IntOrString{IntVal: 8080},
										//Host:        "",
										//Scheme:      "",
										//HTTPHeaders: nil,
									},
								},
								FailureThreshold:    30,
								InitialDelaySeconds: 3,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      5,
							},
							ImagePullPolicy: corev1.PullAlways,
							Command:         []string{"/bin/weaviate"},
							Args: []string{
								"--host",
								"0.0.0.0",
								"--port",
								"8080",
								"--scheme",
								"http",
								//"--config-file",
								//"/weaviate-config/conf.yaml",
								"--read-timeout=60s",
								"--write-timeout=60s",
							},
							Ports: []v1.ContainerPort{
								{
									ContainerPort: 8080,
								},
								{
									Name:          "grpc",
									ContainerPort: 50051,
								},
							},
						},
					},
				},
			},
			VolumeClaimTemplates: []v1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s-data", db.Name)},
					Spec: v1.PersistentVolumeClaimSpec{
						AccessModes: []v1.PersistentVolumeAccessMode{
							v1.ReadWriteOnce,
						},
						//Selector:         nil,
						Resources: v1.ResourceRequirements{Requests: v1.ResourceList{
							v1.ResourceStorage: resource.MustParse("2Gi"),
						}},
						//VolumeName:       "",
						//StorageClassName: nil,
						//VolumeMode:       nil,
						//DataSource:       nil,
						//DataSourceRef:    nil,
					},
				},
			},
			ServiceName: fmt.Sprintf("%s-headless", db.Name),
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
		},
	}

	// TODO: parse and pass envs to weaviate container
	envs := k8sutils.ParseEnvs(db)
	sts.Spec.Template.Spec.Containers[0].Env = envs

	err := r.Client.Create(ctx, sts)
	if err != nil {
		return nil, err
	}

	return sts, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WeaviateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dbv1alpha1.Weaviate{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.Secret{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 3}).
		Complete(r)
}
