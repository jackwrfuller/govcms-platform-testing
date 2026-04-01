package orchestrator

import (
	"bytes"
	"html/template"
	"log"

	"github.com/govcms/platform-testing/internal/models"
	"github.com/govcms/platform-testing/internal/sse"
)

type Orchestrator struct {
	Store    *models.Store
	Broker   *sse.Broker
	Partials *template.Template
}

func New(store *models.Store, broker *sse.Broker, partials *template.Template) *Orchestrator {
	return &Orchestrator{
		Store:    store,
		Broker:   broker,
		Partials: partials,
	}
}

func (o *Orchestrator) StartRun() (*models.TestRun, error) {
	sites, err := o.Store.ListActiveSites()
	if err != nil {
		return nil, err
	}

	if len(sites) == 0 {
		return nil, nil
	}

	run, err := o.Store.CreateRun(len(sites))
	if err != nil {
		return nil, err
	}

	for _, site := range sites {
		if err := o.Store.CreateResult(run.ID, site.ID); err != nil {
			log.Printf("Error creating result for site %s: %v", site.Name, err)
		}
	}

	return run, nil
}

// PublishProgress publishes a run progress SSE event.
func (o *Orchestrator) PublishProgress(runID int) {
	run, err := o.Store.GetRun(runID)
	if err != nil {
		log.Printf("Error fetching run %d for SSE: %v", runID, err)
		return
	}

	var buf bytes.Buffer
	if err := o.Partials.ExecuteTemplate(&buf, "run_progress.html", run); err != nil {
		log.Printf("Error rendering progress partial: %v", err)
		return
	}

	o.Broker.Publish(sse.Event{
		RunID: runID,
		Type:  "RunProgress",
		Data:  buf.String(),
	})
}

// PublishSiteComplete publishes when a site test finishes.
func (o *Orchestrator) PublishSiteComplete(resultID int) {
	result, err := o.Store.GetResult(resultID)
	if err != nil {
		log.Printf("Error fetching result %d for SSE: %v", resultID, err)
		return
	}

	var buf bytes.Buffer
	if err := o.Partials.ExecuteTemplate(&buf, "result_card_oob.html", result); err != nil {
		log.Printf("Error rendering result card: %v", err)
		return
	}

	o.Broker.Publish(sse.Event{
		RunID: result.RunID,
		Type:  "SiteComplete",
		Data:  buf.String(),
	})
}

// PublishRunComplete publishes when all sites in a run are done.
func (o *Orchestrator) PublishRunComplete(runID int) {
	run, err := o.Store.GetRun(runID)
	if err != nil {
		log.Printf("Error fetching run %d for SSE: %v", runID, err)
		return
	}

	var buf bytes.Buffer
	if err := o.Partials.ExecuteTemplate(&buf, "run_progress.html", run); err != nil {
		log.Printf("Error rendering run complete partial: %v", err)
		return
	}

	o.Broker.Publish(sse.Event{
		RunID: runID,
		Type:  "RunComplete",
		Data:  buf.String(),
	})
}
