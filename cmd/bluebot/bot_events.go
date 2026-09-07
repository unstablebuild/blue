// Copyright 2018-2026 Unstable Build, LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

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
