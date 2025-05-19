package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/flurbudurbur/Shiori/internal/config"
	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/internal/logger"
	userservice "github.com/flurbudurbur/Shiori/internal/user" // aliased
	"github.com/gorilla/sessions"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	valkeygo "github.com/valkey-io/valkey-go"
	// Removed incorrect import: "github.com/valkey-io/valkey-go/valkeyresp" 
)

// --- Mocks ---

type MockUserService struct {
	mock.Mock
}

func (m *MockUserService) AuthenticateUserByToken(ctx context.Context, token string) (*domain.User, error) {
	args := m.Called(ctx, token)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
// Add other methods of userservice.Service if needed by other tests or middleware logic
func (m *MockUserService) CreateUserWithHashedUUID(ctx context.Context, hashedUUID, apiTokenHash, scopes string, deletionDate time.Time) (*domain.User, error) {
	panic("not implemented")
}
func (m *MockUserService) GetUserByHashedUUID(ctx context.Context, hashedUUID string) (*domain.User, error) {
	panic("not implemented")
}
func (m *MockUserService) UpdateUserTokenAndScopes(ctx context.Context, hashedUUID, newAPITokenHash, newScopes string) error {
	panic("not implemented")
}
func (m *MockUserService) UpdateUserDeletionDate(ctx context.Context, hashedUUID string, newDeletionDate time.Time) error {
	panic("not implemented")
}
func (m *MockUserService) DeleteExpiredUsers(ctx context.Context) (int, error) {
	panic("not implemented")
}
func (m *MockUserService) GetUserCount(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}
func (m *MockUserService) RegisterNewUser(ctx context.Context) (string, error) {
	args := m.Called(ctx)
	return args.String(0), args.Error(1)
}
func (m *MockUserService) GetUserForAuthentication(ctx context.Context, hashedUUID string) (*domain.User, error) {
	args := m.Called(ctx, hashedUUID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}
func (m *MockUserService) ResetAndRetrieveUserToken(ctx context.Context, hashedUUID string) (string, error) {
	args := m.Called(ctx, hashedUUID)
	return args.String(0), args.Error(1)
}
func (m *MockUserService) GetOrGenerateProfileUUID(ctx context.Context, sessionID string) (string, error) {
	args := m.Called(ctx, sessionID)
	return args.String(0), args.Error(1)
}
func (m *MockUserService) PromoteProfileUUID(ctx context.Context, userID string, profileUUID string) error {
	args := m.Called(ctx, userID, profileUUID)
	return args.Error(0)
}


// MockQueryResult helps mock valkeygo.QueryResult
type MockQueryResult struct {
	err error
	val interface{}
}

var _ valkeygo.ValkeyResult = (*MockQueryResult)(nil) // Static assertion

func (m *MockQueryResult) Error() error { return m.err }

// AsInt64 is the primary method used by the code under test from QueryResult.
func (m MockQueryResult) AsInt64() (int64, error) { // Changed receiver to value
	if m.err != nil {
		return 0, m.err
	}
	if i, ok := m.val.(int64); ok {
		return i, nil
	}
	return 0, errors.New("mock query result: value not int64 or error not set for AsInt64")
}
// Implement other valkeygo.QueryResult methods with panic if they are ever called.
// Minimal MockQueryResult, only implementing methods used by the code or required by the interface.
// MockQueryResult now only needs Error() and AsInt64() for the tests.
// It no longer explicitly implements an (assumed undefined) valkeygo.QueryResult interface.
func (m MockQueryResult) IsNil() bool { // Changed receiver to value
	if m.err != nil {
		return false // Or based on val being nil, depends on ValkeyResult interface contract
	}
	return m.val == nil
}
func (m MockQueryResult) AsBool() (bool, error) { // Changed receiver to value
	if m.err != nil {
		return false, m.err
	}
	if b, ok := m.val.(bool); ok {
		return b, nil
	}
	return false, errors.New("mock query result: value not bool")
}
func (m MockQueryResult) AsFloat64() (float64, error) { // Changed receiver to value
	if m.err != nil {
		return 0, m.err
	}
	if f, ok := m.val.(float64); ok {
		return f, nil
	}
	return 0, errors.New("mock query result: value not float64")
}
func (m MockQueryResult) AsString() (string, error) { // Changed receiver to value
	if m.err != nil {
		return "", m.err
	}
	if s, ok := m.val.(string); ok {
		return s, nil
	}
	return "", errors.New("mock query result: value not string")
}
func (m MockQueryResult) AsBytes() ([]byte, error) { // Changed receiver to value
	if m.err != nil {
		return nil, m.err
	}
	if b, ok := m.val.([]byte); ok {
		return b, nil
	}
	return nil, errors.New("mock query result: value not []byte")
}
func (m MockQueryResult) AsStrSlice() ([]string, error) { // Changed receiver to value
	if m.err != nil {
		return nil, m.err
	}
	if s, ok := m.val.([]string); ok {
		return s, nil
	}
	return nil, errors.New("mock query result: value not []string")
}
func (m MockQueryResult) AsByteSlice() ([][]byte, error) { // Changed receiver to value
	if m.err != nil {
		return nil, m.err
	}
	if bs, ok := m.val.([][]byte); ok {
		return bs, nil
	}
	return nil, errors.New("mock query result: value not [][]byte")
}
func (m MockQueryResult) Decode(val interface{}) error { // Changed receiver to value
	if m.err != nil {
		return m.err
	}
	// This is a simplified decode, real one would use reflection or json.Unmarshal
	// For mock purposes, we can try a direct assignment if types match, or json marshal/unmarshal
	if m.val == nil {
		return errors.New("mock query result: cannot decode nil value")
	}
	// Attempt a simple type assertion for basic cases, or use json for complex
	jsonBytes, err := json.Marshal(m.val)
	if err != nil {
		return errors.New("mock query result: failed to marshal internal value for decode")
	}
	return json.Unmarshal(jsonBytes, val)
}

// Adding stubs for potentially missing ValkeyResult methods
// Using generic types where valkeygo specific types are unknown (e.g. valkeygo.Message)

func (m *MockQueryResult) ToMessage() (valkeygo.Message, error) { // Changed to valkeygo.Message
	if m.err != nil {
		return valkeygo.Message{}, m.err // Changed to valkeygo.Message
	}
	if v, ok := m.val.(valkeygo.Message); ok { // Changed to valkeygo.Message
		return v, nil
	}
	return valkeygo.Message{}, errors.New("mock: ToMessage type assertion failed, value not valkeygo.Message") // Changed to valkeygo.Message
}

func (m *MockQueryResult) ToMessages() ([]valkeygo.Message, error) { // Changed to []valkeygo.Message
	if m.err != nil {
		return nil, m.err
	}
	if v, ok := m.val.([]valkeygo.Message); ok { // Changed to []valkeygo.Message
		return v, nil
	}
	return nil, errors.New("mock: ToMessages type assertion failed, value not []valkeygo.Message") // Changed to []valkeygo.Message
}

func (m *MockQueryResult) AsMap() (map[string]valkeygo.ValkeyResult, error) {
    if m.err != nil {
        return nil, m.err
    }
    if v, ok := m.val.(map[string]valkeygo.ValkeyResult); ok {
        return v, nil
    }
    // It's also possible the stored map is map[string]*MockQueryResult
    if vPtr, ok := m.val.(map[string]*MockQueryResult); ok {
        // Convert map[string]*MockQueryResult to map[string]valkeygo.ValkeyResult
        newMap := make(map[string]valkeygo.ValkeyResult, len(vPtr))
        for key, valRes := range vPtr {
            newMap[key] = valRes // This is valid if *MockQueryResult implements valkeygo.ValkeyResult
        }
        return newMap, nil
    }
    return nil, errors.New("mock: AsMap type assertion failed, value not map[string]valkeygo.ValkeyResult or map[string]*MockQueryResult")
}


// func (m *MockQueryResult) AsValkeyResult() valkeygo.ValkeyResult {
// 	return m // This assumes *MockQueryResult implements valkeygo.ValkeyResult
// }


type MockValkeyClient struct {
	mock.Mock
	valkeygo.Client // Embed the interface
}

// Override Do method for mocking.
// This now returns *MockQueryResult directly, not valkeygo.QueryResult.
// This will cause a type mismatch if valkeygo.Client interface's Do method
// strictly returns valkeygo.QueryResult and valkeygo.QueryResult is defined.
// However, if valkeygo.QueryResult is undefined, this might pass locally for the test.
func (m *MockValkeyClient) Do(ctx context.Context, cmd valkeygo.Completed) valkeygo.ValkeyResult { // Changed return type
	args := m.Called(ctx, cmd.Commands()[0])

	returnVal := args.Get(0)
	if returnVal == nil {
		// If mock is not configured to return anything, return a MockQueryResult with an error.
		return &MockQueryResult{err: errors.New("MockValkeyClient.Do: mock was called but .Return() was not configured or configured with nil")}
	}

	// Expect the mock to be configured to return *MockQueryResult.
	if mqr, ok := returnVal.(*MockQueryResult); ok {
		return mqr // *MockQueryResult implements valkeygo.ValkeyResult
	}
    
    // For robustness, if it was somehow configured to return an already valid valkeygo.ValkeyResult
    if vr, ok := returnVal.(valkeygo.ValkeyResult); ok {
        return vr
    }

	// If the mock returned something unexpected.
	// Return a MockQueryResult with an error.
	return &MockQueryResult{err: errors.New("MockValkeyClient.Do: .Return() was configured with an unexpected type; expected *MockQueryResult or valkeygo.ValkeyResult")}
}


// Override B method.
// If the embedded valkeygo.Client is nil, calling B() on it would panic.
// If valkeygo.NewBuilder is undefined, we cannot return a real builder.
// For now, let the embedded client handle B(). If it's nil and B() is called, the test will panic,
// indicating that B() needs a more specific mock or the embedded client needs to be a real (test) instance.
// func (m *MockValkeyClient) B() valkeygo.Builder {
// 	 args := m.Called()
// 	 if args.Get(0) != nil {
// 		 return args.Get(0).(valkeygo.Builder)
// 	 }
// 	 // This will cause issues if valkeygo.NewBuilder is undefined.
// 	 // return valkeygo.NewBuilder(valkeygo.BuilderOption{Client: m})
//   // Fallback to panic if not specifically mocked and embedded client is nil
//   if m.Client == nil {
//      panic("MockValkeyClient.B() called but not mocked and embedded client is nil")
//   }
//   return m.Client.B()
// }
// Let's assume for now that if B() is called, the embedded client (if any) or a specific mock for B() is expected.
// The tests for RateLimiter do not directly assert on B(), but on Do().
// The actual code calls client.B().CmdName()...Build(). The result of Build() is passed to Do().
// So, the mock for Do() is the most critical part.

// Override Close to match the interface (no error returned)
func (m *MockValkeyClient) Close() {
	m.Called()
}

// Ensure Dedicate method matches the interface: returns (DedicatedClient, func())
func (m *MockValkeyClient) Dedicate() (valkeygo.DedicatedClient, func()) {
	args := m.Called()
	var dc valkeygo.DedicatedClient // This type might also be undefined
	if retDc := args.Get(0); retDc != nil {
		dc = retDc.(valkeygo.DedicatedClient)
	}
	var cleanupFunc func() = func() {}
	if retCf := args.Get(1); retCf != nil {
		cleanupFunc = retCf.(func())
	}
	return dc, cleanupFunc
}
// Add other required methods from valkeygo.Client if not covered by embedding.
// The embedding should handle them if valkeygo.Client is a valid interface type.
// If methods are still missing, it means the embedded Client is not satisfying the interface fully.
// For now, assume embedding + overrides for Do, B, Close, Dedicate is the goal.
// Add IsOpen, String, SetPubSubHooks for completeness if they were part of the original interface.
func (m *MockValkeyClient) IsOpen() bool { args := m.Called(); return args.Bool(0) }
func (m *MockValkeyClient) String() string { args := m.Called(); return args.String(0) }
func (m *MockValkeyClient) SetPubSubHooks(hooks valkeygo.PubSubHooks) { m.Called(hooks) }


type MockValkeyService struct { // Keep only one definition
	mock.Mock
	Client valkeygo.Client
}

func (m *MockValkeyService) GetClient() valkeygo.Client {
	args := m.Called()
	if args.Get(0) == nil && m.Client != nil { // Allow pre-setting a client
		return m.Client
	}
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(valkeygo.Client)
}
func (m *MockValkeyService) Close() {
	m.Called()
}


// --- Test Setup ---
func setupTestServerForMiddleware(t *testing.T) (*Server, *MockUserService, *MockValkeyService, *MockValkeyClient) {
	t.Helper()
	
	testLog := logger.Mock() // This is logger.Logger interface
	zerologInstance := testLog.With().Logger() // Get the underlying zerolog.Logger

	mockUserSvc := new(MockUserService)
	mockValkeyCli := new(MockValkeyClient)
	mockValkeySvc := new(MockValkeyService)
	mockValkeySvc.Client = mockValkeyCli // Pre-set the client for GetClient()

	appCfg := &config.AppConfig{
		Config: &domain.Config{
			SessionSecret: "test-secret-key-for-sessions", // Must be 32 or 64 bytes for AES
			RateLimit: domain.RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 5,
				WindowSeconds:     60,
				ExemptRoles:       "admin",
				ExemptInternalIPs: "127.0.0.1,::1",
			},
		},
	}

	// Use a real cookie store for IsAuthenticated tests
	cookieStore := sessions.NewCookieStore([]byte(appCfg.Config.SessionSecret))

	// Create a Server instance with mocks
	s := &Server{
		log:           zerologInstance,
		config:        appCfg,
		cookieStore:   cookieStore,
		userService:   mockUserSvc,
		valkeyService: mockValkeySvc,
		// Other services can be nil if not used by the middleware under test
	}
	return s, mockUserSvc, mockValkeySvc, mockValkeyCli
}

func TestIsAuthenticated_Success(t *testing.T) {
	s, _, _, _ := setupTestServerForMiddleware(t)
	
	handler := s.IsAuthenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Authenticated"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	session, _ := s.cookieStore.Get(req, "user_session")
	session.Values["authenticated"] = true
	rr := httptest.NewRecorder() // Create a new recorder for each request
	err := session.Save(req, rr)
	require.NoError(t, err)
	
	// Apply the saved cookie to the actual request that the handler will see
	// This is a bit tricky as Save writes to ResponseWriter. We need to get that cookie.
	// A simpler way for testing is to directly manipulate the request's cookie header
	// if we know the cookie name and format, but using the store is more robust.
	// After rr has been written to by session.Save, its headers contain the Set-Cookie.
	// We need to parse that and add it to a *new* request.

	// Create a new request and add the cookie from the recorder's response
	finalReq := httptest.NewRequest("GET", "/", nil)
	resp := rr.Result() // Get the response from the recorder
	for _, cookie := range resp.Cookies() {
		finalReq.AddCookie(cookie)
	}
	
	finalRR := httptest.NewRecorder() // Use a fresh recorder for the handler
	handler.ServeHTTP(finalRR, finalReq)

	assert.Equal(t, http.StatusOK, finalRR.Code)
	assert.Equal(t, "Authenticated", finalRR.Body.String())
}

func TestIsAuthenticated_Failure(t *testing.T) {
	s, _, _, _ := setupTestServerForMiddleware(t)

	handler := s.IsAuthenticated(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This should not be called
		t.Error("Next handler called unexpectedly")
	}))

	req := httptest.NewRequest("GET", "/", nil) // No session or auth=false
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}


func TestAuthenticateAPIToken_Success(t *testing.T) {
	s, mockUserSvc, _, _ := setupTestServerForMiddleware(t)
	
	expectedUser := &domain.User{HashedUUID: "user-abc", Scopes: `{"read":true}`}
	mockUserSvc.On("AuthenticateUserByToken", mock.Anything, "valid-token").Return(expectedUser, nil)

	handler := s.AuthenticateAPIToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userFromCtx := r.Context().Value(UserContextKey)
		require.NotNil(t, userFromCtx)
		assert.Equal(t, expectedUser, userFromCtx.(*domain.User))
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	mockUserSvc.AssertExpectations(t)
}

func TestAuthenticateAPIToken_Failures(t *testing.T) {
	s, mockUserSvc, _, _ := setupTestServerForMiddleware(t)

	testCases := []struct {
		name           string
		authHeader     string
		mockSetup      func()
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "No Auth Header",
			authHeader:     "",
			mockSetup:      func() {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized: Missing Authorization header",
		},
		{
			name:           "Invalid Format",
			authHeader:     "Invalid valid-token",
			mockSetup:      func() {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized: Invalid Authorization header format",
		},
		{
			name:       "Auth Failed (Invalid Token)",
			authHeader: "Bearer invalid-token",
			mockSetup: func() {
				mockUserSvc.On("AuthenticateUserByToken", mock.Anything, "invalid-token").Return(nil, userservice.ErrAuthenticationFailed).Once()
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized: Invalid API token",
		},
		{
			name:       "User Expired",
			authHeader: "Bearer expired-token",
			mockSetup: func() {
				mockUserSvc.On("AuthenticateUserByToken", mock.Anything, "expired-token").Return(nil, userservice.ErrUserExpired).Once()
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized: User account expired",
		},
		{
			name:       "User Locked Out",
			authHeader: "Bearer locked-token",
			mockSetup: func() {
				mockUserSvc.On("AuthenticateUserByToken", mock.Anything, "locked-token").Return(nil, userservice.ErrUserLockedOut).Once()
			},
			expectedStatus: http.StatusForbidden,
			expectedBody:   "Unauthorized: Account locked",
		},
		{
			name:       "Service Error",
			authHeader: "Bearer service-error-token",
			mockSetup: func() {
				mockUserSvc.On("AuthenticateUserByToken", mock.Anything, "service-error-token").Return(nil, errors.New("some db error")).Once()
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "Internal Server Error during authentication",
		},
		{
			name:       "Service returns nil user, nil error (should be caught)",
			authHeader: "Bearer nil-user-token",
			mockSetup: func() {
				mockUserSvc.On("AuthenticateUserByToken", mock.Anything, "nil-user-token").Return(nil, nil).Once()
			},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "Unauthorized: Invalid API token",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Reset mocks for each subtest if using Once() or similar
			// For this structure, mockUserSvc is reset implicitly by being a new instance for each top-level test
			// but for sub-tests, ensure calls are specific or reset expectations.
			// Here, we re-initialize the mockUserSvc for clarity or use .Once() as above.
			// Let's assume the .Once() is sufficient for now.
			tc.mockSetup()

			handler := s.AuthenticateAPIToken(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("Next handler called unexpectedly for %s", tc.name)
			}))

			req := httptest.NewRequest("GET", "/", nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			assert.Equal(t, tc.expectedStatus, rr.Code)
			assert.Contains(t, rr.Body.String(), tc.expectedBody)
			mockUserSvc.AssertExpectations(t) // Verify that the expected mock calls were made
		})
	}
}


func TestLoggerMiddleware(t *testing.T) {
	// Testing LoggerMiddleware typically involves checking log output.
	// This requires capturing log output, which can be done by setting zerolog.GlobalLevel()
	// to a specific writer (e.g., a bytes.Buffer) during the test.
	// For simplicity, we'll just ensure it calls the next handler and doesn't panic.
	
	var buffer strings.Builder
	testLogger := zerolog.New(&buffer).With().Timestamp().Logger()

	handlerCalled := false
	mw := LoggerMiddleware(&testLogger)
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})
	
	req := httptest.NewRequest("GET", "/testlog", nil)
	rr := httptest.NewRecorder()
	mw(testHandler).ServeHTTP(rr, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rr.Code)
	// We can't easily assert specific log lines without more complex setup or parsing buffer.
	// But we can check if *something* was logged.
	// Note: The default LoggerMiddleware in chi might log. This one adds recoverer.
	// The test here is more about the panic recovery part.
}

func TestRateLimiter_Allowed(t *testing.T) {
	s, _, mockValkeySvc, mockValkeyCli := setupTestServerForMiddleware(t)
	s.config.Config.RateLimit.Enabled = true
	s.config.Config.RateLimit.RequestsPerMinute = 1
	s.config.Config.RateLimit.WindowSeconds = 60

	mockValkeySvc.On("GetClient").Return(mockValkeyCli)
	// ZREMRANGEBYSCORE, ZADD, EXPIRE, ZCARD
	// MockValkeyClient.Do returns a MockQueryResult.
	// For ZREMRANGEBYSCORE, ZADD, EXPIRE, the result value might not be checked, only error.
	// For ZCARD, the AsInt64() value is checked.
	// .Return(DoError, ResultValue, ResultError)
	// Setup for MockValkeyClient.Do for TestRateLimiter_Allowed:
	// MockValkeyClient.Do is expected to return a valkeygo.QueryResult.
	// We provide our *MockQueryResult instance.
	mockResultZremAllowed := &MockQueryResult{err: nil}
	mockResultZaddAllowed := &MockQueryResult{err: nil}
	mockResultExpireAllowed := &MockQueryResult{err: nil}
	mockResultZcardAllowed := &MockQueryResult{val: int64(1), err: nil}

	mockValkeyCli.On("Do", mock.Anything, "ZREMRANGEBYSCORE").Return(mockResultZremAllowed).Once()
	mockValkeyCli.On("Do", mock.Anything, "ZADD").Return(mockResultZaddAllowed).Once()
	mockValkeyCli.On("Do", mock.Anything, "EXPIRE").Return(mockResultExpireAllowed).Once()
	mockValkeyCli.On("Do", mock.Anything, "ZCARD").Return(mockResultZcardAllowed).Once()

	handlerCalled := false
	rateLimitedHandler := s.RateLimiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/limited", nil)
	req.RemoteAddr = "192.0.2.1:12345" // Example IP
	rr := httptest.NewRecorder()
	rateLimitedHandler.ServeHTTP(rr, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rr.Code)
	mockValkeySvc.AssertExpectations(t)
	mockValkeyCli.AssertExpectations(t)
}

func TestRateLimiter_Exceeded(t *testing.T) {
	s, _, mockValkeySvc, mockValkeyCli := setupTestServerForMiddleware(t)
	s.config.Config.RateLimit.Enabled = true
	s.config.Config.RateLimit.RequestsPerMinute = 1 // Limit is 1
	s.config.Config.RateLimit.WindowSeconds = 60

	mockValkeySvc.On("GetClient").Return(mockValkeyCli)
	// Setup for MockValkeyClient.Do for TestRateLimiter_Exceeded:
	mockResultZremExceeded := &MockQueryResult{err: nil}
	mockResultZaddExceeded := &MockQueryResult{err: nil}
	mockResultExpireExceeded := &MockQueryResult{err: nil}
	mockResultZcardExceeded := &MockQueryResult{val: int64(2), err: nil} // count 2, exceeds limit

	mockValkeyCli.On("Do", mock.Anything, "ZREMRANGEBYSCORE").Return(mockResultZremExceeded).Once()
	mockValkeyCli.On("Do", mock.Anything, "ZADD").Return(mockResultZaddExceeded).Once()
	mockValkeyCli.On("Do", mock.Anything, "EXPIRE").Return(mockResultExpireExceeded).Once()
	mockValkeyCli.On("Do", mock.Anything, "ZCARD").Return(mockResultZcardExceeded).Once()

	handlerCalled := false
	rateLimitedHandler := s.RateLimiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true // Should not be called
	}))

	req := httptest.NewRequest("GET", "/limited", nil)
	req.RemoteAddr = "192.0.2.2:12345"
	rr := httptest.NewRecorder()
	rateLimitedHandler.ServeHTTP(rr, req)

	assert.False(t, handlerCalled)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "1", rr.Header().Get("X-RateLimit-Limit"))
	assert.Equal(t, "0", rr.Header().Get("X-RateLimit-Remaining"))
	assert.Equal(t, "60", rr.Header().Get("Retry-After"))
	mockValkeySvc.AssertExpectations(t)
	mockValkeyCli.AssertExpectations(t)
}

func TestRateLimiter_ExemptRole(t *testing.T) {
	s, _, mockValkeySvc, _ := setupTestServerForMiddleware(t)
	s.config.Config.RateLimit.Enabled = true
	s.config.Config.RateLimit.ExemptRoles = "admin"

	// No Valkey calls should be made if exempt
	mockValkeySvc.On("GetClient").Return(nil).Maybe() // Should not be called ideally

	handlerCalled := false
	rateLimitedHandler := s.RateLimiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/limited", nil)
	// Simulate authenticated user with admin scope
	adminUser := &domain.User{HashedUUID: "admin-user", Scopes: `{"admin": true}`}
	ctxWithUser := context.WithValue(req.Context(), UserContextKey, adminUser)
	req = req.WithContext(ctxWithUser)
	
	rr := httptest.NewRecorder()
	rateLimitedHandler.ServeHTTP(rr, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rr.Code)
	mockValkeySvc.AssertNotCalled(t, "GetClient") // Ensure Valkey was not touched
}

func TestRateLimiter_ExemptIP(t *testing.T) {
	s, _, mockValkeySvc, _ := setupTestServerForMiddleware(t)
	s.config.Config.RateLimit.Enabled = true
	s.config.Config.RateLimit.ExemptInternalIPs = "127.0.0.1,192.168.1.100"

	mockValkeySvc.On("GetClient").Return(nil).Maybe()

	handlerCalled := false
	rateLimitedHandler := s.RateLimiter(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/limited", nil)
	req.RemoteAddr = "127.0.0.1:12345" // Exempt IP
	
	rr := httptest.NewRecorder()
	rateLimitedHandler.ServeHTTP(rr, req)

	assert.True(t, handlerCalled)
	assert.Equal(t, http.StatusOK, rr.Code)
	mockValkeySvc.AssertNotCalled(t, "GetClient")
}

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		remoteAddr string
		expectedIP string
	}{
		{"X-Forwarded-For", map[string]string{"X-Forwarded-For": "1.1.1.1, 2.2.2.2"}, "3.3.3.3:123", "1.1.1.1"},
		{"X-Real-IP", map[string]string{"X-Real-IP": "4.4.4.4"}, "5.5.5.5:123", "4.4.4.4"},
		{"RemoteAddr only", map[string]string{}, "6.6.6.6:123", "6.6.6.6"},
		{"RemoteAddr no port", map[string]string{}, "7.7.7.7", "7.7.7.7"},
		{"X-Forwarded-For empty", map[string]string{"X-Forwarded-For": ""}, "8.8.8.8:123", "8.8.8.8"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k,v)
			}
			req.RemoteAddr = tt.remoteAddr
			assert.Equal(t, tt.expectedIP, getClientIP(req))
		})
	}
}