package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/shopally-ai/internal/mocks"
	"github.com/shopally-ai/pkg/usecase"
	"github.com/stretchr/testify/suite"
)

type FXHandlerSuite struct {
	suite.Suite
	mockFX *mocks.IFXClient
	h      *FXHandler
}

func (s *FXHandlerSuite) SetupSuite() { gin.SetMode(gin.TestMode) }

func (s *FXHandlerSuite) SetupTest() {
	s.mockFX = mocks.NewIFXClient(s.T())
	s.h = NewFXHandler(s.mockFX)
}

// call invokes GetFX against a gin context wrapping the given target URL.
func (s *FXHandlerSuite) call(target string) (*httptest.ResponseRecorder, *http.Request) {
	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	s.h.GetFX(c)
	return rr, c.Request
}

// decodeFX unwraps the response envelope into the FX payload.
func (s *FXHandlerSuite) decodeFX(rr *httptest.ResponseRecorder) fxResponse {
	var resp struct {
		Data  fxResponse `json:"data"`
		Error *APIError  `json:"error"`
	}
	s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &resp))
	s.Require().Nil(resp.Error)
	return resp.Data
}

func (s *FXHandlerSuite) TestGetFX_HappyPath() {
	req := httptest.NewRequest(http.MethodGet, "/fx?from=usd&to=etb&amount=2.5", nil)
	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = req

	s.mockFX.On("GetRate", req.Context(), "USD", "ETB").Return(56.0, nil).Once()

	s.h.GetFX(c)
	s.Equal(http.StatusOK, rr.Result().StatusCode)
	s.Contains(rr.Header().Get("Content-Type"), "application/json")

	got := s.decodeFX(rr)
	s.Equal("USD", got.From)
	s.Equal("ETB", got.To)
	s.InDelta(56.0, got.Rate, 1e-9)
	s.InDelta(2.5, got.Amount, 1e-9)
	s.InDelta(140.0, got.Converted, 1e-9)
}

func (s *FXHandlerSuite) TestGetFX_DefaultsToUSDtoETB() {
	req := httptest.NewRequest(http.MethodGet, "/fx", nil)
	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = req

	s.mockFX.On("GetRate", req.Context(), "USD", "ETB").Return(1.23, nil).Once()

	s.h.GetFX(c)
	s.Equal(http.StatusOK, rr.Result().StatusCode)

	got := s.decodeFX(rr)
	s.Equal("USD", got.From)
	s.Equal("ETB", got.To)
	s.InDelta(1.23, got.Rate, 1e-9)
	s.InDelta(1.0, got.Amount, 1e-9)
	s.InDelta(1.23, got.Converted, 1e-9)
}

func (s *FXHandlerSuite) TestGetFX_InvalidAmount() {
	rr, _ := s.call("/fx?amount=abc")
	s.Equal(http.StatusBadRequest, rr.Result().StatusCode)

	var resp Envelope
	s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &resp))
	s.Require().NotNil(resp.Error)
	s.Equal(CodeInvalidInput, resp.Error.Code)
}

func (s *FXHandlerSuite) TestGetFX_UpstreamError() {
	req := httptest.NewRequest(http.MethodGet, "/fx?from=USD&to=ETB", nil)
	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = req

	s.mockFX.On("GetRate", req.Context(), "USD", "ETB").
		Return(0.0, usecase.ErrFXUnavailable).Once()

	s.h.GetFX(c)
	s.Equal(http.StatusBadGateway, rr.Result().StatusCode)

	var resp Envelope
	s.Require().NoError(json.Unmarshal(rr.Body.Bytes(), &resp))
	s.Require().NotNil(resp.Error)
	s.Equal(CodeFXUnavailable, resp.Error.Code)
}

// The provider URL carries the API key, so no upstream detail may reach the
// client regardless of which failure mode occurred.
func (s *FXHandlerSuite) TestGetFX_DoesNotLeakUpstreamDetail() {
	const secret = "super-secret-key"

	req := httptest.NewRequest(http.MethodGet, "/fx?from=USD&to=ETB", nil)
	rr := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rr)
	c.Request = req

	s.mockFX.On("GetRate", req.Context(), "USD", "ETB").
		Return(0.0, assertAnError("https://v6.exchangerate-api.com/v6/"+secret+"/latest/USD")).Once()

	s.h.GetFX(c)
	s.Equal(http.StatusBadGateway, rr.Result().StatusCode)
	s.NotContains(rr.Body.String(), secret)
	s.NotContains(rr.Body.String(), "exchangerate-api.com")
}

// helper provides an error without importing extra packages here
type assertAnError string

func (e assertAnError) Error() string { return string(e) }

func TestFXHandlerSuite(t *testing.T) { suite.Run(t, new(FXHandlerSuite)) }
