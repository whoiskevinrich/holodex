package maintenance

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"holodex/internal/repo"
)

// HeldStore is the held_originals journal (implemented by *repo.Repo).
type HeldStore interface {
	GetHeldOriginal(ctx context.Context, videoID int64) (*repo.HeldOriginal, error)
	ListHeldOriginals(ctx context.Context, state string) ([]repo.HeldOriginal, error)
	BeginSwap(ctx context.Context, videoID int64, heldPath, op string, now, expires time.Time) error
	SetHeldState(ctx context.Context, videoID int64, state string) error
	DeleteHeldOriginal(ctx context.Context, videoID int64) error
}

// ErrHeldMismatch: the journal's held_path is not the one derived from the video's current
// path, or a held original exists on disk with no journal row. Either way the file system and
// the journal disagree, so nothing is renamed.
var ErrHeldMismatch = errors.New("maintenance: held original does not match the video's file")

// Swapper performs and recovers the two-rename swap (file-maintenance.md, "The swap is two
// same-directory renames, journaled in held_originals").
type Swapper struct {
	store  HeldStore
	hold   time.Duration
	now    func() time.Time
	rename func(oldpath, newpath string) error // os.Rename; a test injects failures
	remove func(name string) error             // os.Remove
}

// NewSwapper returns a Swapper holding originals for hold.
func NewSwapper(store HeldStore, hold time.Duration) *Swapper {
	return &Swapper{store: store, hold: hold, now: time.Now, rename: os.Rename, remove: os.Remove}
}

// HeldPath is where path's original is held. Always derived, never stored from a request.
func HeldPath(path string) string { return path + repo.HeldPathSuffix }

// NewPath is where a verified candidate for path waits for the swap.
func NewPath(path string) string { return path + ".holodex-new" }

// Swap puts the verified candidate NewPath(path) in place of path. The first operation on a
// file holds the original at HeldPath(path); a later one while that hold stands replaces the
// current file directly, so a restore always returns the file to how it was before maintenance
// began, and the window does not restart.
func (s *Swapper) Swap(ctx context.Context, videoID int64, path, op string) error {
	held, newp := HeldPath(path), NewPath(path)
	for _, name := range []string{held, newp} {
		if err := refuseSymlink(name); err != nil {
			return err
		}
	}
	h, err := s.store.GetHeldOriginal(ctx, videoID)
	if err != nil {
		return err
	}
	if h != nil {
		if h.State != repo.HeldHeld || h.HeldPath != held {
			return fmt.Errorf("%w: journal says %s at %s", ErrHeldMismatch, h.State, h.HeldPath)
		}
		return s.rename(newp, path)
	}
	if has, err := exists(held); err != nil || has {
		return fmt.Errorf("%w: %s exists with no journal row (%v)", ErrHeldMismatch, held, err)
	}

	now := s.now()
	if err := s.store.BeginSwap(ctx, videoID, held, op, now, now.Add(s.hold)); err != nil {
		return err
	}
	if err := s.rename(path, held); err != nil {
		_ = s.store.DeleteHeldOriginal(ctx, videoID)
		return fmt.Errorf("hold original: %w", err)
	}
	if err := s.rename(newp, path); err != nil {
		if rbErr := s.rename(held, path); rbErr != nil {
			// Leave the row in swapping: boot recovery finishes or undoes it.
			return fmt.Errorf("place new file: %w (rollback failed: %v)", err, rbErr)
		}
		_ = s.store.DeleteHeldOriginal(ctx, videoID)
		return fmt.Errorf("place new file: %w", err)
	}
	return s.store.SetHeldState(ctx, videoID, repo.HeldHeld)
}

// Restore puts the held original back in place of path and ends the hold.
func (s *Swapper) Restore(ctx context.Context, videoID int64, path string) error {
	held, err := s.heldFor(ctx, videoID, path)
	if err != nil {
		return err
	}
	if err := s.store.SetHeldState(ctx, videoID, repo.HeldRestoring); err != nil {
		return err
	}
	if err := s.rename(held, path); err != nil {
		// Nothing moved: the hold still stands, so put the row back and let Retry work.
		_ = s.store.SetHeldState(ctx, videoID, repo.HeldHeld)
		return fmt.Errorf("restore original: %w", err)
	}
	return s.store.DeleteHeldOriginal(ctx, videoID)
}

// Discard deletes the held original ("Keep change", expiry) and ends the hold.
func (s *Swapper) Discard(ctx context.Context, videoID int64, path string) error {
	held, err := s.heldFor(ctx, videoID, path)
	if err != nil {
		return err
	}
	if err := s.store.SetHeldState(ctx, videoID, repo.HeldDiscarding); err != nil {
		return err
	}
	if err := s.remove(held); err != nil && !errors.Is(err, fs.ErrNotExist) {
		// The file is still there: keep the hold standing so the expiry pass retries it.
		_ = s.store.SetHeldState(ctx, videoID, repo.HeldHeld)
		return fmt.Errorf("discard original: %w", err)
	}
	return s.store.DeleteHeldOriginal(ctx, videoID)
}

// heldFor re-derives the held path from the video's current path and checks the journal
// agrees, so a stale or tampered row can never steer a rename or a delete.
func (s *Swapper) heldFor(ctx context.Context, videoID int64, path string) (string, error) {
	held := HeldPath(path)
	h, err := s.store.GetHeldOriginal(ctx, videoID)
	if err != nil {
		return "", err
	}
	if h == nil || h.State != repo.HeldHeld || h.HeldPath != held {
		return "", fmt.Errorf("%w: no standing hold at %s", ErrHeldMismatch, held)
	}
	if err := refuseSymlink(held); err != nil {
		return "", err
	}
	return held, nil
}

// RecoverSwaps finishes or undoes every journal row a crash left mid-operation. It must run
// before the queue's orphan sweep, which would delete a .holodex-new a swapping row still
// needs. It returns how many rows it settled; a row it can't settle is logged by the caller
// through the joined error and left for the next boot.
func (s *Swapper) RecoverSwaps(ctx context.Context) (int, error) {
	var settled int
	var errs []error
	for _, state := range []string{repo.HeldSwapping, repo.HeldRestoring, repo.HeldDiscarding} {
		rows, err := s.store.ListHeldOriginals(ctx, state)
		if err != nil {
			return settled, err
		}
		for _, h := range rows {
			if err := s.recoverOne(ctx, h); err != nil {
				errs = append(errs, fmt.Errorf("video %d: %w", h.VideoID, err))
				continue
			}
			settled++
		}
	}
	return settled, errors.Join(errs...)
}

func (s *Swapper) recoverOne(ctx context.Context, h repo.HeldOriginal) error {
	if !strings.HasSuffix(h.HeldPath, repo.HeldPathSuffix) {
		return fmt.Errorf("%w: %s", ErrHeldMismatch, h.HeldPath)
	}
	path := strings.TrimSuffix(h.HeldPath, repo.HeldPathSuffix)
	held, newp := h.HeldPath, NewPath(path)
	for _, name := range []string{held, newp} {
		if err := refuseSymlink(name); err != nil {
			return err
		}
	}
	hasPath, err1 := exists(path)
	hasHeld, err2 := exists(held)
	hasNew, err3 := exists(newp)
	if err := errors.Join(err1, err2, err3); err != nil {
		return fmt.Errorf("can't tell which files exist, so nothing moved: %w", err)
	}

	switch h.State {
	case repo.HeldSwapping:
		switch {
		case hasPath && !hasHeld: // first rename never happened
			return s.store.DeleteHeldOriginal(ctx, h.VideoID)
		case hasPath && hasHeld: // both renames happened; only the state write was lost
			return s.store.SetHeldState(ctx, h.VideoID, repo.HeldHeld)
		case hasHeld && hasNew: // between the renames: finish
			if err := s.rename(newp, path); err != nil {
				return err
			}
			return s.store.SetHeldState(ctx, h.VideoID, repo.HeldHeld)
		case hasHeld: // between the renames, candidate gone: undo
			if err := s.rename(held, path); err != nil {
				return err
			}
			return s.store.DeleteHeldOriginal(ctx, h.VideoID)
		}
		return fmt.Errorf("neither %s nor its held original exists", path)
	case repo.HeldRestoring:
		if hasHeld {
			if err := s.rename(held, path); err != nil {
				return err
			}
		}
		return s.store.DeleteHeldOriginal(ctx, h.VideoID)
	case repo.HeldDiscarding:
		if err := s.remove(held); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return s.store.DeleteHeldOriginal(ctx, h.VideoID)
	}
	return nil
}

// exists reports whether name exists, without following a symlink. Any error other than
// "not exist" is returned: a recovery that guessed would undo a finished swap.
func exists(name string) (bool, error) {
	_, err := os.Lstat(name)
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	}
	return false, err
}
