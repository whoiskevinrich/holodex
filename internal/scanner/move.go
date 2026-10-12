package scanner

import (
	"context"
	"errors"
	"io/fs"
	"os"
)

// matchHeld pairs each held file with the missing rows of the same content and
// repoints the one-to-one pairs; every other held file is inserted as new (F79,
// media-ingest.md "Moves recognised by a sampled content hash"). A row is
// missing when it is hidden, or active but unseen this pass with no file at its
// path. A held file with several candidates, or a candidate several held files
// claim, is ambiguous: nothing is guessed, and the file arrives as new. Runs
// after the walk and before deactivation.
func (s *Scanner) matchHeld(ctx context.Context, st *stats) {
	if len(st.held) == 0 {
		return
	}
	seen := make(map[int64]bool, len(st.seenIDs))
	for _, id := range st.seenIDs {
		seen[id] = true
	}
	cands := make([][]int64, len(st.held))
	claims := map[int64]int{}
	for i, h := range st.held {
		rows, err := s.repo.MoveCandidates(ctx, h.v)
		if err != nil {
			s.log.Warn("move candidates failed; indexing as new", "path", h.path, "err", err)
			continue
		}
		for _, c := range rows {
			if c.Active && seen[c.ID] {
				continue // present this pass: h is a copy, not a move
			}
			// A row whose file failed or was unreadable this pass is unseen but
			// not gone: only a path that definitely no longer exists can move.
			if _, err := os.Stat(c.FilePath); !errors.Is(err, fs.ErrNotExist) {
				continue
			}
			cands[i] = append(cands[i], c.ID)
			claims[c.ID]++
		}
	}
	for i, h := range st.held {
		switch c := cands[i]; {
		case len(c) == 1 && claims[c[0]] == 1:
			if err := s.repo.RepointVideo(ctx, c[0], h.v, h.ex.Extra); err != nil {
				s.log.Warn("repoint failed; indexing as new", "id", c[0], "path", h.path, "err", err)
				break
			}
			st.recordSeen(c[0])
			st.incRepointed()
			s.log.Info("recognised moved file", "id", c[0], "path", h.path)
			continue
		case len(c) > 0:
			st.incAmbiguous()
			s.log.Info("ambiguous move; indexing as new", "path", h.path, "candidates", len(c))
			if id := s.commit(ctx, st, h.path, h.v, h.ex, false, false); id != 0 {
				if st.ambiguousIDs == nil {
					st.ambiguousIDs = map[int64]bool{}
				}
				st.ambiguousIDs[id] = true
			}
			continue
		}
		s.commit(ctx, st, h.path, h.v, h.ex, false, false)
	}
}

// foldMoves heals moves that already happened: each hidden older row is folded
// with the one visible newer row of the same content (F79 P0-5). It runs after
// deactivation, so a row this pass just hid is a candidate too, and re-derives
// the survivor's studio and people links from the edits it now carries.
// Ambiguous pairs are counted and left alone.
func (s *Scanner) foldMoves(ctx context.Context, st *stats) {
	pairs, ambiguous, err := s.repo.FoldPairs(ctx)
	if err != nil {
		s.log.Warn("listing moves to heal failed", "err", err)
		return
	}
	for _, id := range ambiguous {
		if !st.ambiguousIDs[id] { // matchHeld already counted it this pass
			st.incAmbiguous()
		}
	}
	for _, p := range pairs {
		if err := s.repo.FoldMovedVideo(ctx, p.Older, p.Newer); err != nil {
			s.log.Warn("healing a moved file failed", "older", p.Older, "newer", p.Newer, "err", err)
			continue
		}
		st.incFolded()
		s.log.Info("healed moved file", "older", p.Older, "newer", p.Newer)
		if s.relink != nil {
			if err := s.relink(ctx, p.Older); err != nil {
				s.log.Warn("relink after heal failed", "id", p.Older, "err", err)
			}
		}
	}
}
