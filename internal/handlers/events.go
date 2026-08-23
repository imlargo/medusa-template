package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/imlargo/medusa/pkg/medusa"
	"github.com/imlargo/medusa/pkg/medusa/core/handler"
	"github.com/imlargo/medusa/pkg/medusa/core/jwt"
	"github.com/imlargo/medusa/pkg/medusa/core/responses"
	"github.com/imlargo/sse"
)

// EventsHandler streams server-sent events to authenticated users.
//
// The whole transport — headers, flushing, heartbeats, write deadlines,
// reconnection and replay — belongs to the sse package. What is left here is the
// only part that is application-specific: deciding who may read which topics.
type EventsHandler struct {
	*handler.Handler

	broker *sse.Broker
	jwt    *jwt.JWT
}

// NewEventsHandler wires a handler onto an existing broker.
func NewEventsHandler(base *handler.Handler, broker *sse.Broker, jwtAuth *jwt.JWT) *EventsHandler {
	return &EventsHandler{
		Handler: base,
		broker:  broker,
		jwt:     jwtAuth,
	}
}

// UserTopic is the topic a single user's events are published to.
// Publishing to user.42.notifications reaches exactly user 42.
func UserTopic(userID uint, kind string) sse.Topic {
	return sse.MustTopic(fmt.Sprintf("user.%d.%s", userID, kind))
}

// Stream is the SSE endpoint: GET /v1/events
//
// It is registered without the JWT middleware on purpose. The library's
// Authorizer runs on the raw request, which is what lets it both authenticate
// and decide the subscriber's filters in one place — and set a Deadline from the
// token's own expiry, so a session ends when the credential does instead of
// outliving it.
func (h *EventsHandler) Stream() gin.HandlerFunc {
	return gin.WrapH(h.broker.Handler(sse.WithAuthorizer(h.authorize)))
}

// authorize resolves the caller's identity from their token and confines them to
// their own topics. A subscriber asking for anything outside user.<id>.> gets it
// reported as denied rather than silently dropped.
func (h *EventsHandler) authorize(r *http.Request) (sse.Grant, error) {
	token, err := bearerToken(r)
	if err != nil {
		return sse.Grant{}, sse.Unauthorized(err.Error())
	}

	claims, err := h.jwt.ParseToken(token)
	if err != nil {
		return sse.Grant{}, sse.Unauthorized("invalid or expired token")
	}

	grant := sse.Grant{
		Identity: fmt.Sprintf("user:%d", claims.UserID),
		Filters:  []sse.Filter{sse.MustFilter(fmt.Sprintf("user.%d.>", claims.UserID))},
	}

	// End the stream when the access token expires. The client reconnects with a
	// fresh token and resumes from its cursor, so nothing is missed.
	if claims.ExpiresAt != nil {
		grant.Deadline = claims.ExpiresAt.Time
	}

	return grant, nil
}

// Publish sends a notification to one user: POST /v1/events/publish
//
// It exists to make the endpoint above easy to try out; a real application would
// publish from the service that owns the event, not from an HTTP handler.
func (h *EventsHandler) Publish(ctx *medusa.Context, in *PublishEventRequest) (*PublishEventResponse, error) {
	userID, ok := ctx.UserID()
	if !ok {
		return nil, responses.Unauthorized("authentication required")
	}

	offset, err := h.broker.Publish(ctx.Ctx(), UserTopic(userID, "notifications"), in.Data, sse.Name(in.Event))
	if err != nil {
		return nil, responses.InternalWithMessage("could not publish the event", err)
	}

	return &PublishEventResponse{Offset: uint64(offset)}, nil
}

// PublishEventRequest is the body of a publish call.
type PublishEventRequest struct {
	Event string `json:"event" binding:"required"`
	Data  any    `json:"data"  binding:"required"`
}

// PublishEventResponse reports where the event landed in the log.
type PublishEventResponse struct {
	Offset uint64 `json:"offset"`
}

// bearerToken reads the token from the Authorization header, falling back to an
// access_token query parameter.
//
// The fallback exists because the browser EventSource API cannot set headers.
// A token in a URL is easy to leak through logs and referrers, so prefer the
// header wherever the client can send one.
func bearerToken(r *http.Request) (string, error) {
	if header := r.Header.Get("Authorization"); header != "" {
		scheme, token, found := strings.Cut(header, " ")
		if !found || !strings.EqualFold(scheme, "bearer") {
			return "", fmt.Errorf("authorization header must be in format 'Bearer <token>'")
		}
		return strings.TrimSpace(token), nil
	}

	if token := r.URL.Query().Get("access_token"); token != "" {
		return token, nil
	}

	return "", fmt.Errorf("authorization header is missing")
}
