package service

import (
	"context"
	"crypto/sha1"
	"fmt"
	"reflect"
	"sort"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var (
	log = logf.Log.WithName("controller_service")
)

const (
	svcNameLabel       = "simplelb.amurant.io/svcname"
	svcHashAnnotation  = "simplelb.amurant.io/svchash"
	daemonsetNodeLabel = "simplelb.amurant.io/enablelb"
)

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func AddToManager(mgr manager.Manager, portforwardImage *string) error {
	// Create a new controller
	c, err := controller.New("service-controller", mgr, controller.Options{Reconciler: newServiceReconciler(mgr, portforwardImage)})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Service
	err = c.Watch(source.Kind(
		mgr.GetCache(),
		&corev1.Service{},
		&handler.TypedEnqueueRequestForObject[*corev1.Service]{},
	))
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner Service
	err = c.Watch(source.Kind(
		mgr.GetCache(),
		&appsv1.DaemonSet{},
		handler.TypedEnqueueRequestForOwner[*appsv1.DaemonSet](
			mgr.GetScheme(),
			mgr.GetRESTMapper(),
			&corev1.Service{},
			handler.OnlyControllerOwner(),
		),
	))
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileService implements reconcile.Reconciler
var _ reconcile.Reconciler = &ServiceReconciler{}

// ReconcileService reconciles a Service object
type ServiceReconciler struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme

	portforwardImage *string
}

// newServiceReconciler returns a new reconcile.Reconciler
func newServiceReconciler(mgr manager.Manager, portforwardImage *string) reconcile.Reconciler {
	return &ServiceReconciler{
		client:           mgr.GetClient(),
		scheme:           mgr.GetScheme(),
		portforwardImage: portforwardImage,
	}
}

// Reconcile reads that state of the cluster for a Service object and makes changes based on the state read
// and what is in the Service.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ServiceReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Service")

	// Fetch the Service instance
	svc := &corev1.Service{}
	err := r.client.Get(ctx, request.NamespacedName, svc)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer ||
		svc.Spec.ClusterIP == "" ||
		svc.Spec.ClusterIP == "None" {
		// WE're only interested in LoadBalancer type services
		// Return and don't requeue
		reqLogger.Info("Not a LoadBalancer type of service, ignoring")
		return reconcile.Result{}, nil
	}

	// Generate the needed DS for the service
	ds := newDaemonSetForService(svc, *r.portforwardImage)

	// Set Service instance as the owner and controller
	if err := controllerutil.SetControllerReference(svc, ds, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this DaemonSet already exists
	found := &appsv1.DaemonSet{}
	err = r.client.Get(ctx, types.NamespacedName{Name: ds.Name, Namespace: ds.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new DS", "DS.Namespace", ds.Namespace, "DS.Name", ds.Name)

		if err := r.client.Create(ctx, ds); err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	if found.Annotations[svcHashAnnotation] != serviceHash(svc) {
		// Need to update the DS
		reqLogger.Info("Updating DS", "DS.Namespace", ds.Namespace, "DS.Name", ds.Name)

		if err := r.client.Update(ctx, ds); err != nil {
			return reconcile.Result{}, err
		}

		// DaemonSet updated successfully - don't requeue
		// Changes in the DS will trigger proper requests for this as the DS rollout progresses
		return reconcile.Result{}, nil
	}

	// DS already exists - don't requeue but sync up the addresses for the service
	// We also get the reconcile request on changes to the created DS, so for each of those try to sync the service addresses

	if err := r.syncServiceAddresses(ctx, svc); err != nil {
		reqLogger.Error(err, "Failed to sync service addresses")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// update Loadbalancer service status
func (r *ServiceReconciler) syncServiceAddresses(ctx context.Context, svc *corev1.Service) error {
	sw := ServiceWrangler{
		client:  r.client,
		service: *svc,
	}

	pods, err := sw.FindPods(ctx)
	if err != nil {
		return err
	}

	ips, err := r.podIPs(ctx, pods.Items)
	if err != nil {
		return err
	}

	existingIPs := sw.ExistingIPs()

	sort.Strings(ips)
	sort.Strings(existingIPs)

	if reflect.DeepEqual(ips, existingIPs) {
		log.Info("Existing service addresses match, no need to update")
		return nil
	}

	log.Info("Addresses need to be updated for service:", "IPs", ips)

	return sw.UpdateAddresses(ctx, ips)
}

// newDaemonSetForService returns a DaemonSet with the same name/namespace as the cr
func newDaemonSetForService(svc *corev1.Service, portforwardImage string) *appsv1.DaemonSet {
	boolPointer := func(b bool) *bool { return &b }

	labels := map[string]string{
		"app":        "simplelb-" + svc.Name + "-ds-pod",
		svcNameLabel: svc.Name,
	}
	annotations := map[string]string{
		svcHashAnnotation: serviceHash(svc),
	}
	ownerReferences := []metav1.OwnerReference{
		{
			Name:       svc.Name,
			APIVersion: "v1",
			Kind:       "Service",
			UID:        svc.UID,
			Controller: boolPointer(true),
		},
	}

	// Add toleration to noderole.kubernetes.io/master=*:NoSchedule
	noScheduleToleration := corev1.Toleration{
		Key:      "node-role.kubernetes.io/master",
		Operator: "Exists",
		Effect:   "NoSchedule",
	}

	// Add toleration to CriticalAddonsOnly
	criticalAddonsOnlyToleration := corev1.Toleration{
		Key:      "CriticalAddonsOnly",
		Operator: "Exists",
	}

	containers := []corev1.Container{}
	// Create a container per exposed port
	for i, port := range svc.Spec.Ports {
		portName := port.Name
		if portName == "" {
			portName = fmt.Sprintf("port-%d", i)
		}
		container := corev1.Container{
			Name:            portName,
			Image:           portforwardImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Ports: []corev1.ContainerPort{
				{
					Name:          portName,
					ContainerPort: port.Port,
					HostPort:      port.Port,
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "SRC_PORT",
					Value: strconv.Itoa(int(port.Port)),
				},
				{
					Name:  "DEST_PROTO",
					Value: string(port.Protocol),
				},
				{
					Name:  "DEST_PORT",
					Value: strconv.Itoa(int(port.Port)),
				},
				{
					Name:  "DEST_IP",
					Value: svc.Spec.ClusterIP,
				},
			},
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{
						"NET_ADMIN",
					},
				},
			},
		}

		containers = append(containers, container)
	}

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "simplelb-" + svc.Name + "-ds",
			Namespace:       svc.Namespace,
			OwnerReferences: ownerReferences,
			Annotations:     annotations,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "false",
					},
				},
				Spec: corev1.PodSpec{
					Tolerations: []corev1.Toleration{
						noScheduleToleration,
						criticalAddonsOnlyToleration,
					},
					NodeSelector: map[string]string{
						daemonsetNodeLabel: "true",
					},
					InitContainers: []corev1.Container{
						{
							Name:  "sysctl",
							Image: portforwardImage,
							Command: []string{
								"sh",
								"-c",
								"sudo sysctl -w net.ipv4.ip_forward=1",
							},
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPointer(true),
							},
						},
					},
					Containers: containers,
				},
			},
			UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
				Type: appsv1.RollingUpdateDaemonSetStrategyType,
			},
		},
	}
}

// Calculates the "checksum" for the services spec part.
// This is used to track update need for the created daemonset.
func serviceHash(svc *corev1.Service) string {
	d, err := svc.Spec.Marshal()
	if err != nil {
		return ""
	}

	h := sha1.New()
	h.Write(d)

	checksum := fmt.Sprintf("%x", h.Sum(nil))

	return checksum
}
