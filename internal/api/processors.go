package api

import (
	"fmt"
	"sort"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// processClusterCapacity calcula o uso total de CPU e memória do cluster.
func processClusterCapacity(nodes *v1.NodeList, nodeMetrics *metricsv1beta1.NodeMetricsList) ClusterCapacityInfo {
	var totalCPU, usedCPU, totalMemory, usedMemory int64

	if nodes != nil {
		for _, node := range nodes.Items {
			totalCPU += node.Status.Allocatable.Cpu().MilliValue()
			totalMemory += node.Status.Allocatable.Memory().Value()
		}
	}

	if nodeMetrics != nil {
		for _, m := range nodeMetrics.Items {
			usedCPU += m.Usage.Cpu().MilliValue()
			usedMemory += m.Usage.Memory().Value()
		}
	}

	var cpuUsage, memUsage float64
	if totalCPU > 0 {
		cpuUsage = (float64(usedCPU) / float64(totalCPU)) * 100
	}
	if totalMemory > 0 {
		memUsage = (float64(usedMemory) / float64(totalMemory)) * 100
	}

	return ClusterCapacityInfo{
		TotalCPU:              totalCPU,
		UsedCPU:               usedCPU,
		CPUUsagePercentage:    cpuUsage,
		TotalMemory:           totalMemory,
		UsedMemory:            usedMemory,
		MemoryUsagePercentage: memUsage,
	}
}

// processNamespaces filtra os namespaces do sistema.
func processNamespaces(namespaces *v1.NamespaceList) (int, map[string]bool) {
	systemNamespaces := map[string]bool{
		"default": true, "kube-system": true, "kube-public": true, "kube-node-lease": true,
		"local": true, "cert-manager": true,
	}
	systemPrefixes := []string{"cattle-", "fleet-", "cluster-fleet-", "local-p-", "p-", "user-"}
	userNamespaceCount := 0
	userNamespaces := map[string]bool{}

	if namespaces == nil {
		return 0, userNamespaces
	}

	for _, ns := range namespaces.Items {
		isSystem := false
		if systemNamespaces[ns.Name] {
			isSystem = true
		} else {
			for _, prefix := range systemPrefixes {
				if strings.HasPrefix(ns.Name, prefix) {
					isSystem = true
					break
				}
			}
		}
		if !isSystem {
			userNamespaceCount++
			userNamespaces[ns.Name] = true
		}
	}
	return userNamespaceCount, userNamespaces
}

// processNodeInfo formata os dados dos nós.
func processNodeInfo(nodes *v1.NodeList, pods *v1.PodList, nodeMetrics *metricsv1beta1.NodeMetricsList) []NodeInfo {
	nodeInfoList := []NodeInfo{}
	if nodes == nil {
		return nodeInfoList
	}

	for _, node := range nodes.Items {
		usedCPU, usedMemory := getNodeUsage(node.Name, nodeMetrics)
		podCount := countPodsOnNode(node.Name, pods)
		info := NodeInfo{
			Name:        node.Name,
			PodCount:    podCount,
			TotalCPU:    fmt.Sprintf("%.2f", float64(node.Status.Allocatable.Cpu().MilliValue())/1000.0),
			UsedCPU:     fmt.Sprintf("%.2f", float64(usedCPU.MilliValue())/1000.0),
			TotalMemory: fmt.Sprintf("%.2f Gi", float64(node.Status.Allocatable.Memory().Value())/(1024*1024*1024)),
			UsedMemory:  fmt.Sprintf("%.2f Gi", float64(usedMemory.Value())/(1024*1024*1024)),
		}
		nodeInfoList = append(nodeInfoList, info)
	}
	sort.Slice(nodeInfoList, func(i, j int) bool { return nodeInfoList[i].Name < nodeInfoList[j].Name })
	return nodeInfoList
}

func getNodeUsage(nodeName string, nodeMetrics *metricsv1beta1.NodeMetricsList) (*resource.Quantity, *resource.Quantity) {
	if nodeMetrics == nil {
		return resource.NewQuantity(0, resource.DecimalSI), resource.NewQuantity(0, resource.BinarySI)
	}
	for _, m := range nodeMetrics.Items {
		if m.Name == nodeName {
			return m.Usage.Cpu(), m.Usage.Memory()
		}
	}
	return resource.NewQuantity(0, resource.DecimalSI), resource.NewQuantity(0, resource.BinarySI)
}

func countPodsOnNode(nodeName string, pods *v1.PodList) int {
	if pods == nil {
		return 0
	}
	count := 0
	for _, pod := range pods.Items {
		if pod.Spec.NodeName == nodeName && pod.Status.Phase == v1.PodRunning {
			count++
		}
	}
	return count
}

// processPodInfo formata os dados dos pods.
func processPodInfo(pods *v1.PodList, podMetrics *metricsv1beta1.PodMetricsList, userNamespaces map[string]bool) []PodInfo {
	podInfoList := []PodInfo{}
	if podMetrics == nil || pods == nil {
		return podInfoList
	}

	podMap := make(map[string]v1.Pod)
	for _, p := range pods.Items {
		podMap[p.Namespace+"/"+p.Name] = p
	}

	for _, podMetric := range podMetrics.Items {
		if !userNamespaces[podMetric.Namespace] {
			continue
		}

		pod, ok := podMap[podMetric.Namespace+"/"+podMetric.Name]
		if !ok {
			continue
		}

		totalCPU := resource.NewQuantity(0, resource.DecimalSI)
		totalMemory := resource.NewQuantity(0, resource.BinarySI)
		for _, container := range podMetric.Containers {
			totalCPU.Add(*container.Usage.Cpu())
			totalMemory.Add(*container.Usage.Memory())
		}
		info := PodInfo{
			Name:            podMetric.Name,
			Namespace:       podMetric.Namespace,
			NodeName:        pod.Spec.NodeName,
			UsedCPUMilli:    totalCPU.MilliValue(),
			UsedCPU:         fmt.Sprintf("%d m", totalCPU.MilliValue()),
			UsedMemoryBytes: totalMemory.Value(),
			UsedMemory:      fmt.Sprintf("%.2f Mi", float64(totalMemory.Value())/(1024*1024)),
		}
		podInfoList = append(podInfoList, info)
	}
	return podInfoList
}

// processEvents formata e ordena os eventos.
func processEvents(events *v1.EventList, userNamespaces map[string]bool) []EventInfo {
	eventInfoList := []EventInfo{}
	if events == nil {
		return eventInfoList
	}

	sort.Slice(events.Items, func(i, j int) bool {
		return events.Items[j].LastTimestamp.Before(&events.Items[i].LastTimestamp)
	})

	for _, event := range events.Items {
		if !userNamespaces[event.Namespace] && event.Namespace != "" {
			continue
		}

		info := EventInfo{
			Timestamp: event.LastTimestamp.Format(time.RFC822),
			Type:      event.Type,
			Reason:    event.Reason,
			Object:    fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name),
			Message:   event.Message,
		}
		eventInfoList = append(eventInfoList, info)
		if len(eventInfoList) >= 50 {
			break
		}
	}
	return eventInfoList
}

// processPvcs formata os dados dos PVCs.
func processPvcs(pvcs *v1.PersistentVolumeClaimList, userNamespaces map[string]bool) []PvcInfo {
	pvcInfoList := []PvcInfo{}
	if pvcs == nil {
		return pvcInfoList
	}

	for _, pvc := range pvcs.Items {
		if !userNamespaces[pvc.Namespace] {
			continue
		}

		storage := pvc.Spec.Resources.Requests[v1.ResourceStorage]
		info := PvcInfo{
			Name:      pvc.Name,
			Namespace: pvc.Namespace,
			Status:    string(pvc.Status.Phase),
			Capacity:  storage.String(),
		}
		pvcInfoList = append(pvcInfoList, info)
	}
	sort.Slice(pvcInfoList, func(i, j int) bool { return pvcInfoList[i].Name < pvcInfoList[j].Name })
	return pvcInfoList
}
