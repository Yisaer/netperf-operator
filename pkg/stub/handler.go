package stub

import (
	"context"

	"github.com/piontec/netperf-operator/pkg/apis/app/v1alpha1"

	"github.com/operator-framework/operator-sdk/pkg/sdk"
	"github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	NetperfTypeServer = "server"
	NetperfTypeClient = "client"
)

func NewHandler() sdk.Handler {
	return &Handler{}
}

type Handler struct {
}

func (h *Handler) Handle(ctx context.Context, event sdk.Event) error {
	switch o := event.Object.(type) {
	case *v1alpha1.Netperf:
		logrus.Debugf("New Netperf event, name: %s, deleted: %v, status: %v", o.Name, event.Deleted, o.Status.Status)
		if event.Deleted {
			return h.deleteNetperfPods(o)
		}
		return h.handleNetperfUpdateEvent(o)
	case *v1.Pod:
		pod := event.Object.(*v1.Pod)
		if pod.ObjectMeta.OwnerReferences[0].Kind != "Netperf" {
			return nil
		}
		if pod.ObjectMeta.OwnerReferences[0].UID == "" {
			logrus.Warnf("Pod %s/%s has owner of type Netperf, but UID is unknown", pod.Namespace, pod.Name)
		}
		logrus.Debugf("New pod event: %s/%s, deleted status: %v", pod.Namespace, pod.Name, event.Deleted)
		return h.handlePodUpdateEvent(pod)
	default:
		logrus.Warnf("unknown event received: %s", event)
	}
	return nil
}

func (h *Handler) deleteNetperfPods(cr *v1alpha1.Netperf) error {
	//TODO: implement
	logrus.Fatalf("deleteNetperfPods Not implemented")
	return nil
}

func (h *Handler) handleNetperfUpdateEvent(cr *v1alpha1.Netperf) error {
	switch cr.Status.Status {
	case v1alpha1.NetperfPhaseInitial:
		return h.startServerPod(cr)
	case v1alpha1.NetperfPhaseServer:
		return h.startServerPod(cr)
	default:
		logrus.Debugf("Nothing needed to do for update event on Netperf %s in state %s",
			cr.Name, cr.Status.Status)
		return nil
	}
}

func (h *Handler) startServerPod(cr *v1alpha1.Netperf) error {
	serverPod := h.newNetperfPod(cr, "netperf-server", NetperfTypeServer, []string{})

	err := sdk.Create(serverPod)
	if err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create server pod : %v", err)
		return err
	}
	if err != nil && errors.IsAlreadyExists(err) {
		logrus.Debugf("Server pod is already created for netperf: %v", cr.Name)
		if cr.Status.ServerPod == serverPod.Name {
			logrus.Debugf("Server pod %v already registered for netperf %v", serverPod.Name,
				cr.Name)
			return nil
		}
	} else {
		logrus.Debugf("New server pod started for netperf: %s", cr.Name)
	}

	if err := h.registerNetperfServer(cr, serverPod); err != nil {
		return err
	}
	return nil
}

func (h *Handler) newNetperfPod(cr *v1alpha1.Netperf, name, netperfType string, command []string) *v1.Pod {
	labels := map[string]string{
		"app":          "netperf-operator",
		"netperf-type": netperfType,
	}
	pod := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: cr.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(cr, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "Netperf",
				}),
			},
			Labels: labels,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    name,
					Image:   "alectolytic/netperf",
					Command: command,
				},
			},
		},
	}
	return pod
}

func (h *Handler) registerNetperfServer(cr *v1alpha1.Netperf, serverPod *v1.Pod) error {
	c := cr.DeepCopy()
	c.Status.Status = v1alpha1.NetperfPhaseServer
	c.Status.ServerPod = serverPod.Name
	return sdk.Update(c)
}

func (h *Handler) handlePodUpdateEvent(pod *v1.Pod) error {
	logrus.Debugf("Trying to bind the pod %s with CR", pod.Name)
	cr := &v1alpha1.Netperf{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Netperf",
			APIVersion: "app.example.com/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.OwnerReferences[0].Name,
			Namespace: pod.Namespace,
		},
	}
	if err := sdk.Get(cr); err != nil {
		logrus.Errorf("error trying to fetch Netperf object %s/%s defined as owner of pod %s/%s: %v",
			pod.Namespace, pod.OwnerReferences[0].Name, pod.Namespace, pod.Name, err)
		return nil
	}

	isServerPod := pod.Name == cr.Status.ServerPod
	isClientPod := pod.Name == cr.Status.ClientPod
	if !isServerPod && !isClientPod {
		logrus.Errorf("pod with UID %s was not detected as server nor client pod of the CR",
			pod.UID)
		return nil
	}

	if isClientPod {
		logrus.Debugf("This is client pod event for CR %s: pod phase: %s, pod IP: %s",
			cr.Name, pod.Status.Phase, pod.Status.PodIP)
		return h.handleClientPodEvent(cr, pod)
	}
	if isServerPod {
		logrus.Debugf("This is server pod event for CR %s: pod phase: %s, pod IP: %s",
			cr.Name, pod.Status.Phase, pod.Status.PodIP)
		return h.handleServerPodEvent(cr, pod)
	}

	return nil
}

func (h *Handler) handleClientPodEvent(cr *v1alpha1.Netperf, pod *v1.Pod) error {
	if pod.Status.Phase == "Running" {
		logrus.Debugf("Client pod is running")
		return nil
	}

	if pod.Status.Phase == "Completed" {
		logrus.Debugf("test completed")
	}

	return nil
}

func (h *Handler) handleServerPodEvent(cr *v1alpha1.Netperf, pod *v1.Pod) error {

	if pod.Status.Phase != "Running" {
		logrus.Debugf("Server pod is not running yet")
		return nil
	}

	if cr.Status.ClientPod != "" {
		logrus.Debugf("It seems client pod as already created, skipping creation")
		return nil
	}

	logrus.Debugf("Creating client pod for netperf: %v", cr.Name)
	clientPod := h.newNetperfPod(cr, "netperf-client", NetperfTypeClient, []string{"netperf", "-H", pod.Status.PodIP})
	err := sdk.Create(clientPod)
	if err != nil && !errors.IsAlreadyExists(err) {
		logrus.Errorf("Failed to create client pod : %v", err)
		return err
	}
	if err != nil && errors.IsAlreadyExists(err) {
		logrus.Debugf("client pod already created for netperf: %s", cr.Name)
		if cr.Status.ClientPod == clientPod.Name {
			logrus.Debugf("Client pod %v already registered with netperf %v", clientPod.Name,
				cr.Name)
			return nil
		}
	} else {
		logrus.Debugf("New client pod started: %s/%s", clientPod.Namespace, clientPod.Name)
	}
	c := cr.DeepCopy()
	c.Status.Status = v1alpha1.NetperfPhaseTest
	c.Status.ClientPod = clientPod.Name
	sdk.Update(c)
	logrus.Debugf("Custom resource %s updated with client pod info: %s", cr.Name, clientPod.Name)

	return nil
}
