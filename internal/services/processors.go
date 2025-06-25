package services

import (
	"fmt"
	"kubeowl/internal/models"
	"sort"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// processServiceInfo formata os dados brutos dos serviços em uma estrutura mais amigável.
func processServiceInfo(services *v1.ServiceList, userNamespaces map[string]bool) []models.ServiceInfo {
	serviceInfoList := []models.ServiceInfo{}
	if services == nil {
		return serviceInfoList
	}

	for _, service := range services.Items {
		if !userNamespaces[service.Namespace] {
			continue
		}
		externalIP := ""
		if len(service.Status.LoadBalancer.Ingress) > 0 {
			externalIP = service.Status.LoadBalancer.Ingress[0].IP
		}
		var portStrings []string
		for _, port := range service.Spec.Ports {
			portStrings = append(portStrings, fmt.Sprintf("%d:%d", port.Port, port.NodePort))
		}
		info := models.ServiceInfo{
			Name:       service.Name, Namespace: service.Namespace, Type: string(service.Spec.Type),
			ClusterIP: service.Spec.ClusterIP, ExternalIP: externalIP, Ports: strings.Join(portStrings, ", "),
		}
		serviceInfoList = append(serviceInfoList, info)
	}
	sort.Slice(serviceInfoList, func(i, j int) bool {
		if serviceInfoList[i].Namespace == serviceInfoList[j].Namespace {
			return serviceInfoList[i].Name < serviceInfoList[j].Name
		}
		return serviceInfoList[i].Namespace < serviceInfoList[j].Namespace
	})
	return serviceInfoList
}

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
	if status == "" {
		status = string(pod.Status.Phase)
	}
	return status, totalRestarts
}

// processIngressInfo formata os dados dos Ingresses.
func processIngressInfo(ingresses *networkingv1.IngressList, userNamespaces map[string]bool) []models.IngressInfo {
	ingressInfoList := []models.IngressInfo{}
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
		info := models.IngressInfo{
			Name: ingress.Name, Namespace: ingress.Namespace,
			Hosts: strings.Join(hosts, ", "), Service: backendService,
		}
		ingressInfoList = append(ingressInfoList, info)
	}
	sort.Slice(ingressInfoList, func(i, j int) bool { return ingressInfoList[i].Name < ingressInfoList[j].Name })
	return ingressInfoList
}

// processClusterCapacity calcula o uso total de CPU e memória do cluster.
func processClusterCapacity(nodes *v1.NodeList, nodeMetrics *metricsv1beta1.NodeMetricsList) models.ClusterCapacityInfo {
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
	return models.ClusterCapacityInfo{
		TotalCPU: totalCPU, UsedCPU: usedCPU, CPUUsagePercentage: cpuUsage,
		TotalMemory: totalMemory, UsedMemory: usedMemory, MemoryUsagePercentage: memUsage,
	}
}

// processNamespaces filtra e conta os namespaces que não são do sistema.
func processNamespaces(namespaces *v1.NamespaceList) (int, map[string]bool) {
	systemNamespaces := map[string]bool{"kube-system": true, "kube-public": true, "kube-node-lease": true, "cert-manager": true}
	systemPrefixes := []string{"cattle-", "fleet-", "cluster-fleet-", "local-p-", "p-", "user-", "kube-"}
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

// processNamespacesForSelector prepara a lista de namespaces para o dropdown do frontend.
func processNamespacesForSelector(namespaces *v1.NamespaceList) []models.NamespaceInfo {
	var namespaceInfoList []models.NamespaceInfo
	_, userNamespaces := processNamespaces(namespaces)
	for nsName := range userNamespaces {
		namespaceInfoList = append(namespaceInfoList, models.NamespaceInfo{Name: nsName})
	}
	sort.Slice(namespaceInfoList, func(i, j int) bool { return namespaceInfoList[i].Name < namespaceInfoList[j].Name })
	return namespaceInfoList
}

// processNodeInfo formata os dados dos nós do cluster.
func processNodeInfo(nodes *v1.NodeList, pods *v1.PodList, nodeMetrics *metricsv1beta1.NodeMetricsList) []models.NodeInfo {
	nodeInfoList := []models.NodeInfo{}
	if nodes == nil {
		return nodeInfoList
	}
	metricsMap := make(map[string]metricsv1beta1.NodeMetrics)
	if nodeMetrics != nil {
		for _, m := range nodeMetrics.Items {
			metricsMap[m.Name] = m
		}
	}
	podCountMap := make(map[string]int)
	if pods != nil {
		for _, pod := range pods.Items {
			podCountMap[pod.Spec.NodeName]++
		}
	}

	for _, node := range nodes.Items {
		role := "Worker"
		if _, ok := node.Labels["node-role.kubernetes.io/master"]; ok || node.Labels["node-role.kubernetes.io/control-plane"] == "true" {
			role = "Control-Plane"
		}

		var usedCPU, usedMemory, totalCPU, totalMemory int64
		var cpuUsagePercentage, memoryUsagePercentage float64
		
		totalCPU = node.Status.Allocatable.Cpu().MilliValue()
		totalMemory = node.Status.Allocatable.Memory().Value()
		
		if nm, ok := metricsMap[node.Name]; ok {
			usedCPU = nm.Usage.Cpu().MilliValue()
			usedMemory = nm.Usage.Memory().Value()
		}

		if totalCPU > 0 { cpuUsagePercentage = (float64(usedCPU) / float64(totalCPU)) * 100 }
		if totalMemory > 0 { memoryUsagePercentage = (float64(usedMemory) / float64(totalMemory)) * 100 }

		info := models.NodeInfo{
			Name: node.Name, Role: role, PodCount: podCountMap[node.Name],
			TotalCPU: fmt.Sprintf("%.2f Cores", float64(totalCPU)/1000.0),
			UsedCPU: fmt.Sprintf("%.2f Cores", float64(usedCPU)/1000.0),
			CPUUsagePercentage: cpuUsagePercentage,
			TotalMemory: fmt.Sprintf("%.2f Gi", float64(totalMemory)/(1024*1024*1024)),
			UsedMemory: fmt.Sprintf("%.2f Gi", float64(usedMemory)/(1024*1024*1024)),
			MemoryUsagePercentage: memoryUsagePercentage,
		}
		nodeInfoList = append(nodeInfoList, info)
	}
	sort.Slice(nodeInfoList, func(i, j int) bool { return nodeInfoList[i].Name < nodeInfoList[j].Name })
	return nodeInfoList
}

// processPodInfo formata os dados dos pods.
func processPodInfo(pods *v1.PodList, podMetricsList *metricsv1beta1.PodMetricsList, userNamespaces map[string]bool) []models.PodInfo {
	podInfoList := []models.PodInfo{}
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
			for _, container := range pm.Containers {
				usedCPUMilli += container.Usage.Cpu().MilliValue()
				usedMemoryBytes += container.Usage.Memory().Value()
			}
			usedCPU = fmt.Sprintf("%d m", usedCPUMilli)
			usedMemory = fmt.Sprintf("%.2f Mi", float64(usedMemoryBytes)/(1024*1024))
		}

		info := models.PodInfo{
			Uid: string(pod.UID), Name: pod.Name, Namespace: pod.Namespace, NodeName: pod.Spec.NodeName, Status: status, Restarts: restarts,
			UsedCPUMilli: usedCPUMilli, UsedCPU: usedCPU, UsedMemoryBytes: usedMemoryBytes, UsedMemory: usedMemory,
		}
		podInfoList = append(podInfoList, info)
	}
	return podInfoList
}

// processEvents formata e ordena os eventos do cluster.
func processEvents(events *v1.EventList, userNamespaces map[string]bool) []models.EventInfo {
	eventInfoList := []models.EventInfo{}
	if events == nil {
		return eventInfoList
	}
	for _, event := range events.Items {
		if !userNamespaces[event.Namespace] && event.Namespace != "" {
			continue
		}
		info := models.EventInfo{
			Timestamp: event.LastTimestamp.Time.Format(time.RFC3339),
			Type: event.Type, Reason: event.Reason,
			Object: fmt.Sprintf("%s/%s", event.InvolvedObject.Kind, event.InvolvedObject.Name),
			Message: event.Message, Namespace: event.Namespace,
		}
		eventInfoList = append(eventInfoList, info)
	}
	sort.Slice(eventInfoList, func(i, j int) bool {
		return eventInfoList[j].Timestamp < eventInfoList[i].Timestamp
	})
	if len(eventInfoList) > 50 {
		return eventInfoList[:50]
	}
	return eventInfoList
}

// processPvcs formata os dados dos PVCs.
func processPvcs(pvcs *v1.PersistentVolumeClaimList, userNamespaces map[string]bool) []models.PvcInfo {
	pvcInfoList := []models.PvcInfo{}
	if pvcs == nil {
		return pvcInfoList
	}
	for _, pvc := range pvcs.Items {
		if !userNamespaces[pvc.Namespace] {
			continue
		}
		storage := pvc.Spec.Resources.Requests[v1.ResourceStorage]
		info := models.PvcInfo{
			Name: pvc.Name, Namespace: pvc.Namespace, Status: string(pvc.Status.Phase),
			Capacity: storage.String(),
		}
		pvcInfoList = append(pvcInfoList, info)
	}
	sort.Slice(pvcInfoList, func(i, j int) bool { return pvcInfoList[i].Name < pvcInfoList[j].Name })
	return pvcInfoList
}