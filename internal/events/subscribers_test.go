package events

import (
	"context"
	"testing"
	"time"

	// "github.com/asaskevich/EventBus" // Removed unused import
	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockEventBus is a mock for EventBus.Bus
type MockEventBus struct {
	mock.Mock
}

func (m *MockEventBus) Subscribe(topic string, fn interface{}) error {
	args := m.Called(topic, fn)
	return args.Error(0)
}

func (m *MockEventBus) SubscribeAsync(topic string, fn interface{}, transactional bool) error {
	args := m.Called(topic, fn, transactional)
	return args.Error(0)
}

func (m *MockEventBus) SubscribeOnce(topic string, fn interface{}) error {
	args := m.Called(topic, fn)
	return args.Error(0)
}

func (m *MockEventBus) SubscribeOnceAsync(topic string, fn interface{}) error {
	args := m.Called(topic, fn)
	return args.Error(0)
}

func (m *MockEventBus) Unsubscribe(topic string, handler interface{}) error {
	args := m.Called(topic, handler)
	return args.Error(0)
}

func (m *MockEventBus) Publish(topic string, args ...interface{}) {
	// Convert variadic args to a slice for Called
	m.Called(append([]interface{}{topic}, args...)...)
}

func (m *MockEventBus) HasCallback(topic string) bool {
	args := m.Called(topic)
	return args.Bool(0)
}

func (m *MockEventBus) WaitAsync() {
	m.Called()
}

// MockNotificationService is a mock for notification.Service
type MockNotificationService struct {
	mock.Mock
}

// Implement all methods of notification.Service interface
func (m *MockNotificationService) Find(ctx context.Context, params domain.NotificationQueryParams) ([]domain.Notification, int, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]domain.Notification), args.Int(1), args.Error(2)
}

func (m *MockNotificationService) FindByID(ctx context.Context, id int) (*domain.Notification, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockNotificationService) Store(ctx context.Context, n domain.Notification) (*domain.Notification, error) {
	args := m.Called(ctx, n)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockNotificationService) Update(ctx context.Context, n domain.Notification) (*domain.Notification, error) {
	args := m.Called(ctx, n)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Notification), args.Error(1)
}

func (m *MockNotificationService) Delete(ctx context.Context, id int) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockNotificationService) Send(event domain.NotificationEvent, payload domain.NotificationPayload) {
	m.Called(event, payload)
}

func (m *MockNotificationService) Test(ctx context.Context, n domain.Notification) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}
// Removed incorrect GetSender method

func TestNewSubscribers(t *testing.T) {
	log := logger.Mock()
	mockBus := new(MockEventBus)
	mockNotifSvc := new(MockNotificationService)

	// Expect Subscribe to be called during NewSubscribers (via Register)
	// We need to capture the handler function to test it later.
	var capturedHandler interface{}
	mockBus.On("Subscribe", "events:notification", mock.AnythingOfType("func(*domain.NotificationEvent, *domain.NotificationPayload)")).
		Run(func(args mock.Arguments) {
			capturedHandler = args.Get(1) // Capture the function
		}).
		Return(nil)

	_ = NewSubscribers(log, mockBus, mockNotifSvc)

	mockBus.AssertCalled(t, "Subscribe", "events:notification", mock.AnythingOfType("func(*domain.NotificationEvent, *domain.NotificationPayload)"))
	require.NotNil(t, capturedHandler, "Handler function should have been captured")

	// Test the captured handler (which is s.sendNotification)
	handlerFunc, ok := capturedHandler.(func(*domain.NotificationEvent, *domain.NotificationPayload))
	require.True(t, ok, "Captured handler is not of the expected type")

	testEvent := domain.NotificationEventAppUpdateAvailable // Corrected event name
	testPayload := domain.NotificationPayload{
		Subject: "Test Subject", // Corrected field name
		Message: "Test Message",
		Event: testEvent, // Ensure Event field is also set if needed by Send logic
		Timestamp: time.Now(),
	}

	mockNotifSvc.On("Send", testEvent, testPayload).Return()

	// Call the captured handler
	handlerFunc(&testEvent, &testPayload)

	mockNotifSvc.AssertCalled(t, "Send", testEvent, testPayload)
}

func TestSubscriber_Register_SubscribeError(t *testing.T) {
	log := logger.Mock()
	mockBus := new(MockEventBus)
	mockNotifSvc := new(MockNotificationService)

	expectedError := assert.AnError // Using testify's assert.AnError
	mockBus.On("Subscribe", "events:notification", mock.AnythingOfType("func(*domain.NotificationEvent, *domain.NotificationPayload)")).Return(expectedError)

	// NewSubscribers calls Register, so we test Register's error handling via NewSubscribers
	// No panic or unhandled error should occur. The error should be logged (which we can't easily check here without capturing logs).
	assert.NotPanics(t, func() {
		_ = NewSubscribers(log, mockBus, mockNotifSvc)
	})
	mockBus.AssertCalled(t, "Subscribe", "events:notification", mock.AnythingOfType("func(*domain.NotificationEvent, *domain.NotificationPayload)"))
}