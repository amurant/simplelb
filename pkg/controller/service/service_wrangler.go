package service

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ServiceWrangler struct {
	client  client.Client
	service corev1.Service
}

func (sw *ServiceWrangler) ExistingIPs() []string {
	var ips []string

	for _, ingress := range sw.service.Status.LoadBalancer.Ingress {
		if ingress.IP != "" {
			ips = append(ips, ingress.IP)
		}
	}

	return ips
}

func (sw *ServiceWrangler) FindPods(ctx context.Context) (*corev1.PodList, error) {
	opts := []client.ListOption{
		client.InNamespace(sw.service.Namespace),
		client.MatchingLabels{svcNameLabel: sw.service.Name},
	}

	pods := &corev1.PodList{}
	err := sw.client.List(ctx, pods, opts...)

	return pods, err
}

func (sw *ServiceWrangler) UpdateAddresses(ctx context.Context, ips []string) error {
	svc := sw.service.DeepCopy()
	svc.Status.LoadBalancer.Ingress = nil
	for _, ip := range ips {
		svc.Status.LoadBalancer.Ingress = append(svc.Status.LoadBalancer.Ingress, corev1.LoadBalancerIngress{
			IP: ip,
		})
	}

	return sw.client.Status().Update(ctx, svc)
}
