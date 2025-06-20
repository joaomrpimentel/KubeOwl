package api

import (
	"fmt"
	"sort"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// getPodStatus determina a condição detalhada de um pod.
func getPodStatus(pod v1.Pod) (string, int32) {
	var totalRestarts int32
	var status string
	for _, cs := range pod.Status.ContainerStatuses {
		totalRestarts += cs.RestartCount
		if cs.State.Waiting != nil {
			status = cs.State.Waiting.Reason
			if status == "CrashLoopBackOff" {
				return status, totalRestarts
			}
		}
		if cs.State.Terminated != nil {
			status = cs.State.Terminated.Reason
		}
	}

	// Se nenhum status de contêiner se destacou, usa a fase geral do pod.
	if status == "" {
		status = string(pod.Status.Phase)
	}

	return status, totalRestarts
}

// processIngressInfo formata os dados dos Ingresses.
func processIngressInfo(ingresses *networkingv1.IngressList, userNamespaces map[string]bool) []IngressInfo {
	ingressInfoList := []IngressInfo{}
	if ingresses == nil {
		return ingressInfoList
	}

	for _, ingress := range ingresses.Items {
		if !userNamespaces[ingress.Namespace] {
			continue
		}

		var hosts []string
		var backendService string
		for _, rule := range ingress.Spec.Rules {
			hosts = append(hosts, rule.Host)
			if rule.HTTP != nil && len(rule.HTTP.Paths) > 0 {
				backendService = rule.HTTP.Paths[0].Backend.Service.Name
			}
		}

		info := IngressInfo{
			Name:      ingress.Name,
			Namespace: ingress.Namespace,
			Hosts:     strings.Join(hosts, ", "),
			Service:   backendService,
		}
		ingressInfoList = append(ingressInfoList, info)
	}
	sort.Slice(ingressInfoList, func(i, j int) bool { return ingressInfoList[i].Name < ingressInfoList[j].Name })
	return ingressInfoList
}

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

// processNamespaces filtra e conta os namespaces que não são do sistema.
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

// processNodeInfo formata os dados dos nós do cluster, identificando o master.
func processNodeInfo(nodes *v1.NodeList, pods *v1.PodList, nodeMetrics *metricsv1beta1.NodeMetricsList) []NodeInfo {
	nodeInfoList := []NodeInfo{}
	if nodes == nil {
		return nodeInfoList
	}

	for _, node := range nodes.Items {
		role := "Worker" // Define 'Worker' como padrão.
		if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok {
			role = "Control-Plane"
		}
		if _, ok := node.Labels["node-role.kubernetes.io/control-plane"]; ok {
			role = "Control-Plane"
		}

		usedCPU, usedMemory := getNodeUsage(node.Name, nodeMetrics)
		podCount := countPodsOnNode(node.Name, pods)

		var cpuUsagePercentage, memoryUsagePercentage float64
		
		totalCPUMilli := node.Status.Allocatable.Cpu().MilliValue()
		usedCPUMilli := usedCPU.MilliValue()
		if totalCPUMilli > 0 {
			cpuUsagePercentage = (float64(usedCPUMilli) / float64(totalCPUMilli)) * 100
		}
		
		totalMemoryBytes := node.Status.Allocatable.Memory().Value()
		usedMemoryBytes := usedMemory.Value()
		if totalMemoryBytes > 0 {
			memoryUsagePercentage = (float64(usedMemoryBytes) / float64(totalMemoryBytes)) * 100
		}
		
		info := NodeInfo{
			Name:                  node.Name,
			Role:                  role,
			PodCount:              podCount,
			TotalCPU:              fmt.Sprintf("%.2f", float64(node.Status.Allocatable.Cpu().MilliValue())/1000.0),
			UsedCPU:               fmt.Sprintf("%.2f", float64(usedCPU.MilliValue())/1000.0),
			CPUUsagePercentage:    cpuUsagePercentage,
			TotalMemory:           fmt.Sprintf("%.2f Gi", float64(node.Status.Allocatable.Memory().Value())/(1024*1024*1024)),
			UsedMemory:            fmt.Sprintf("%.2f Gi", float64(usedMemory.Value())/(1024*1024*1024)),
			MemoryUsagePercentage: memoryUsagePercentage,
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
		if pod.Spec.NodeName == nodeName {
			count++
		}
	}
	return count
}

// processPodInfo formata os dados dos pods, incluindo status detalhado e reinicializações.
func processPodInfo(pods *v1.PodList, podMetricsList *metricsv1beta1.PodMetricsList, userNamespaces map[string]bool) []PodInfo {
	podInfoList := []PodInfo{}
	if pods == nil {
		return podInfoList
	}

	metricsMap := make(map[string]metricsv1beta1.PodMetrics)
	if podMetricsList != nil {
		for _, pm := range podMetricsList.Items {
			metricsMap[pm.Namespace+"/"+pm.Name] = pm
		}
	}

	for _, pod := range pods.Items {
		if !userNamespaces[pod.Namespace] {
			continue
		}

		status, restarts := getPodStatus(pod)
		var usedCPU, usedMemory string
		var usedCPUMilli, usedMemoryBytes int64

		if pm, ok := metricsMap[pod.Namespace+"/"+pod.Name]; ok {
			totalCPU := resource.NewQuantity(0, resource.DecimalSI)
			totalMemory := resource.NewQuantity(0, resource.BinarySI)
			for _, container := range pm.Containers {
				totalCPU.Add(*container.Usage.Cpu())
				totalMemory.Add(*container.Usage.Memory())
			}
			usedCPUMilli = totalCPU.MilliValue()
			usedCPU = fmt.Sprintf("%d m", usedCPUMilli)
			usedMemoryBytes = totalMemory.Value()
			usedMemory = fmt.Sprintf("%.2f Mi", float64(usedMemoryBytes)/(1024*1024))
		}

		info := PodInfo{
			Name:            pod.Name,
			Namespace:       pod.Namespace,
			NodeName:        pod.Spec.NodeName,
			Status:          status,
			Restarts:        restarts,
			UsedCPUMilli:    usedCPUMilli,
			UsedCPU:         usedCPU,
			UsedMemoryBytes: usedMemoryBytes,
			UsedMemory:      usedMemory,
		}
		podInfoList = append(podInfoList, info)
	}
	return podInfoList
}

// processEvents formata e ordena os eventos do cluster.
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

// processPvcs formata os dados dos PVCs (PersistentVolumeClaims).
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
