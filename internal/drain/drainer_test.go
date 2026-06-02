package drain

import (
	"context"
	"testing"
	"time"

	"github.com/d-madiou/spot-termination-handler/internal/config"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// helper: builds a test Drainer with a fake Kubernetes client
func newTestDrainer(nodeName string, fakeClient *fake.Clientset) *Drainer {
	cfg := &config.Config{
		NodeName:     nodeName,
		DrainTimeout: 30 * time.Second,
		GracePeriod:  10 * time.Second,
	}
	return NewDrainer(cfg, fakeClient)
}

// helper: builds a basic schedulable node
func newNode(name string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       v1.NodeSpec{Unschedulable: false},
	}
}

// helper: builds a regular pod on a given node
func newPod(name, namespace, nodeName string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{NodeName: nodeName},
	}
}

// helper: builds a DaemonSet-owned pod
func newDaemonSetPod(name, namespace, nodeName string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "DaemonSet", Name: "my-daemonset"},
			},
		},
		Spec: v1.PodSpec{NodeName: nodeName},
	}
}

// ── cordon() Tests ────────────────────────────────────────────────────────────

func TestCordon_Success(t *testing.T) {
	node := newNode("worker-1")
	fakeClient := fake.NewSimpleClientset(node)
	drainer := newTestDrainer("worker-1", fakeClient)

	err := drainer.cordon(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// verify node is now marked unschedulable
	updated, _ := fakeClient.CoreV1().Nodes().Get(
		context.Background(), "worker-1", metav1.GetOptions{},
	)
	if !updated.Spec.Unschedulable {
		t.Error("expected node to be unschedulable after cordon")
	}
}

func TestCordon_AlreadyCordoned(t *testing.T) {
	node := newNode("worker-1")
	node.Spec.Unschedulable = true // already cordoned
	fakeClient := fake.NewSimpleClientset(node)
	drainer := newTestDrainer("worker-1", fakeClient)

	err := drainer.cordon(context.Background())
	if err != nil {
		t.Errorf("expected no error for already cordoned node, got: %v", err)
	}
}

func TestCordon_NodeNotFound(t *testing.T) {
	fakeClient := fake.NewSimpleClientset() // no nodes registered
	drainer := newTestDrainer("worker-1", fakeClient)

	err := drainer.cordon(context.Background())
	if err == nil {
		t.Error("expected error for missing node, got nil")
	}
}

// ── getEvictablePods() Tests ──────────────────────────────────────────────────

func TestGetEvictablePods_FiltersDaemonSetPods(t *testing.T) {
	regularPod := newPod("app-pod", "default", "worker-1")
	daemonPod := newDaemonSetPod("daemon-pod", "kube-system", "worker-1")

	fakeClient := fake.NewSimpleClientset(regularPod, daemonPod)
	drainer := newTestDrainer("worker-1", fakeClient)

	pods, err := drainer.getEvictablePods(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(pods) != 1 {
		t.Errorf("expected 1 evictable pod, got %d", len(pods))
	}
	if pods[0].Name != "app-pod" {
		t.Errorf("expected app-pod, got %s", pods[0].Name)
	}
}

func TestGetEvictablePods_FiltersTerminatingPods(t *testing.T) {
	regularPod := newPod("app-pod", "default", "worker-1")
	terminatingPod := newPod("dying-pod", "default", "worker-1")

	// mark as already terminating
	now := metav1.Now()
	terminatingPod.DeletionTimestamp = &now

	fakeClient := fake.NewSimpleClientset(regularPod, terminatingPod)
	drainer := newTestDrainer("worker-1", fakeClient)

	pods, err := drainer.getEvictablePods(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(pods) != 1 {
		t.Errorf("expected 1 evictable pod, got %d", len(pods))
	}
}

func TestGetEvictablePods_EmptyNode(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	drainer := newTestDrainer("worker-1", fakeClient)

	pods, err := drainer.getEvictablePods(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(pods) != 0 {
		t.Errorf("expected 0 pods, got %d", len(pods))
	}
}

// ── Run() Tests ───────────────────────────────────────────────────────────────

func TestRun_FullDrainSequence(t *testing.T) {
	node := newNode("worker-1")
	pod1 := newPod("app-pod-1", "default", "worker-1")
	pod2 := newPod("app-pod-2", "default", "worker-1")
	daemonPod := newDaemonSetPod("daemon-pod", "kube-system", "worker-1")

	fakeClient := fake.NewSimpleClientset(node, pod1, pod2, daemonPod)
	drainer := newTestDrainer("worker-1", fakeClient)

	err := drainer.Run(context.Background())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// verify node is cordoned
	updated, _ := fakeClient.CoreV1().Nodes().Get(
		context.Background(), "worker-1", metav1.GetOptions{},
	)
	if !updated.Spec.Unschedulable {
		t.Error("expected node to be cordoned after Run()")
	}
}

func TestRun_NodeNotFound(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	drainer := newTestDrainer("worker-1", fakeClient)

	err := drainer.Run(context.Background())
	if err == nil {
		t.Error("expected error when node does not exist, got nil")
	}
}

// ── isDaemonSetPod() Tests ────────────────────────────────────────────────────

func TestIsDaemonSetPod_True(t *testing.T) {
	pod := newDaemonSetPod("daemon", "kube-system", "worker-1")
	if !isDaemonSetPod(*pod) {
		t.Error("expected true for DaemonSet pod")
	}
}

func TestIsDaemonSetPod_False(t *testing.T) {
	pod := newPod("app", "default", "worker-1")
	if isDaemonSetPod(*pod) {
		t.Error("expected false for regular pod")
	}
}
