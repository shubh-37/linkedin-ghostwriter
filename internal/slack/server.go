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
	approvalHandler *ApprovalHandler
	signingSecret   string
	processedEvents map[string]bool  // Add this for deduplication
}

func NewServer(client *Client, messageHandler *MessageHandler, approvalHandler *ApprovalHandler, signingSecret string) *Server {
	return &Server{
		client:          client,
		messageHandler:  messageHandler,
		approvalHandler: approvalHandler,
		signingSecret:   signingSecret,
		processedEvents: make(map[string]bool),
	}
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sv, err := slack.NewSecretsVerifier(r.Header, s.signingSecret)
	if err != nil {
		log.Printf("Error creating secrets verifier: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, err := sv.Write(body); err != nil {
		log.Printf("Error writing to verifier: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := sv.Ensure(); err != nil {
		log.Printf("Error verifying signature: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Printf("Error parsing event: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		err := json.Unmarshal(body, &r)
		if err != nil {
			log.Printf("Error unmarshaling challenge: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text")
		w.Write([]byte(r.Challenge))
		return
	}

	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		var eventEnvelope struct {
			EventID string `json:"event_id"`
		}
		var eventID string
		if err := json.Unmarshal(body, &eventEnvelope); err == nil && eventEnvelope.EventID != "" {
			eventID = eventEnvelope.EventID
		} else {
			eventID = eventsAPIEvent.TeamID + ":" + eventsAPIEvent.Type
		}
		
		if eventID != "" {
			if s.processedEvents[eventID] {
				w.WriteHeader(http.StatusOK)
				return
			}
			s.processedEvents[eventID] = true
		}

		innerEvent := eventsAPIEvent.InnerEvent
		ctx := context.Background()

		switch ev := innerEvent.Data.(type) {
		case *slackevents.MessageEvent:
			if err := s.messageHandler.HandleMessage(ctx, ev); err != nil {
				log.Printf("Error handling message: %v", err)
			}

		case *slackevents.AppMentionEvent:
			if err := s.messageHandler.HandleAppMention(ctx, ev); err != nil {
				log.Printf("Error handling mention: %v", err)
			}

		case *slackevents.ReactionAddedEvent:
			if err := s.approvalHandler.HandleReaction(ctx, ev); err != nil {
				log.Printf("Error handling reaction: %v", err)
			}

		default:
			log.Printf("Unsupported event type: %v", innerEvent.Type)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) Start(port string) error {
	http.HandleFunc("/slack/events", s.handleEvents)
	http.HandleFunc("/health", s.healthCheck)
	
	log.Printf("Slack server starting on port %s", port)
	
	return http.ListenAndServe(":"+port, nil)
}

func (s *Server) healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}