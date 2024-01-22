package worker

import (
	"context"
	"fmt"
	log "github.com/gophish/gophish/logger"
	"testing"
	"time"

	"github.com/gophish/gophish/config"
	"github.com/gophish/gophish/mailer"
	"github.com/gophish/gophish/models"
)

type logMailer struct {
	queue chan []mailer.Mail
}

func (m *logMailer) Start(_ context.Context) {}

func (m *logMailer) Queue(ms []mailer.Mail) {
	m.queue <- ms
}

// testContext is context to cover API related functions
type testContext struct {
	config *config.Config
}

func setupTest(t *testing.T) *testContext {
	conf := &config.Config{
		DBName:         "sqlite3",
		DBPath:         ":memory:",
		MigrationsPath: "../db/db_sqlite3/migrations/",
	}
	err := models.Setup(conf)
	if err != nil {
		t.Fatalf("Failed creating database: %v", err)
	}
	ctx := &testContext{}
	ctx.config = conf
	createTestData(t, ctx)
	return ctx
}

func createTestData(_ *testing.T, ctx *testContext) {
	ctx.config.TestFlag = true
	// Add a group
	group := models.Group{Name: "Test Group"}
	for i := 0; i < 10; i++ {
		group.Targets = append(group.Targets, models.Target{
			BaseRecipient: models.BaseRecipient{
				Email:     fmt.Sprintf("test%d@example.com", i),
				FirstName: "First",
				LastName:  "Example"}})
	}
	group.UserId = 1
	err := models.PostGroup(&group)
	if err != nil {
		log.Error(err)
	}

	// Add a template
	template := models.Template{Name: "Test Template"}
	template.Subject = "Test subject"
	template.Text = "Text text"
	template.HTML = "<html>Test</html>"
	template.UserId = 1
	err = models.PostTemplate(&template)
	if err != nil {
		log.Error(err)
	}

	// Add a landing page
	p := models.Page{Name: "Test Page"}
	p.HTML = "<html>Test</html>"
	p.UserId = 1
	err = models.PostPage(&p)
	if err != nil {
		log.Error(err)
	}

	// Add a sending profile
	smtp := models.SMTP{Name: "Test Page"}
	smtp.UserId = 1
	smtp.Host = "example.com"
	smtp.FromAddress = "test@test.com"
	err = models.PostSMTP(&smtp)
	if err != nil {
		log.Error(err)
	}
}

func setupCampaign(id int) (*models.Campaign, error) {
	// Setup and "launch" our campaign
	// Set the status such that no emails are attempted
	c := models.Campaign{Name: fmt.Sprintf("Test campaign - %d", id)}
	c.UserId = 1
	template, err := models.GetTemplate(1, 1)
	if err != nil {
		return nil, err
	}
	c.Template = template

	page, err := models.GetPage(1, 1)
	if err != nil {
		return nil, err
	}
	c.Page = page

	smtp, err := models.GetSMTP(1, 1)
	if err != nil {
		return nil, err
	}
	c.SMTP = smtp

	group, err := models.GetGroup(1, 1)
	if err != nil {
		return nil, err
	}
	c.Groups = []models.Group{group}
	err = models.PostCampaign(&c, c.UserId)
	if err != nil {
		return nil, err
	}
	err = c.UpdateStatus(models.CampaignEmailsSent)
	return &c, err
}

func TestMailLogGrouping(t *testing.T) {
	setupTest(t)

	// Create the campaigns and unlock the mail logs so that they're picked up
	// by the worker
	for i := 0; i < 10; i++ {
		campaign, err := setupCampaign(i)
		if err != nil {
			t.Fatalf("error creating campaign: %v", err)
		}
		ms, err := models.GetMailLogsByCampaign(campaign.Id)
		if err != nil {
			t.Fatalf("error getting maillogs for campaign: %v", err)
		}
		for _, m := range ms {
			err := m.Unlock()
			if err != nil {
				log.Error(err)
			}
		}
	}

	lm := &logMailer{queue: make(chan []mailer.Mail)}
	worker := &DefaultWorker{}
	worker.mailer = lm

	// Trigger the worker, generating the mail logs and sending them to the
	// mailer
	err := worker.processCampaigns(time.Now())
	if err != nil {
		log.Error(err)
	}

	// Verify that each slice of mail logs received belong to the same campaign
	for i := 0; i < 10; i++ {
		ms := <-lm.queue
		maillog, ok := ms[0].(*models.MailLog)
		if !ok {
			t.Fatalf("unable to cast mail to models.MailLog")
		}
		expected := maillog.CampaignId
		for _, m := range ms {
			maillog, ok = m.(*models.MailLog)
			if !ok {
				t.Fatalf("unable to cast mail to models.MailLog")
			}
			got := maillog.CampaignId
			if got != expected {
				t.Fatalf("unexpected campaign ID received for maillog: got %d expected %d", got, expected)
			}
		}
	}
}
