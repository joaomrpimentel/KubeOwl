package services

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	metricsvake "k8s.io/metrics/pkg/client/clientset/versioned/fake"
)

// TestGetOverviewData_Success testa o caminho feliz da função GetOverviewData.
func TestGetOverviewData_Success(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&v1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	)
	fakeMetricsClient := metricsvake.NewSimpleClientset()

	service := NewK8sService(fakeClient, fakeMetricsClient)

	overview, err := service.GetOverviewData(context.Background())

	assert.NoError(t, err)
	assert.NotNil(t, overview)
	assert.Equal(t, 1, overview.NodeCount)
	assert.Equal(t, 1, overview.NamespaceCount) // app-ns
}

// TestGetService_K8sError testa o tratamento de erro quando o cliente K8s falha.
func TestGetService_K8sError(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	expectedError := errors.New("erro forçado da API")
	fakeClient.PrependReactor("list", "*", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, expectedError
	})

	service := NewK8sService(fakeClient, metricsvake.NewSimpleClientset())

	_, err := service.GetPodInfo(context.Background())
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)

	_, err = service.GetNodeInfo(context.Background())
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
}

// TestGetPodInfo_Success testa o caminho feliz da função GetPodInfo.
func TestGetPodInfo_Success(t *testing.T) {
	fakeClient := fake.NewSimpleClientset(
		&v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "app-ns"}},
		&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "app-ns"}},
	)
	fakeMetricsClient := metricsvake.NewSimpleClientset()
	service := NewK8sService(fakeClient, fakeMetricsClient)

	pods, err := service.GetPodInfo(context.Background())

	assert.NoError(t, err)
	assert.Len(t, pods, 1)
	assert.Equal(t, "pod-1", pods[0].Name)
}
