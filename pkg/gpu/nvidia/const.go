package nvidia

import (
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
)

// ShareUnit describes GPU Share
type ShareUnit string

const (
	resourceName  = "openxpu.com/xpu-shares"
	resourceCount = "openxpu.com/xpu-counts"
	serverSock    = pluginapi.DevicePluginPath + "openxpu.sock"

	OptimisticLockErrorMsg = "the object has been modified; please apply your changes to the latest version and try again"

	allHealthChecks             = "xids"
	containerTypeLabelKey       = "io.kubernetes.docker.type"
	containerTypeLabelSandbox   = "podsandbox"
	containerTypeLabelContainer = "container"
	containerLogPathLabelKey    = "io.kubernetes.container.logpath"
	sandboxIDLabelKey           = "io.kubernetes.sandbox.id"

	envNVGPU                   = "NVIDIA_VISIBLE_DEVICES"
	EnvResourceIndex           = "OPENXPU_XPU_SHARES_INDEX"
	EnvResourceByPod           = "OPENXPU_XPU_SHARES_POD"
	EnvResourceByContainer     = "OPENXPU_XPU_SHARES"
	EnvResourceByDev           = "OPENXPU_XPU_SHARES_TOTAL"
	EnvAssignedFlag            = "OPENXPU_XPU_SHARES_ALLOCATED"
	EnvResourceAssumeTime      = "OPENXPU_XPU_SHARES_FILTER_STAMP"
	EnvResourceAssignTime      = "OPENXPU_XPU_SHARES_ALLOCATED_STAMP"
	EnvNodeLabelForDisableCGPU = "xpu.disable.isolation"

	GiBPrefix = ShareUnit("GiB")
	MiBPrefix = ShareUnit("MiB")
	XPUShareUnit = 1000
)
