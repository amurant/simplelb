package service

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

func (r *ServiceReconciler) getNode(ctx context.Context, name string) (*corev1.Node, error) {
	node := &corev1.Node{}
	if err := r.client.Get(ctx, types.NamespacedName{Name: name, Namespace: ""}, node); err != nil {
		return nil, err
	}
	return node, nil
}

func (r *ServiceReconciler) podIPs(ctx context.Context, pods []corev1.Pod) ([]string, error) {
	ips := map[string]bool{}

	for _, pod := range pods {
		if pod.Spec.NodeName == "" || pod.Status.PodIP == "" {
			continue
		}
		if !isPodReady(&pod) {
			continue
		}

		node, err := r.getNode(ctx, pod.Spec.NodeName)
		if errors.IsNotFound(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		var internal string
		var external string
		for _, addr := range node.Status.Addresses {
			// Prefer external address, if not set on node use internal address
			if addr.Type == corev1.NodeExternalIP {
				external = addr.Address
			}

			if addr.Type == corev1.NodeInternalIP {
				internal = addr.Address
			}
		}
		if external != "" {
			ips[external] = true
		} else {
			ips[internal] = true
		}
	}

	var ipList []string
	for k := range ips {
		ipList = append(ipList, k)
	}
	return ipList, nil
}

func isPodReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == "Ready" && c.Status == "True" {
			return true
		}
	}

	return false
}
