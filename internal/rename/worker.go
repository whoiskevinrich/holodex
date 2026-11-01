package rename

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"holodex/internal/repo"
)

// Payload is a rename job's data: what the plan showed. The worker never trusts it; it builds the
// name again and fails the job when the two differ (security-perimeter.md "Rename names").
type Payload struct {
	Name     string              `json:"name"`
	Template string              `json:"template,omitempty"`
	Written  map[string][]string `json:"written,omitempty"`
	// UndoOf names the done journal row an undo reverses (P0-14); Name is then its old name.
	UndoOf int64 `json:"undo_of,omitempty"`
}

// PlanFunc gives one video's plan row, with the videos in run counting as moving in the same run.
type PlanFunc func(ctx context.Context, videoID int64, run []int64) (Row, error)

// Job outcomes the queue acts on.
var (
	// ErrRequeue sends a job back to pending behind the job it waits for (a batch).
	ErrRequeue = errors.New("waiting for another rename in the batch")
	// ErrNoop finishes a job that had nothing to do: the name is already the desired one.
	ErrNoop = errors.New("already named")
)

// maxWaits bounds how often a job goes back to pending before it fails.
const maxWaits = 20

// Executor runs rename jobs. Root is the library root every folder must sit inside.
type Executor struct {
	Repo *repo.Repo
	Root string
	Plan PlanFunc
}

// Run executes one rename job (file-rename.md "Crash ordering"): journal, rename, one transaction.
// It returns nil on success, ErrNoop or ErrRequeue, or the failure.
func (e *Executor) Run(ctx context.Context, job *repo.WritebackJob) error {
	var p Payload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return errors.New("corrupt payload")
	}
	path, err := e.Repo.PathByID(ctx, job.VideoID)
	if err != nil {
		return fmt.Errorf("video not found: %w", err)
	}
	if held, err := e.Repo.GetHeldOriginal(ctx, job.VideoID); err != nil {
		return err
	} else if held != nil {
		return errors.New("the video has a held original")
	}
	dir, cur := filepath.Dir(path), filepath.Base(path)
	f, err := OpenFolder(e.Root, dir)
	if err != nil {
		return err
	}
	defer f.Close()

	// A journal row left by an earlier attempt: finish a rename that landed, resume one aside at
	// its via name, and drop any other, so this attempt plans afresh.
	pending, err := e.Repo.PendingRename(ctx, job.VideoID)
	if err != nil {
		return err
	}
	if pending != nil {
		atOld, atNew, atVia, err := where(f, *pending)
		switch {
		case err != nil:
			return err
		case atNew && !atOld:
			return e.Repo.CompleteRename(ctx, *pending, dir)
		case !atVia:
			if err := e.Repo.DeletePendingRename(ctx, pending.ID); err != nil {
				return err
			}
			pending = nil
		}
	}
	fr := repo.FileRename{VideoID: job.VideoID, OldName: cur, NewName: p.Name, Template: p.Template,
		TokenValues: p.Written, Trigger: repo.RenameTriggerManual}
	if p.UndoOf != 0 {
		if err := e.checkUndo(ctx, job.VideoID, cur, p); err != nil {
			return err
		}
		fr.Trigger, fr.UndoOf, fr.Template, fr.TokenValues = repo.RenameTriggerUndo, p.UndoOf, "", nil
	} else {
		if locked, err := e.Repo.NameLocked(ctx, job.VideoID); err != nil {
			return err
		} else if locked {
			return errors.New("the name is locked")
		}
		run, err := e.batchRun(ctx, job)
		if err != nil {
			return err
		}
		row, err := e.Plan(ctx, job.VideoID, run)
		if err != nil {
			return err
		}
		switch {
		case row.Reason == ReasonAlreadyNamed && pending == nil:
			return ErrNoop
		case !Renames(row.Reason) && pending == nil:
			return fmt.Errorf("not renamed: %s", row.Reason)
		case row.Name != p.Name && pending == nil:
			return errors.New("the name changed since the plan; refresh the plan")
		}
	}

	src := cur
	if pending != nil {
		fr, src = *pending, pending.ViaName
	} else {
		if fr.ID, err = e.Repo.InsertPendingRename(ctx, fr); err != nil {
			return err
		}
	}
	journal := func(via string) error {
		fr.ViaName = via
		return e.Repo.SetRenameVia(ctx, fr.ID, via)
	}
	if err := f.Move(job.VideoID, src, fr.NewName, journal); err != nil {
		return e.unwind(ctx, f, dir, job, fr, err)
	}
	return e.Repo.CompleteRename(ctx, fr, dir)
}

// unwind handles a failed move. A target held by another job of this batch waits for it, and a
// cycle of such jobs frees itself by stepping this file aside to its via name (the row gives up
// its old path with it). Anything else puts the file back at its old name and drops the journal
// row.
func (e *Executor) unwind(ctx context.Context, f *Folder, dir string, job *repo.WritebackJob, fr repo.FileRename, moveErr error) error {
	via := ViaName(job.VideoID)
	aside, _ := f.Exists(via)
	if errors.Is(moveErr, ErrExists) && fr.UndoOf == 0 && job.Attempts <= maxWaits {
		waits, cycle, err := e.waitsInBatch(ctx, job, fr.NewName)
		if err != nil {
			return err
		}
		switch {
		case waits && aside:
			return ErrRequeue // the pending row keeps the via name
		case waits && cycle:
			if err := f.StepAside(job.VideoID, fr.OldName); err != nil {
				return errors.Join(err, e.Repo.DeletePendingRename(ctx, fr.ID))
			}
			return errors.Join(ErrRequeue, e.Repo.StepAsideRename(ctx, fr, dir, via))
		case waits:
			return errors.Join(ErrRequeue, e.Repo.DeletePendingRename(ctx, fr.ID))
		}
	}
	if aside {
		if err := f.move(via, fr.OldName); err != nil {
			// The file stays at its via name, journalled; boot recovery or Retry finishes it.
			return fmt.Errorf("%w; the file is left at its via name", moveErr)
		}
		fr.ViaName = via
	}
	if errors.Is(moveErr, ErrExists) {
		moveErr = errors.New("the name is taken")
	}
	return errors.Join(moveErr, e.Repo.AbandonRename(ctx, fr, dir))
}

// batchRun lists the videos whose renames are still queued in this job's batch.
func (e *Executor) batchRun(ctx context.Context, job *repo.WritebackJob) ([]int64, error) {
	if job.BatchID == "" {
		return []int64{job.VideoID}, nil
	}
	jobs, err := e.Repo.BatchRenameJobs(ctx, job.BatchID)
	if err != nil {
		return nil, err
	}
	run := make([]int64, 0, len(jobs))
	for _, j := range jobs {
		run = append(run, j.VideoID)
	}
	return run, nil
}

// waitsInBatch reports whether target is held by a video with a queued rename in this batch, and
// whether following each holder's own target leads back to this job (a cycle, such as a swap).
func (e *Executor) waitsInBatch(ctx context.Context, job *repo.WritebackJob, target string) (waits, cycle bool, err error) {
	if job.BatchID == "" {
		return false, false, nil
	}
	jobs, err := e.Repo.BatchRenameJobs(ctx, job.BatchID)
	if err != nil {
		return false, false, err
	}
	type member struct {
		id     int64
		target string
	}
	byName := map[string]member{} // fold key of a member's current name → the member
	for _, j := range jobs {
		path, err := e.Repo.PathByID(ctx, j.VideoID)
		if err != nil {
			continue
		}
		var p Payload
		if json.Unmarshal([]byte(j.Payload), &p) != nil {
			continue
		}
		byName[FoldKey(filepath.Base(path))] = member{j.VideoID, p.Name}
	}
	m, ok := byName[FoldKey(target)]
	if !ok || m.id == job.VideoID {
		return false, false, nil
	}
	for range len(byName) {
		if m.id == job.VideoID {
			return true, true, nil
		}
		next, ok := byName[FoldKey(m.target)]
		if !ok {
			break
		}
		m = next
	}
	return true, false, nil
}

// checkUndo enforces P0-14: only the latest rename, only while the file still has the name
// Holodex wrote, and only onto a free name. Undo never disambiguates.
func (e *Executor) checkUndo(ctx context.Context, videoID int64, cur string, p Payload) error {
	last, err := e.Repo.LatestDoneRename(ctx, videoID)
	if err != nil {
		return err
	}
	if last == nil || last.ID != p.UndoOf || last.NewName != cur || last.OldName != p.Name {
		return errors.New("only the latest rename can be undone, while the file still has that name")
	}
	return nil
}

// RecoverPending replays every pending journal row before the queue starts (file-rename.md "Boot
// recovery"). It returns how many rows it settled.
func RecoverPending(ctx context.Context, r *repo.Repo, root string) (int, error) {
	rows, err := r.PendingRenames(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	var errs []error
	for _, fr := range rows {
		if err := recoverOne(ctx, r, root, fr); err != nil {
			errs = append(errs, fmt.Errorf("rename %d: %w", fr.ID, err))
			continue
		}
		n++
	}
	return n, errors.Join(errs...)
}

// where reports which of a journal row's names the file answers to.
func where(f *Folder, fr repo.FileRename) (atOld, atNew, atVia bool, err error) {
	atOld, _ = f.Exists(fr.OldName)
	atNew, _ = f.Exists(fr.NewName)
	if fr.ViaName != "" {
		atVia, _ = f.Exists(fr.ViaName)
	}
	// On a case-insensitive mount a case-only rename's old and new names answer alike; the
	// listing tells which spelling is on disk.
	if FoldKey(fr.OldName) == FoldKey(fr.NewName) && atOld && atNew {
		names, err := f.list()
		if err != nil {
			return false, false, false, err
		}
		atOld, atNew = false, false
		for _, n := range names {
			atOld = atOld || n == fr.OldName
			atNew = atNew || n == fr.NewName
		}
	}
	return atOld, atNew, atVia, nil
}

func recoverOne(ctx context.Context, r *repo.Repo, root string, fr repo.FileRename) error {
	path, err := r.PathByID(ctx, fr.VideoID)
	if err != nil {
		return r.DeletePendingRename(ctx, fr.ID)
	}
	dir := filepath.Dir(path)
	f, err := OpenFolder(root, dir)
	if err != nil {
		return err // the mount may not be ready; the scan's F79 net is the fallback
	}
	defer f.Close()
	atOld, atNew, atVia, err := where(f, fr)
	if err != nil {
		return err
	}
	switch {
	case atNew && !atOld:
		return r.CompleteRename(ctx, fr, dir)
	case atOld && !atNew && !atVia:
		return r.DeletePendingRename(ctx, fr.ID) // the requeued job starts again
	case atVia && !atOld && !atNew:
		if job, err := r.HasRenameJob(ctx, fr.VideoID); err != nil || job {
			return err // the requeued job resumes from the via name
		}
		if err := f.move(fr.ViaName, fr.OldName); err != nil {
			return err
		}
		return r.AbandonRename(ctx, fr, dir)
	default:
		// At none of the names, or at more than one: leave the files to the next scan.
		if err := r.FailRenameJobs(ctx, fr.VideoID, "the file was not where the rename left it"); err != nil {
			return err
		}
		return r.DeletePendingRename(ctx, fr.ID)
	}
}
