package trash

import "github.com/wubinstu/adrm/internal/model"

// Bin returns every item currently in the trash bin.
func (e *Engine) Bin() ([]model.Item, error) { return e.St.ListBin() }

// Reflog returns the full operation history (oldest first).
func (e *Engine) Reflog() ([]model.Reflog, error) { return e.St.ListReflog() }

// ResetDB clears both tables (bin and reflog) and records the reset.
func (e *Engine) ResetDB() error {
	return e.St.Reset("adrm db --reset")
}
