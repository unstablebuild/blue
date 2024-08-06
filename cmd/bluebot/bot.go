// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	log "github.com/sirupsen/logrus"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
	"github.com/unstablebuild/blue/issue"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	keyAssignee   = "assignee"
	keyMilestones = "milestones"
)

type bot struct {
	ctx        context.Context
	cancel     func()
	hostname   string
	slack      *socketmode.Client
	firestore  *firestore.Client
	channelID  string
	collection string
	issues     map[string]issue.ReportDocument
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

	name, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("os hostname: %w", err)
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
		hostname:   name,
		ctx:        ctx,
		cancel:     cancel,
		issues:     make(map[string]issue.ReportDocument),
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
	defer b.cancel()

	it := b.firestore.Collection(b.collection).Snapshots(b.ctx)
	// process first snapshot separately: it contains all documents in the collection
	snapshot, err := it.Next()
	if err != nil {
		log.Errorf("fetch first snapshot: %v", err)
		return
	}
	if err := b.processFirstSnapshot(snapshot); err != nil {
		log.Errorf("process first snapshot: %v", err)
		return
	}

	for {
		// this blocks until next change is available
		snap, err := it.Next()
		if code := status.Code(err); code == codes.Canceled || code == codes.DeadlineExceeded {
			return
		}
		if err != nil {
			log.Errorf("Snapshots.Next: %v", err)
			// force graceful cleanup
			return
		}
		if snap == nil {
			continue
		}
		for _, change := range snap.Changes {
			b.processSnapshotChange(change)
		}
	}
}

func (b *bot) processFirstSnapshot(snap *firestore.QuerySnapshot) error {
	var count int
	for _, change := range snap.Changes {
		doc, ok := b.unmarshalDocumentFromChange(&change)
		if !ok {
			continue
		}
		count++
		switch change.Kind {
		case firestore.DocumentAdded:
			b.issues[doc.ID()] = doc
		case firestore.DocumentModified:
			// an update sneaked while snapshot was being created?
			// process it like below
			b.handleDocumentModified(doc)
		case firestore.DocumentRemoved:
			// no-op
		}
	}

	log.Debugf("processed first snapshot: cached %d issues, total in snapshot: %d",
		count, len(snap.Changes))
	return nil
}

func (b *bot) processSnapshotChange(change firestore.DocumentChange) {
	doc, ok := b.unmarshalDocumentFromChange(&change)
	if !ok {
		return
	}
	switch change.Kind {
	case firestore.DocumentAdded:
		// add to history for modified comparisons
		b.issues[doc.ID()] = doc
		const color = "#CAB3E8"
		msg := fmt.Sprintf("Issue %s created 🔧", doc.ID())
		if _, ok := doc.Report.Metadata["bug"]; ok {
			msg = fmt.Sprintf("Bug %s created 🐞", doc.ID())
		} else if _, ok := doc.Report.Metadata["feature"]; ok {
			msg = fmt.Sprintf("Feature request %s created 🚀", doc.ID())
		}
		b.postSlackMessage(doc, color, msg, doc.Report.Author)

	case firestore.DocumentModified:
		b.handleDocumentModified(doc)

	case firestore.DocumentRemoved:
		// delete from history
		delete(b.issues, doc.ID())
		// issues are never deleted; just marked as closed
		log.Debugf("skipping document removed: %s", doc.ID())
	}
}

func (b *bot) handleDocumentModified(doc issue.ReportDocument) {
	color := "#CA9E69"
	msg := fmt.Sprintf("Issue %s modified", doc.ID())
	prev, ok := b.issues[doc.ID()]
	if ok {
		if !prev.Report.Closed && doc.Report.Closed {
			msg = fmt.Sprintf("Issue %s closed 🎉", doc.ID())
			color = "#009F4D"
		} else if prev.Report.Closed && !doc.Report.Closed {
			msg = fmt.Sprintf("Issue %s re-opened 🧐", doc.ID())
			color = "#A53C3C"
		} else if assignee := doc.Report.Metadata[keyAssignee]; prev.Report.Metadata[keyAssignee] == "" && assignee != "" {
			msg = fmt.Sprintf("Issue %s assigned to %s", doc.ID(), assignee)
		} else if assignee := doc.Report.Metadata[keyAssignee]; prev.Report.Metadata[keyAssignee] != "" && assignee == "" {
			msg = fmt.Sprintf("Issue %s asignee removed (was %s)", doc.ID(), prev.Report.Metadata[keyAssignee])
		} else if assignee := doc.Report.Metadata[keyAssignee]; prev.Report.Metadata[keyAssignee] != assignee {
			msg = fmt.Sprintf("Issue %s asignee changed from %s to %s", doc.ID(),
				prev.Report.Metadata[keyAssignee], assignee)
		} else if prev.Report.Subject != doc.Report.Subject {
			msg = fmt.Sprintf("Issue %s subject changed", doc.ID())
		} else if prev.Report.Notes != doc.Report.Notes {
			msg = fmt.Sprintf("Issue %s notes changed", doc.ID())
		} else if prev.Report.Package != doc.Report.Package {
			msg = fmt.Sprintf("Issue %s package changed", doc.ID())
		} else if prev.Report.Version != doc.Report.Version {
			msg = fmt.Sprintf("Issue %s version changed", doc.ID())
		} else if prev.Report.Author != doc.Report.Author {
			msg = fmt.Sprintf("Issue %s author changed", doc.ID())
		} else if milestones := doc.Report.Metadata[keyMilestones]; prev.Report.Metadata[keyMilestones] != milestones {
			msg = fmt.Sprintf("Issue %s milestones changed from '%s' to %s", doc.ID(),
				prev.Report.Metadata[keyMilestones], milestones)
		} else {
			return
		}
	}
	b.postSlackMessage(doc, color, msg, doc.Report.UpdatedBy)

	// update issue in cache
	b.issues[doc.ID()] = doc
}

func (b *bot) postSlackMessage(doc issue.ReportDocument, color, msg, author string) {
	rep := doc.Report
	var typeOfIssue string
	if _, ok := rep.Metadata["bug"]; ok {
		typeOfIssue = "bug"
	} else if _, ok := rep.Metadata["feature"]; ok {
		typeOfIssue = "feature"
	} else {
		typeOfIssue = "unspecified"
	}

	author = b.resolveUpdatedBy(author)
	assignee := b.resolveAssignee(rep.Metadata[keyAssignee])

	attachment := slack.Attachment{
		Color:      color,
		MarkdownIn: []string{"text"},
		Title:      doc.ID(),
		Text:       rep.Subject,
		Fields: []slack.AttachmentField{
			{Title: "Package", Value: rep.Package, Short: true},
			{Title: "Version", Value: rep.Version, Short: true},
			{Title: "CreatedAt", Value: rep.CreatedAt.Format(time.RFC3339), Short: true},
			{Title: "Closed", Value: fmt.Sprintf("%t", rep.Closed), Short: true},
			{Title: "Assignee", Value: assignee, Short: true},
			{Title: "UpdatedBy", Value: author, Short: true},
			{Title: "Type", Value: typeOfIssue, Short: true},
			{Title: "Notes", Value: rep.Notes, Short: false},
		},
		Footer: fmt.Sprintf("Bluebot %s running on %q", Version, b.hostname),
	}

	_, _, err := b.slack.PostMessage(
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

func (b *bot) unmarshalDocumentFromChange(change *firestore.DocumentChange) (
	ret issue.ReportDocument, ok bool,
) {
	if strings.HasSuffix(change.Doc.Ref.ID, ".swp") {
		log.Debugf("skipping swap file: %s", change.Doc.Ref.ID)
		return
	}
	err := change.Doc.DataTo(&ret)
	if err != nil {
		log.Warnf("skipping issue: unmarshal issue report: %v", err)
		return
	}
	ok = true
	return
}

func (b *bot) resolveUpdatedBy(author string) string {
	user := b.resolveSlackUser(author)
	if user == nil {
		return author
	}
	// do not notify author
	return fmt.Sprintf("@%s", user.Profile.DisplayName)
}

func (b *bot) resolveAssignee(author string) string {
	user := b.resolveSlackUser(author)
	if user == nil {
		return author
	}
	// notify assignee
	return fmt.Sprintf("<@%s>", user.ID)
}

// best effort resolve author as a slack username
func (b *bot) resolveSlackUser(author string) *slack.User {
	if author == "" {
		return nil
	}

	user, err := b.slack.GetUserByEmail(author)
	if err != nil {
		log.Warnf("slack: get user by email: %v", err)
		return nil
	}
	return user
}

func (b *bot) Close() error {
	b.cancel()
	return b.firestore.Close()
}
