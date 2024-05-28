package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
	"github.com/unstablebuild/blue/issue"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type bot struct {
	ctx        context.Context
	cancel     func()
	slack      *socketmode.Client
	firestore  *firestore.Client
	channelID  string
	collection string
}

func newBot(
	debug bool,
	projectID, collection string,
	channelID, botToken, appToken string,
) (*bot, error) {
	if !strings.HasPrefix(appToken, "xapp-") {
		return nil, errors.New("appToken must have the prefix \"xapp-\".")
	}

	if !strings.HasPrefix(botToken, "xoxb-") {
		return nil, errors.New("botToken must have the prefix \"xoxb-\".")
	}

	api := slack.New(
		botToken,
		slack.OptionDebug(debug),
		slack.OptionLog(loggingAdapter{logger: log.StandardLogger()}),
		slack.OptionAppLevelToken(appToken),
	)

	slackClient := socketmode.New(
		api,
		socketmode.OptionDebug(debug),
		socketmode.OptionLog(loggingAdapter{logger: log.StandardLogger()}),
	)

	ctx, cancel := context.WithCancel(context.Background())
	firestoreClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("firestore.NewClient: %w", err)
	}

	return &bot{
		ctx:        ctx,
		cancel:     cancel,
		firestore:  firestoreClient,
		slack:      slackClient,
		channelID:  channelID,
		collection: collection,
	}, nil
}

func (b *bot) run() error {
	go b.handleSlackEvents()
	go b.handleFirestoreSnapshots()
	return b.slack.RunContext(b.ctx)
}

func (b *bot) handleFirestoreSnapshots() {
	it := b.firestore.Collection(b.collection).Snapshots(b.ctx)
	// skip first snapshot: it contains all events in the collection
	_ , _ = it.Next()
	for {
		snap, err := it.Next()
		// DeadlineExceeded will be returned when ctx is cancelled.
		if status.Code(err) == codes.DeadlineExceeded {
			return
		}
		if err != nil {
			log.Errorf("Snapshots.Next: %v", err)
			// force graceful cleanup
			b.cancel()
			return
		}
		if snap == nil {
			continue
		}

		for _, change := range snap.Changes {
			b.postChange(change)
		}
	}
}

func (b *bot) postChange(change firestore.DocumentChange) {
	var color, msg string
	var report issue.ReportDocument
	err := change.Doc.DataTo(&report)
	if err != nil {
		log.Errorf("unmarshal issue report: %v", err)
		return
	}
	if strings.HasSuffix(report.ID(), ".swp") {
		log.Debugf("skipping swap file: %s", report.ID())
		return
	}
	switch change.Kind {
	case firestore.DocumentAdded:
		msg = fmt.Sprintf("Issue %s created", report.ID())
		color = "#CAB3E8"
	case firestore.DocumentModified:
		msg = fmt.Sprintf("Issue %s modified", report.ID())
		color = "#DF5D32"
	case firestore.DocumentRemoved:
		// TODO documents are never deleted, so we should
		// handle an issue going from not closed to closed,
		// via update message.
		msg = fmt.Sprintf("Issue %s deleted", report.ID())
		color = "#23272D"
	}
	rep := report.Report
	var typeOfIssue string
	if _, ok := rep.Metadata["bug"]; ok {
		typeOfIssue = "bug"
	} else if _, ok := rep.Metadata["feature"]; ok {
		typeOfIssue = "feature"
	} else {
		typeOfIssue = "unspecified"
	}
	attachment := slack.Attachment{
		MarkdownIn: []string{"text"},
		// TODO add updated_by: field to correctly set this
		// TODO lookup via email userID
		AuthorName: rep.Author,
		Color:      color,
		Title:      report.ID(),
		Text:       rep.Subject,
		Fields: []slack.AttachmentField{
			{Title: "Package", Value: rep.Package, Short: true},
			{Title: "Version", Value: rep.Version, Short: true},
			{Title: "CreatedAt", Value: rep.CreatedAt.Format(time.RFC3339), Short: true},
			{Title: "UpdatedAt", Value: rep.UpdatedAt.Format(time.RFC3339), Short: true},
			{Title: "Closed", Value: fmt.Sprintf("%t", rep.Closed), Short: true},
			{Title: "ClosedAt", Value: rep.ClosedAt.Format(time.RFC3339), Short: true},
			{Title: "Assignee", Value: rep.Metadata["assignee"], Short: true},
			{Title: "Type", Value: typeOfIssue, Short: true},
			{Title: "Notes", Value: rep.Notes, Short: false},
		},
	}

	_, _, err = b.slack.PostMessage(
		b.channelID,
		slack.MsgOptionText(msg, false),
		slack.MsgOptionAttachments(attachment),
		// Add this if you want that the bot would post message as a user
		// otherwise it will send response using the default slackbot
		slack.MsgOptionAsUser(true),
	)
	if err != nil {
		log.Errorf("failed posting message: %v", err)
	}
}

func (b *bot) handleSlackEvents() {
	for evt := range b.slack.Events {
		switch evt.Type {
		case socketmode.EventTypeConnecting:
			log.Info("connecting to Slack with Socket Mode...")
		case socketmode.EventTypeConnectionError:
			log.Info("connection failed. Retrying later...")
		case socketmode.EventTypeConnected:
			log.Info("connected to Slack with Socket Mode.")
		case socketmode.EventTypeEventsAPI:
			eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				log.Debugf("event ignored %+v", evt)
				continue
			}

			log.Debugf("event received: %+v", eventsAPIEvent)

			b.slack.Ack(*evt.Request)

			switch eventsAPIEvent.Type {
			case slackevents.CallbackEvent:
				innerEvent := eventsAPIEvent.InnerEvent
				switch ev := innerEvent.Data.(type) {
				case *slackevents.AppMentionEvent:
					_, _, err := b.slack.PostMessage(ev.Channel, slack.MsgOptionText("Yes, hello.", false))
					if err != nil {
						log.Errorf("failed posting message: %v", err)
					}
				case *slackevents.MemberJoinedChannelEvent:
					log.Debugf("user %q joined to channel %q", ev.User, ev.Channel)
				}
			default:
				b.slack.Debugf("unsupported Events API event received")
				log.Warn("unsupported Events API event received")
			}
		case socketmode.EventTypeInteractive:
			callback, ok := evt.Data.(slack.InteractionCallback)
			if !ok {
				log.Debugf("interaction ignored %+v", evt)
				continue
			}

			log.Debugf("interaction received: %+v", callback)

			var payload interface{}
			switch callback.Type {
			case slack.InteractionTypeBlockActions:
				// See https://api.slack.com/apis/connections/socket-implement#button
				b.slack.Debugf("button clicked!")
			case slack.InteractionTypeShortcut:
			case slack.InteractionTypeViewSubmission:
				// See https://api.slack.com/apis/connections/socket-implement#modal
			case slack.InteractionTypeDialogSubmission:
			default:

			}
			b.slack.Ack(*evt.Request, payload)

		case socketmode.EventTypeSlashCommand:
			cmd, ok := evt.Data.(slack.SlashCommand)
			if !ok {
				log.Debugf("slash command ignored %+v", evt)
				continue
			}

			log.Debugf("slash command received: %+v", cmd)

			payload := map[string]interface{}{
				"blocks": []slack.Block{
					slack.NewSectionBlock(
						&slack.TextBlockObject{
							Type: slack.MarkdownType,
							Text: "foo",
						},
						nil,
						slack.NewAccessory(
							slack.NewButtonBlockElement(
								"",
								"somevalue",
								&slack.TextBlockObject{
									Type: slack.PlainTextType,
									Text: "bar",
								},
							),
						),
					),
				},
			}

			b.slack.Ack(*evt.Request, payload)
		case socketmode.EventTypeHello:
			log.Debugf("connection with the server has been correctly opened")
		default:
			log.Warnf("unexpected payload received: %s", evt.Type)
		}
	}
}

func (b *bot) Close() error {
	b.cancel()
	return b.firestore.Close()
}
