package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"kubeowl/internal/models"
	"kubeowl/internal/services"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestMain silencia a saída de log durante os testes deste pacote.
func TestMain(m *testing.M) {
	log.SetOutput(io.Discard)
	os.Exit(m.Run())
}

// MockService é uma implementação mock da interface de serviço para testes.
type MockService struct {
	mock.Mock
}

// Garante que MockService implementa a interface services.Service
var _ services.Service = (*MockService)(nil)

func (m *MockService) GetOverviewData(ctx context.Context) (*models.OverviewResponse, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.OverviewResponse), args.Error(1)
}
func (m *MockService) GetNodeInfo(ctx context.Context) ([]models.NodeInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.NodeInfo), args.Error(1)
}
func (m *MockService) GetPodInfo(ctx context.Context) ([]models.PodInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PodInfo), args.Error(1)
}
func (m *MockService) GetServiceInfo(ctx context.Context) ([]models.ServiceInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ServiceInfo), args.Error(1)
}
func (m *MockService) GetIngressInfo(ctx context.Context) ([]models.IngressInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.IngressInfo), args.Error(1)
}
func (m *MockService) GetPvcInfo(ctx context.Context) ([]models.PvcInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.PvcInfo), args.Error(1)
}
func (m *MockService) GetEventInfo(ctx context.Context) ([]models.EventInfo, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.EventInfo), args.Error(1)
}

// TestHandlers_Success utiliza uma tabela de testes para validar todos os cenários de sucesso.
func TestHandlers_Success(t *testing.T) {
	mockService := new(MockService)
	router := NewRouter(nil, mockService) // Hub não é necessário para testes REST

	testCases := []struct {
		name         string
		handler      http.HandlerFunc
		mockSetup    func()
		expectedCode int
		path         string
	}{
		{
			name:    "OverviewHandler Success",
			handler: router.OverviewHandler,
			mockSetup: func() {
				mockService.On("GetOverviewData", mock.Anything).Return(&models.OverviewResponse{NodeCount: 3}, nil).Once()
			},
			path: "/api/overview",
		},
		{
			name:    "NodesHandler Success",
			handler: router.NodesHandler,
			mockSetup: func() {
				mockService.On("GetNodeInfo", mock.Anything).Return([]models.NodeInfo{{Name: "node-1"}}, nil).Once()
			},
			path: "/api/nodes",
		},
		{
			name:    "PodsHandler Success",
			handler: router.PodsHandler,
			mockSetup: func() {
				mockService.On("GetPodInfo", mock.Anything).Return([]models.PodInfo{{Name: "pod-1"}}, nil).Once()
			},
			path: "/api/pods",
		},
		{
			name:    "ServicesHandler Success",
			handler: router.ServicesHandler,
			mockSetup: func() {
				mockService.On("GetServiceInfo", mock.Anything).Return([]models.ServiceInfo{{Name: "service-1"}}, nil).Once()
			},
			path: "/api/services",
		},
		{
			name:    "IngressesHandler Success",
			handler: router.IngressesHandler,
			mockSetup: func() {
				mockService.On("GetIngressInfo", mock.Anything).Return([]models.IngressInfo{{Name: "ingress-1"}}, nil).Once()
			},
			path: "/api/ingresses",
		},
		{
			name:    "PvcsHandler Success",
			handler: router.PvcsHandler,
			mockSetup: func() {
				mockService.On("GetPvcInfo", mock.Anything).Return([]models.PvcInfo{{Name: "pvc-1"}}, nil).Once()
			},
			path: "/api/pvcs",
		},
		{
			name:    "EventsHandler Success",
			handler: router.EventsHandler,
			mockSetup: func() {
				mockService.On("GetEventInfo", mock.Anything).Return([]models.EventInfo{{Reason: "Scheduled"}}, nil).Once()
			},
			path: "/api/events",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.mockSetup()
			req, _ := http.NewRequest("GET", tc.path, nil)
			rr := httptest.NewRecorder()
			tc.handler(rr, req)
			assert.Equal(t, http.StatusOK, rr.Code)
			mockService.AssertExpectations(t)
		})
	}
}

// TestHandler_ServiceError testa um cenário de erro genérico.
func TestHandler_ServiceError(t *testing.T) {
	mockService := new(MockService)
	router := NewRouter(nil, mockService)

	expectedError := errors.New("falha no serviço")
	mockService.On("GetNodeInfo", mock.Anything).Return(nil, expectedError)

	req, _ := http.NewRequest("GET", "/api/nodes", nil)
	rr := httptest.NewRecorder()
	router.NodesHandler(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	var errorResponse map[string]string
	json.Unmarshal(rr.Body.Bytes(), &errorResponse)
	assert.Contains(t, errorResponse["error"], "Falha ao buscar dados dos nós")
	mockService.AssertExpectations(t)
}
