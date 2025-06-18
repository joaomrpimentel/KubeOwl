package api

// NodeInfo contém informações sobre um nó do cluster.
type NodeInfo struct {
	Name        string `json:"name"`
	PodCount    int    `json:"podCount"`
	TotalCPU    string `json:"totalCpu"`
	UsedCPU     string `json:"usedCpu"`
	TotalMemory string `json:"totalMemory"`
	UsedMemory  string `json:"usedMemory"`
}

// PodInfo contém informações sobre um pod.
type PodInfo struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	NodeName        string `json:"nodeName"`
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

// RealtimeMetricsResponse é a estrutura principal da resposta da API em tempo real.
type RealtimeMetricsResponse struct {
	IsRunningInCluster bool                `json:"isRunningInCluster"`
	Nodes              []NodeInfo          `json:"nodes"`
	Pods               []PodInfo           `json:"pods"`
	Events             []EventInfo         `json:"events"`
	Pvcs               []PvcInfo           `json:"pvcs"`
	Capacity           ClusterCapacityInfo `json:"capacity"`
	DeploymentCount    int                 `json:"deploymentCount"`
	ServiceCount       int                 `json:"serviceCount"`
	NamespaceCount     int                 `json:"namespaceCount"`
}
