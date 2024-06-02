package issue

import (
	"context"
	"io"
	"strconv"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/iterator"
)

func newTestingDocumentTracker() (m Tracker, svc document.Service) {
	svc = document.NewInMemoryService()
	m = NewDocumentTracker(svc)
	return
}

func TestIsInternal(t *testing.T) {
	assert.True(t, IsInternalLabel(ReportMetadataIDField))
}

func listPackageReports(m Tracker, pkg string,
	filters map[string]string) ([]Report, error) {
	iter, err := m.ListPackageReports(context.Background(), pkg, filters)
	if err != nil {
		return nil, err
	}
	return iterator.ToSlice(iter)
}

func listVersionReports(m Tracker, pkg, ver string,
	filters map[string]string) ([]Report, error) {
	iter, err := m.ListVersionReports(context.Background(), pkg, ver, filters)
	if err != nil {
		return nil, err
	}
	return iterator.ToSlice(iter)
}

func TestDocumentTracker(t *testing.T) {
	t.Run("issue workflow", func(t *testing.T) {
		m, _ := newTestingDocumentTracker()

		ll := log.New()
		ll.Out = io.Discard
		for i := 0; i < 10; i++ {
			report := Report{
				Author:    "test.capturePanic",
				Subject:   "bummers",
				Package:   "pkg",
				Version:   "v1.0.0",
				CreatedAt: time.Now(),
			}
			report.Metadata = make(map[string]string)
			report.Metadata["i"] = strconv.Itoa(i)
			id, err := m.CreateReport(context.Background(), report)
			require.NoError(t, err)
			assert.NotZero(t, id)
		}

		// should not be returned as its closed
		closedReport := Report{
			Author:    "test.capturePanic",
			Subject:   "bummers",
			Package:   "pkg",
			Version:   "v1.0.0",
			CreatedAt: time.Now(),
		}
		closedReport.Metadata = make(map[string]string)
		closedReport.Metadata["i"] = "closing"
		_, err := m.CreateReport(context.Background(), closedReport)
		require.NoError(t, err)

		reports, err := listPackageReports(m, "pkg", map[string]string{"Metadata.i": "closing"})
		require.NoError(t, err)
		require.Len(t, reports, 1)

		closedIssueID := reports[0].Metadata[ReportMetadataIDField]
		err = m.CloseReport(context.Background(), closedIssueID)
		require.NoError(t, err)

		reports, err = listVersionReports(m, "pkg", "UNKNOWNVERSION", nil)
		require.NoError(t, err)
		require.Len(t, reports, 0)

		assertReport := func(report Report) {
			assert.WithinDuration(t, report.CreatedAt, time.Now(), 1*time.Minute)
			assert.NotZero(t, report.Build)
			assert.Equal(t, "pkg", report.Package)
			assert.Equal(t, "v1.0.0", report.Version)
		}

		reports, err = listVersionReports(m, "pkg", "v1.0.0", map[string]string{"Metadata.i": "1"})
		require.NoError(t, err)
		require.Len(t, reports, 1)
		assertReport(reports[0])

		reports, err = listPackageReports(m, "pkg", map[string]string{"Metadata.i": "1"})
		require.NoError(t, err)
		require.Len(t, reports, 1)
		assertReport(reports[0])

		require.NotNil(t, reports[0].Metadata)
		id, ok := reports[0].Metadata[ReportMetadataIDField]
		require.True(t, ok)
		r, err := m.GetReport(context.Background(), id)
		require.NoError(t, err)
		assertReport(r)

		err = m.DeleteReport(context.Background(), id)
		require.NoError(t, err)

		_, err = m.GetReport(context.Background(), id)
		require.Error(t, err)
		assert.Equal(t, document.ErrNotFound, err)

		// test that we are able to list closed reports
		reports, err = listPackageReports(m, "pkg", map[string]string{"Closed": "true"})
		require.NoError(t, err)
		require.Len(t, reports, 1)
		assertReport(reports[0])
		assert.True(t, reports[0].Closed)

		update := Report{
			Author:    "test.capturePanic",
			Subject:   "reopened",
			Package:   "pkg",
			Version:   "v1.0.0",
			Closed:    false,
			CreatedAt: time.Now(),
		}

		require.NoError(t, m.UpdateReport(context.Background(), closedIssueID, update))
		reports, err = listPackageReports(m, "pkg", map[string]string{"Subject": "reopened"})
		require.NoError(t, err)
		require.Len(t, reports, 1)
		assertReport(reports[0])
		assert.False(t, reports[0].Closed)
	})

	t.Run("should be able to create issues past max number of retries", func(t *testing.T) {
		svc := document.NewInMemoryService()
		for i := 0; i < int(10+1); i++ {
			m := NewDocumentTracker(svc)
			report := Report{
				Author:  "test2",
				Subject: "bummers",
				Package: "pkg",
			}
			id, err := m.CreateReport(context.Background(), report)
			require.NoError(t, err)
			assert.NotZero(t, id)
		}
	})
}
