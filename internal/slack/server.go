package slack

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type Server struct {
	client          *Client
	messageHandler  *MessageHandler
	approvalHandler *ApprovalHandler  // Add this
	signingSecret   string
}

func NewServer(client *Client, messageHandler *MessageHandler, approvalHandler *ApprovalHandler, signingSecret string) *Server {
	log.Printf("🔐 Slack signing secret configured (length: %d)", len(signingSecret))
	return &Server{
		client:          client,
		messageHandler:  messageHandler,
		approvalHandler: approvalHandler,  // Add this
		signingSecret:   signingSecret,
	}
}

// Update handleEvents to handle reaction events
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("❌ Error reading body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Verify the request signature
	sv, err := slack.NewSecretsVerifier(r.Header, s.signingSecret)
	if err != nil {
		log.Printf("❌ Error creating secrets verifier: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, err := sv.Write(body); err != nil {
		log.Printf("❌ Error writing to verifier: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := sv.Ensure(); err != nil {
		log.Printf("❌ Error verifying signature: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Parse the event
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Printf("❌ Error parsing event: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Handle URL verification challenge
	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal(body, &r)
		if err != nil {
			log.Printf("❌ Error unmarshaling challenge: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		log.Printf("✅ Responding to URL verification challenge")
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(r.Challenge))
		return
	}

	// Handle callback events
	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		innerEvent := eventsAPIEvent.InnerEvent
		ctx := context.Background()

		log.Printf("📬 Inner event type: %s", innerEvent.Type)

		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			log.Printf("💬 Message event received")
			if err := s.messageHandler.HandleMessage(ctx, ev); err != nil {
				log.Printf("❌ Error handling message: %v", err)
			}

		case *slackevents.AppMentionEvent:
			log.Printf("📣 App mention event received")
			if err := s.messageHandler.HandleAppMention(ctx, ev); err != nil {
				log.Printf("❌ Error handling mention: %v", err)
			}

		case *slackevents.ReactionAddedEvent:  // Add this case
			log.Printf("👍 Reaction added event received")
			if err := s.approvalHandler.HandleReaction(ctx, ev); err != nil {
				log.Printf("❌ Error handling reaction: %v", err)
			}

		default:
			log.Printf("⚠️ Unsupported event type: %v", innerEvent.Type)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// Start starts the Slack event server
func (s *Server) Start(port string) error {
	http.HandleFunc("/slack/events", s.handleEvents)
	http.HandleFunc("/health", s.healthCheck)
	
	log.Printf("🚀 Slack server starting on port %s", port)
	log.Printf("📡 Event endpoint: http://localhost:%s/slack/events", port)
	log.Printf("🏥 Health check: http://localhost:%s/health", port)
	
	return http.ListenAndServe(":"+port, nil)
}

// healthCheck provides a simple health check endpoint
func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}