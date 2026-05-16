package app

import (
	"context"
	"errors"
	"io"
	"testing"

	"vpnkit/internal/service"
)

type fakeReloader struct {
	reloadErr error
	called    int
}

func (f *fakeReloader) ReloadConfig(_ context.Context, _ string) error {
	f.called++
	return f.reloadErr
}

type fakeService struct {
	restartCalls int
	restartErr   error
}

func (f *fakeService) Mode() service.Mode                                  { return service.ModePID }
func (f *fakeService) Install(_ context.Context) error                     { return nil }
func (f *fakeService) Uninstall(_ context.Context) error                   { return nil }
func (f *fakeService) Start(_ context.Context) error                       { return nil }
func (f *fakeService) Stop(_ context.Context) error                        { return nil }
func (f *fakeService) Restart(_ context.Context) error {
	f.restartCalls++
	return f.restartErr
}
func (f *fakeService) Status(_ context.Context) (service.Status, error)    { return service.Status{}, nil }
func (f *fakeService) Logs(_ context.Context, _ bool) (io.ReadCloser, error) {
	return nil, nil
}

func TestApplyConfigReloadSucceeds(t *testing.T) {
	r := &fakeReloader{reloadErr: nil}
	s := &fakeService{}
	if err := applyConfig(context.Background(), r, s); err != nil {
		t.Fatal(err)
	}
	if r.called != 1 {
		t.Errorf("reload called %d, want 1", r.called)
	}
	if s.restartCalls != 0 {
		t.Errorf("restart called %d, want 0 (reload should have succeeded)", s.restartCalls)
	}
}

func TestApplyConfigFallsBackToRestartOnReloadFailure(t *testing.T) {
	r := &fakeReloader{reloadErr: errors.New("401 Unauthorized")}
	s := &fakeService{}
	if err := applyConfig(context.Background(), r, s); err != nil {
		t.Fatal(err)
	}
	if r.called != 1 {
		t.Errorf("reload called %d, want 1", r.called)
	}
	if s.restartCalls != 1 {
		t.Errorf("restart called %d, want 1 (fallback expected)", s.restartCalls)
	}
}

func TestApplyConfigReturnsRestartErrorWhenBothFail(t *testing.T) {
	r := &fakeReloader{reloadErr: errors.New("401")}
	s := &fakeService{restartErr: errors.New("systemctl failed")}
	err := applyConfig(context.Background(), r, s)
	if err == nil {
		t.Fatal("expected error when restart also fails")
	}
}

func TestApplyConfigNoopWhenServiceNil(t *testing.T) {
	r := &fakeReloader{reloadErr: errors.New("401")}
	// nil service → caller has no way to restart; should return the reload error.
	err := applyConfig(context.Background(), r, nil)
	if err == nil {
		t.Fatal("expected error when no svc available")
	}
}
