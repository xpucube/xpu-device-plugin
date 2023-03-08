package nvidia

import (
	"fmt"
	"strings"
	log "github.com/golang/glog"
	"github.com/NVIDIA/gpu-monitoring-tools/bindings/go/nvml"
	"golang.org/x/net/context"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

var (
	xpuShares uint
	metric    ShareUnit
)

func check(err error) {
	if err != nil {
		log.Fatalln("fatal:", err)
	}
}

func generateFakeDeviceID(realID string, fakeCounter uint) string {
	return fmt.Sprintf("%s-_-%d", realID, fakeCounter)
}

func extractRealDeviceID(fakeDeviceID string) string {
	return strings.Split(fakeDeviceID, "-_-")[0]
}

func setXPUShares(raw uint) {
	v := raw
	if metric == GiBPrefix {
		v = raw / XPUShareUnit
	}
	xpuShares = v
	log.Infof("set xpu shares: %d", xpuShares)
}

func getXPUShares() uint {
	return xpuShares
}

func getDeviceCount() uint {
	n, err := nvml.GetDeviceCount()
	check(err)
	return n
}

func getDevices() ([]*pluginapi.Device, map[string]uint) {
	n, err := nvml.GetDeviceCount()
	check(err)

	var devs []*pluginapi.Device
	realDevNames := map[string]uint{}

	for i := uint(0); i < n; i++ {
		d, err := nvml.NewDevice(i)
		check(err)
		var id uint
		log.Infof("deivce %s's path is %s", d.UUID, d.Path)
		_, err = fmt.Sscanf(d.Path, "/dev/nvidia%d", &id)
		check(err)

		realDevNames[d.UUID] = id
		if getXPUShares() == uint(0) {
			setXPUShares(uint(*d.Memory))
		}
		log.Infof("device memory: %d", uint(*d.Memory))
		for j := uint(0); j < getXPUShares(); j++ {
			fakeID := generateFakeDeviceID(d.UUID, j)
			devs = append(devs, &pluginapi.Device{
				ID:     fakeID,
				Health: pluginapi.Healthy,
			})
			log.Infoln("add device ID: " + fakeID)
		}
	}

	return devs, realDevNames
}

func deviceExists(devs []*pluginapi.Device, id string) bool {
	for _, d := range devs {
		if d.ID == id {
			return true
		}
	}
	return false
}

func watchXIDs(ctx context.Context, devs []*pluginapi.Device, xids chan<- *pluginapi.Device) {
	eventSet := nvml.NewEventSet()
	defer nvml.DeleteEventSet(eventSet)

	for _, d := range devs {
		realDeviceID := extractRealDeviceID(d.ID)
		err := nvml.RegisterEventForDevice(eventSet, nvml.XidCriticalError, realDeviceID)
		if err != nil && strings.HasSuffix(err.Error(), "not supported") {
			log.Infof("warning: %s (%s) is too old to support healthchecking: %s. marking it unhealthy.", realDeviceID, d.ID, err)

			xids <- d
			continue
		}

		if err != nil {
			log.Fatalf("fatal error:", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		e, err := nvml.WaitForEvent(eventSet, 5000)
		if err != nil && e.Etype != nvml.XidCriticalError {
			continue
		}

		// FIXME: formalize the full list and document it.
		// http://docs.nvidia.com/deploy/xid-errors/index.html#topic_4
		// Application errors: the GPU should still be healthy
		if e.Edata == 31 || e.Edata == 43 || e.Edata == 45 {
			continue
		}

		if e.UUID == nil || len(*e.UUID) == 0 {
			// All devices are unhealthy
			for _, d := range devs {
				xids <- d
			}
			continue
		}

		for _, d := range devs {
			if extractRealDeviceID(d.ID) == *e.UUID {
				xids <- d
			}
		}
	}
}
