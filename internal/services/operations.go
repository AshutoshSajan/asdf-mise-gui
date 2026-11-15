package services

import (
	"fmt"
	"os"

	"github.com/AshutoshSajan/asdf-mise-gui/internal/core"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/ops"
	"github.com/AshutoshSajan/asdf-mise-gui/internal/settings"
)

// OperationService exposes the FIFO queue to the UI.
type OperationService struct {
	rt *core.Runtime
}

func NewOperationService(rt *core.Runtime) *OperationService {
	return &OperationService{rt: rt}
}

func (s *OperationService) List() []ops.Operation {
	return s.rt.Queue().List()
}

func (s *OperationService) Get(id string) (ops.Detail, error) {
	return s.rt.Queue().Get(id)
}

func (s *OperationService) Cancel(id string) (ops.Operation, error) {
	return s.rt.Queue().Cancel(id)
}

func (s *OperationService) ClearFinished() int {
	return s.rt.Queue().ClearFinished()
}

func (s *OperationService) RunningCount() int {
	return s.rt.Queue().RunningCount()
}

func (s *OperationService) OpenLog(id string) error {
	detail, err := s.rt.Queue().Get(id)
	if err != nil {
		return err
	}
	if _, err := os.Stat(detail.Operation.LogPath); err != nil {
		return fmt.Errorf("log file is not available yet")
	}
	return openInFileManager(detail.Operation.LogPath, true)
}

func (s *OperationService) OpenLogsDir() error {
	dir, err := settings.LogsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return openInFileManager(dir, false)
}
