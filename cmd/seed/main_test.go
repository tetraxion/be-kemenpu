package main

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"irrigation-server/internal/features/ingest"
)

// stubService records what the seeder asked for.
type stubService struct {
	statusAfterRun string
	followUpErr    error
	ran, followed  int
	status         string
}

func (s *stubService) Run(context.Context) { s.ran++; s.status = s.statusAfterRun }
func (s *stubService) Status() string      { return s.status }
func (s *stubService) MarkFailed()         { s.status = ingest.StatusFailed }
func (s *stubService) ImportFollowUp(context.Context) (ingest.Summary, error) {
	s.followed++
	return ingest.Summary{Added: 1}, s.followUpErr
}
func (s *stubService) ImportFollowUpFrom(context.Context, io.Reader) (ingest.Summary, error) {
	return ingest.Summary{}, nil
}

func TestSeedImportsTheDataFolderOnly(t *testing.T) {
	svc := &stubService{statusAfterRun: ingest.StatusReady}

	require.NoError(t, seed(context.Background(), svc, false))

	assert.Equal(t, 1, svc.ran)
	assert.Zero(t, svc.followed, "the follow-up file is imported only when asked")
}

func TestSeedWithFollowUp(t *testing.T) {
	svc := &stubService{statusAfterRun: ingest.StatusReady}

	require.NoError(t, seed(context.Background(), svc, true))

	assert.Equal(t, 1, svc.followed)
}

func TestSeedFailsWhenTheImportFails(t *testing.T) {
	svc := &stubService{statusAfterRun: ingest.StatusFailed}

	err := seed(context.Background(), svc, true)

	assert.Error(t, err, "a failed import must give a non-zero exit code")
	assert.Zero(t, svc.followed, "the follow-up import is not attempted on top of a failed import")
}

func TestSeedReportsAFollowUpError(t *testing.T) {
	svc := &stubService{statusAfterRun: ingest.StatusReady, followUpErr: ingest.ErrFollowUpMissing}

	err := seed(context.Background(), svc, true)

	assert.True(t, errors.Is(err, ingest.ErrFollowUpMissing))
}
