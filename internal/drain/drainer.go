package drain

import (
	"context"
	"fmt"
	"time"

	"github.com/d-madiou/spot-termination-handler/internal/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Drainer handles cordoning and draining a Kubernetes node
type Drainer struct {
	client       kubernetes.Interface
	nodeName     string
	drainTimeout time.Duration
	gracePeriod  time.Duration
}

// NewDrainer constructs a Drainer from config and a Kubernetes client
func NewDrainer(cfg *config.Config, client kubernetes.Interface) *Drainer {
	return &Drainer{
		client:       client,
		nodeName:     cfg.NodeName,
		drainTimeout: cfg.DrainTimeout,
		gracePeriod:  cfg.GracePeriod,
	}
}

// Run executes the full drain sequence:
//  1. Cordon the node  → no new pods scheduled
//  2. Evict all pods   → existing pods moved to healthy nodes
func (d *Drainer) Run(ctx context.Context) error {
	fmt.Printf("[drain] starting drain sequence for node: %s\n", d.nodeName)

	if err := d.cordon(ctx); err != nil {
		return fmt.Errorf("cordon failed: %w", err)
	}

	if err := d.evictPods(ctx); err != nil {
		return fmt.Errorf("evict pods failed: %w", err)
	}

	fmt.Printf("[drain] node %s drained successfully\n", d.nodeName)
	return nil
}

// cordon marks the node as unschedulable so no new pods land on it
func (d *Drainer) cordon(ctx context.Context) error {
	fmt.Printf("[drain] cordoning node: %s\n", d.nodeName)

	node, err := d.client.CoreV1().Nodes().Get(ctx, d.nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get node %s: %w", d.nodeName, err)
	}

	if node.Spec.Unschedulable {
		fmt.Printf("[drain] node %s is already cordoned\n", d.nodeName)
		return nil
	}

	node.Spec.Unschedulable = true
	_, err = d.client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to cordon node %s: %w", d.nodeName, err)
	}

	fmt.Printf("[drain] node %s cordoned successfully\n", d.nodeName)
	return nil
}

// evictPods lists and evicts all non-system pods from the node
func (d *Drainer) evictPods(ctx context.Context) error {
	fmt.Printf("[drain] listing pods on node: %s\n", d.nodeName)

	// create a timeout context for the entire eviction process
	drainCtx, cancel := context.WithTimeout(ctx, d.drainTimeout)
	defer cancel()

	pods, err := d.getEvictablePods(ctx)
	if err != nil {
		return err
	}

	if len(pods) == 0 {
		fmt.Printf("[drain] no evictable pods found on node %s\n", d.nodeName)
		return nil
	}

	fmt.Printf("[drain] evicting %d pods from node %s\n", len(pods), d.nodeName)

	for _, pod := range pods {
		if err := d.evictPod(drainCtx, pod); err != nil {
			// log but continue — best effort eviction
			fmt.Printf("[drain] warning: failed to evict pod %s/%s: %v\n",
				pod.Namespace, pod.Name, err)
		}
	}

	return nil
}

// getEvictablePods returns all pods on the node excluding DaemonSet pods
// and already terminating pods
func (d *Drainer) getEvictablePods(ctx context.Context) ([]v1.Pod, error) {
	podList, err := d.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", d.nodeName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	var evictable []v1.Pod
	for _, pod := range podList.Items {
		// skip pods already being terminated
		if pod.DeletionTimestamp != nil {
			continue
		}
		// skip DaemonSet pods — they are managed per-node and will be removed
		// automatically when the node terminates
		if isDaemonSetPod(pod) {
			continue
		}
		evictable = append(evictable, pod)
	}

	return evictable, nil
}

// evictPod deletes a single pod with the configured grace period
func (d *Drainer) evictPod(ctx context.Context, pod v1.Pod) error {
	gracePeriodSeconds := int64(d.gracePeriod.Seconds())

	fmt.Printf("[drain] evicting pod %s/%s\n", pod.Namespace, pod.Name)

	err := d.client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
		GracePeriodSeconds: &gracePeriodSeconds,
	})
	if err != nil {
		return fmt.Errorf("failed to delete pod %s/%s: %w", pod.Namespace, pod.Name, err)
	}

	return nil
}

// isDaemonSetPod returns true if the pod is owned by a DaemonSet
func isDaemonSetPod(pod v1.Pod) bool {
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == "DaemonSet" {
			return true
		}
	}
	return false
}
