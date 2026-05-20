package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"

	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

// Images handles OpenAI Images API endpoints.
// POST /v1/images/generations
// POST /v1/images/edits
func (h *OpenAIGatewayHandler) Images(c *gin.Context) {
	setOpenAIClientTransportHTTP(c)

	apiKey, ok := middleware2.GetAPIKeyFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusUnauthorized, "authentication_error", "Invalid API key")
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		h.errorResponse(c, http.StatusInternalServerError, "api_error", "User context not found")
		return
	}
	reqLog := requestLogger(
		c,
		"handler.openai_gateway.images",
		zap.Int64("user_id", subject.UserID),
		zap.Int64("api_key_id", apiKey.ID),
		zap.Any("group_id", apiKey.GroupID),
	)
	if !h.ensureResponsesDependencies(c, reqLog) {
		return
	}

	body, err := pkghttputil.ReadRequestBodyWithPrealloc(c.Request)
	if err != nil {
		if maxErr, ok := extractMaxBytesError(err); ok {
			h.errorResponse(c, http.StatusRequestEntityTooLarge, "invalid_request_error", buildBodyTooLargeMessage(maxErr.Limit))
			return
		}
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to read request body")
		return
	}
	if len(body) == 0 {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Request body is empty")
		return
	}
	isMultipart := isOpenAIImagesMultipartContentType(c.GetHeader("Content-Type"))
	if !isMultipart && !gjson.ValidBytes(body) {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "Failed to parse request body")
		return
	}
	reqModel := extractOpenAIImagesRequestModel(body, c.GetHeader("Content-Type"))
	if reqModel == "" {
		h.errorResponse(c, http.StatusBadRequest, "invalid_request_error", "model is required")
		return
	}
	setOpsRequestContext(c, reqModel, false, body)
	setOpsEndpointContext(c, "", int16(service.RequestTypeSync))

	subscription, _ := middleware2.GetSubscriptionFromContext(c)
	if err := h.billingCacheService.CheckBillingEligibility(c.Request.Context(), apiKey.User, apiKey, apiKey.Group, subscription); err != nil {
		status, code, message := billingErrorDetails(err)
		h.errorResponse(c, status, code, message)
		return
	}

	channelMapping, _ := h.gatewayService.ResolveChannelMappingAndRestrict(c.Request.Context(), apiKey.GroupID, reqModel)
	forwardBody := body
	if channelMapping.Mapped {
		if isMultipart {
			channelMapping.Mapped = false
			channelMapping.MappedModel = reqModel
		} else {
			forwardBody = h.gatewayService.ReplaceModelInBody(body, channelMapping.MappedModel)
		}
	}

	failedAccountIDs := map[int64]struct{}{}
	var lastFailoverErr *service.UpstreamFailoverError
	for switchCount := 0; ; switchCount++ {
		selection, _, err := h.gatewayService.SelectAccountWithScheduler(
			c.Request.Context(),
			apiKey.GroupID,
			"",
			"",
			reqModel,
			failedAccountIDs,
			service.OpenAIUpstreamTransportHTTPSSE,
		)
		if err != nil || selection == nil || selection.Account == nil {
			if lastFailoverErr != nil {
				h.handleFailoverExhausted(c, lastFailoverErr, false)
				return
			}
			h.errorResponse(c, http.StatusServiceUnavailable, "api_error", "No available accounts")
			return
		}

		account := selection.Account
		setOpsSelectedAccount(c, account.ID, account.Platform)
		result, err := h.gatewayService.ForwardImages(c.Request.Context(), c, account, forwardBody, reqModel, GetInboundEndpoint(c))
		if err != nil {
			var failoverErr *service.UpstreamFailoverError
			if errors.As(err, &failoverErr) && switchCount < h.maxAccountSwitches {
				failedAccountIDs[account.ID] = struct{}{}
				lastFailoverErr = failoverErr
				continue
			}
			if !c.Writer.Written() {
				h.errorResponse(c, http.StatusBadGateway, "upstream_error", "Upstream request failed")
			}
			return
		}

		userAgent := c.GetHeader("User-Agent")
		clientIP := ip.GetClientIP(c)
		requestPayloadHash := service.HashUsageRequestPayload(body)
		h.submitUsageRecordTask(func(ctx context.Context) {
			if err := h.gatewayService.RecordUsage(ctx, &service.OpenAIRecordUsageInput{
				Result:             result,
				APIKey:             apiKey,
				User:               apiKey.User,
				Account:            account,
				Subscription:       subscription,
				InboundEndpoint:    GetInboundEndpoint(c),
				UpstreamEndpoint:   GetUpstreamEndpoint(c, account.Platform),
				UserAgent:          userAgent,
				IPAddress:          clientIP,
				RequestPayloadHash: requestPayloadHash,
				APIKeyService:      h.apiKeyService,
				ChannelUsageFields: channelMapping.ToUsageFields(reqModel, result.UpstreamModel),
			}); err != nil {
				logger.L().With(
					zap.String("component", "handler.openai_gateway.images"),
					zap.Int64("user_id", subject.UserID),
					zap.Int64("api_key_id", apiKey.ID),
					zap.Any("group_id", apiKey.GroupID),
					zap.String("model", reqModel),
					zap.Int64("account_id", account.ID),
				).Error("openai.images.record_usage_failed", zap.Error(err))
			}
		})
		return
	}
}

func isOpenAIImagesMultipartContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "multipart/form-data"
}

func extractOpenAIImagesRequestModel(body []byte, contentType string) string {
	if isOpenAIImagesMultipartContentType(contentType) {
		_, params, err := mime.ParseMediaType(contentType)
		if err != nil {
			return ""
		}
		boundary := params["boundary"]
		if boundary == "" {
			return ""
		}
		reader := multipart.NewReader(bytes.NewReader(body), boundary)
		for {
			part, err := reader.NextPart()
			if errors.Is(err, io.EOF) {
				return ""
			}
			if err != nil {
				return ""
			}
			if part.FormName() != "model" {
				_ = part.Close()
				continue
			}
			value, _ := io.ReadAll(io.LimitReader(part, 1024))
			_ = part.Close()
			return string(bytes.TrimSpace(value))
		}
	}
	modelResult := gjson.GetBytes(body, "model")
	if !modelResult.Exists() || modelResult.Type != gjson.String {
		return ""
	}
	return modelResult.String()
}
