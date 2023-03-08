package nvidia

import (
	"fmt"
	"time"

	log "github.com/golang/glog"
	"golang.org/x/net/context"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

var (
	clientTimeout    = 30 * time.Second
	lastAllocateTime time.Time
)

// create docker client
func init() {
	kubeInit()
}

func buildErrResponse(reqs *pluginapi.AllocateRequest, podReqXPUShares uint) *pluginapi.AllocateResponse {
	responses := pluginapi.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		response := pluginapi.ContainerAllocateResponse{
			Envs: map[string]string{
				envNVGPU:               fmt.Sprintf("no-gpu-has-%dMiB-to-run", podReqXPUShares),
				EnvResourceIndex:       fmt.Sprintf("-1"),
				EnvResourceByPod:       fmt.Sprintf("%d", podReqXPUShares),
				EnvResourceByContainer: fmt.Sprintf("%d", uint(len(req.DevicesIDs))),
				EnvResourceByDev:       fmt.Sprintf("%d", getXPUShares()),
			},
		}
		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}
	return &responses
}

// Allocate which return list of devices.
func (m *NvidiaDevicePlugin) Allocate(ctx context.Context,
	reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	responses := pluginapi.AllocateResponse{}

	log.Infoln("-----allocating XPU shares is started-----")
	var (
		podReqXPUShares uint
		found           bool
		assumePod       *v1.Pod
	)

	for _, req := range reqs.ContainerRequests {
		podReqXPUShares += uint(len(req.DevicesIDs))
	}
	log.Infof("request pod XPU shares: %d", podReqXPUShares)

	m.Lock()
	defer m.Unlock()
	log.Infoln("checking...")
	pods, err := getCandidatePods()
	if err != nil {
		log.Infof("invalid allocation requst: failed to find candidate pods due to %v", err)
		return buildErrResponse(reqs, podReqXPUShares), nil
	}

	if log.V(4) {
		for _, pod := range pods {
			log.Infof("pod %s in ns %s request XPU shares %d with timestamp %v",
				pod.Name,
				pod.Namespace,
				getRequestXPUSharesFromPodResource(pod),
				getAssumeTimeFromPodAnnotation(pod))
		}
	}

	for _, pod := range pods {
		if getRequestXPUSharesFromPodResource(pod) == podReqXPUShares {
			log.Infof("found assumed pod %s in namespace %s with XPU shares %d",
				pod.Name,
				pod.Namespace,
				podReqXPUShares)
			assumePod = pod
			found = true
			break
		}
	}

	if found {
		id := getGPUIDFromPodAnnotation(assumePod)
		if id < 0 {
			log.Warningf("failed to get the dev ", assumePod)
		}

		candidateDevID := ""
		if id >= 0 {
			ok := false
			candidateDevID, ok = m.GetDeviceNameByIndex(uint(id))
			if !ok {
				log.Warningf("failed to find the dev for pod %v because it's not able to find dev with index %d",
					assumePod,
					id)
				id = -1
			}
		}

		if id < 0 {
			return buildErrResponse(reqs, podReqXPUShares), nil
		}

		// 1. create container requests
		for _, req := range reqs.ContainerRequests {
			reqXPUShare := uint(len(req.DevicesIDs))
			response    := pluginapi.ContainerAllocateResponse{
				Envs: map[string]string{
					envNVGPU:               candidateDevID,
					EnvResourceIndex:       fmt.Sprintf("%d", id),
					EnvResourceByPod:       fmt.Sprintf("%d", podReqXPUShares),
					EnvResourceByContainer: fmt.Sprintf("%d:%d-%d%%", id, (reqXPUShare*XPUShareUnit), (100*reqXPUShare/getXPUShares())),
					EnvResourceByDev:       fmt.Sprintf("%d", getXPUShares()),
				},
			}
			if m.disableXPU {
				response.Envs["YOYOWORKS_XPU_SHARES_DISABLE"] = "true"
			}
			responses.ContainerResponses = append(responses.ContainerResponses, &response)
		}

		// 2. update pod spec
		newPod := updatePodAnnotations(assumePod)
		_, err = clientset.CoreV1().Pods(newPod.Namespace).Update(newPod)
		if err != nil {
			// the object has been modified; please apply your changes to the latest version and try again
			if err.Error() == OptimisticLockErrorMsg {
				// retry
				pod, err := clientset.CoreV1().Pods(assumePod.Namespace).Get(assumePod.Name, metav1.GetOptions{})
				if err != nil {
					log.Warningf("failed due to %v", err)
					return buildErrResponse(reqs, podReqXPUShares), nil
				}
				newPod = updatePodAnnotations(pod)
				_, err = clientset.CoreV1().Pods(newPod.Namespace).Update(newPod)
				if err != nil {
					log.Warningf("failed due to %v", err)
					return buildErrResponse(reqs, podReqXPUShares), nil
				}
			} else {
				log.Warningf("failed due to %v", err)
				return buildErrResponse(reqs, podReqXPUShares), nil
			}
		}

	} else {
		log.Warningf("invalid allocation requst: request XPU shares %d can't be satisfied.",
			podReqXPUShares)
		// return &responses, fmt.Errorf("invalid allocation requst: request GPU memory %d can't be satisfied", reqXPUShare)
		return buildErrResponse(reqs, podReqXPUShares), nil
	}

	log.Infof("new allocated XPU shares info %v", &responses)
	log.Infoln("-----allocating XPU shares is ended-----")
	// // Add this to make sure the container is created at least
	// currentTime := time.Now()

	// currentTime.Sub(lastAllocateTime)

	return &responses, nil
}
