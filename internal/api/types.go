package api

// OverviewResponse contém os dados para a tela principal do dashboard.
type OverviewResponse struct {
	IsRunningInCluster bool                `json:"isRunningInCluster"`
	DeploymentCount    int                 `json:"deploymentCount"`
	NamespaceCount     int                 `json:"namespaceCount"`
	NodeCount          int                 `json:"nodeCount"`
	Capacity           ClusterCapacityInfo `json:"capacity"`
}

// ServiceInfo contém informações formatadas sobre um Service.
type ServiceInfo struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Type       string `json:"type"`
	ClusterIP  string `json:"clusterIp"`
	ExternalIP string `json:"externalIp"`
	Ports      string `json:"ports"`
}

// IngressInfo contém informações sobre um Ingress.
type IngressInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Hosts     string `json:"hosts"`
	Service   string `json:"service"`
}

// NodeInfo contém informações sobre um nó do cluster.
type NodeInfo struct {
	Name                  string  `json:"name"`
	Role                  string  `json:"role"`
	PodCount              int     `json:"podCount"`
	TotalCPU              string  `json:"totalCpu"`
	UsedCPU               string  `json:"usedCpu"`
	CPUUsagePercentage    float64 `json:"cpuUsagePercentage"`
	TotalMemory           string  `json:"totalMemory"`
	UsedMemory            string  `json:"usedMemory"`
	MemoryUsagePercentage float64 `json:"memoryUsagePercentage"`
}

// PodInfo contém informações sobre um pod.
type PodInfo struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	NodeName        string `json:"nodeName"`
	Status          string `json:"status"`
	Restarts        int32  `json:"restarts"`
	UsedCPU         string `json:"usedCpu"`
	UsedCPUMilli    int64  `json:"usedCpuMilli"`
	UsedMemory      string `json:"usedMemory"`
	UsedMemoryBytes int64  `json:"usedMemoryBytes"`
}

// EventInfo contém informações sobre um evento do cluster.
type EventInfo struct {
	Timestamp string `json:"timestamp"`
	Type      string `json:"type"`
	Reason    string `json:"reason"`
	Object    string `json:"object"`
	Message   string `json:"message"`
}

// PvcInfo contém informações sobre um PersistentVolumeClaim.
type PvcInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"`
	Capacity  string `json:"capacity"`
}

// ClusterCapacityInfo resume o uso de recursos de todo o cluster.
type ClusterCapacityInfo struct {
	TotalCPU              int64   `json:"totalCpu"`
	UsedCPU               int64   `json:"usedCpu"`
	CPUUsagePercentage    float64 `json:"cpuUsagePercentage"`
	TotalMemory           int64   `json:"totalMemory"`
	UsedMemory            int64   `json:"usedMemory"`
	MemoryUsagePercentage float64 `json:"memoryUsagePercentage"`
}