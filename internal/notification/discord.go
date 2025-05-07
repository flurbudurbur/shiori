package notification

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/flurbudurbur/Shiori/internal/domain"
	"github.com/flurbudurbur/Shiori/pkg/errors"
	"github.com/rs/zerolog"
)

type DiscordMessage struct {
	Content interface{}     `json:"content"`
	Embeds  []DiscordEmbeds `json:"embeds,omitempty"`
}

type DiscordEmbeds struct {
	Title       string                `json:"title"`
	Description string                `json:"description"`
	Color       int                   `json:"color"`
	Fields      []DiscordEmbedsFields `json:"fields,omitempty"`
	Timestamp   time.Time             `json:"timestamp"`
}
type DiscordEmbedsFields struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type EmbedColors int

const (
	LIGHT_BLUE  EmbedColors = 5814783  // 58b9ff
	RED         EmbedColors = 15548997 // ed4245
	GREEN       EmbedColors = 5763719  // 57f287
	GRAY        EmbedColors = 10070709 // 99aab5
	LightYellow EmbedColors = 16776960 // ffff00
)

type discordSender struct {
	log      zerolog.Logger
	Settings domain.Notification
}

func NewDiscordSender(log zerolog.Logger, settings domain.Notification) domain.NotificationSender {
	return &discordSender{
		log:      log.With().Str("sender", "discord").Logger(),
		Settings: settings,
	}
}

func (a *discordSender) Send(event domain.NotificationEvent, payload domain.NotificationPayload) error {
	m := DiscordMessage{
		Content: nil,
		Embeds:  []DiscordEmbeds{a.buildEmbed(event, payload)},
	}

	jsonData, err := json.Marshal(m)
	if err != nil {
		a.log.Error().Err(err).Msgf("discord client could not marshal data: %v", m)
		return errors.Wrap(err, "could not marshal data: %+v", m)
	}

	req, err := http.NewRequest(http.MethodPost, a.Settings.Webhook, bytes.NewBuffer(jsonData))
	if err != nil {
		a.log.Error().Err(err).Msgf("discord client request error: %v", event)
		return errors.Wrap(err, "could not create request")
	}

	req.Header.Set("Content-Type", "application/json")

	t := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := http.Client{Transport: t, Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		a.log.Error().Err(err).Msgf("discord client request error: %v", event)
		return errors.Wrap(err, "could not make request: %+v", req)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		a.log.Error().Err(err).Msgf("discord client request error: %v", event)
		return errors.Wrap(err, "could not read data")
	}

	defer res.Body.Close()

	a.log.Trace().Msgf("discord status: %v response: %v", res.StatusCode, string(body))

	// discord responds with 204, Notifiarr with 204 so lets take all 200 as ok
	if res.StatusCode >= 300 {
		a.log.Error().Err(err).Msgf("discord client request error: %v", string(body))
		return errors.New("bad status: %v body: %v", res.StatusCode, string(body))
	}

	a.log.Debug().Msg("notification successfully sent to discord")

	return nil
}

func (a *discordSender) CanSend(event domain.NotificationEvent) bool {
	if a.isEnabled() && a.isEnabledEvent(event) {
		return true
	}
	return false
}

func (a *discordSender) isEnabled() bool {
	if a.Settings.Enabled && a.Settings.Webhook != "" {
		return true
	}
	return false
}

func (a *discordSender) isEnabledEvent(event domain.NotificationEvent) bool {
	for _, e := range a.Settings.Events {
		if e == string(event) {
			return true
		}
	}

	return false
}

func (a *discordSender) buildEmbed(event domain.NotificationEvent, payload domain.NotificationPayload) DiscordEmbeds {

	color := LIGHT_BLUE
	switch event {
	case domain.NotificationEventSyncStarted:
		color = LightYellow
	case domain.NotificationEventSyncSuccess:
		color = GREEN
	case domain.NotificationEventSyncFailed:
		color = RED
	case domain.NotificationEventSyncError:
		color = RED
	case domain.NotificationEventTest:
		color = LIGHT_BLUE
	}

	var fields []DiscordEmbedsFields

	//if payload.Status != "" {
	//	f := DiscordEmbedsFields{
	//		Name:   "Status",
	//		Value:  payload.Status.String(),
	//		Inline: true,
	//	}
	//	fields = append(fields, f)
	//}
	//if payload.Indexer != "" {
	//	f := DiscordEmbedsFields{
	//		Name:   "Indexer",
	//		Value:  payload.Indexer,
	//		Inline: true,
	//	}
	//	fields = append(fields, f)
	//}
	//if payload.Filter != "" {
	//	f := DiscordEmbedsFields{
	//		Name:   "Filter",
	//		Value:  payload.Filter,
	//		Inline: true,
	//	}
	//	fields = append(fields, f)
	//}
	//if payload.Action != "" {
	//	f := DiscordEmbedsFields{
	//		Name:   "Action",
	//		Value:  payload.Action,
	//		Inline: true,
	//	}
	//	fields = append(fields, f)
	//}
	//if payload.ActionType != "" {
	//	f := DiscordEmbedsFields{
	//		Name:   "Action type",
	//		Value:  string(payload.ActionType),
	//		Inline: true,
	//	}
	//	fields = append(fields, f)
	//}
	//if payload.ActionClient != "" {
	//	f := DiscordEmbedsFields{
	//		Name:   "Action client",
	//		Value:  payload.ActionClient,
	//		Inline: true,
	//	}
	//	fields = append(fields, f)
	//}
	//if len(payload.Rejections) > 0 {
	//	f := DiscordEmbedsFields{
	//		Name:   "Reasons",
	//		Value:  fmt.Sprintf("```\n%v\n```", strings.Join(payload.Rejections, ", ")),
	//		Inline: false,
	//	}
	//	fields = append(fields, f)
	//}

	embed := DiscordEmbeds{
		Title:       payload.Subject,
		Description: "New Event!",
		Color:       int(color),
		Fields:      fields,
		Timestamp:   time.Now(),
	}

	if payload.Subject != "" && payload.Message != "" {
		embed.Title = payload.Subject
		embed.Description = payload.Message
	}

	return embed
}
