package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"os"
)

// completenessInputsKey is the settings row holding the fingerprint of the
// inputs the stored completeness scores were computed against (ADR-112 D1).
// It lives in the database, beside the scores it describes, so a restored /data
// carries its own fingerprint and is re-scored if it came from another build.
const completenessInputsKey = "completeness.inputs_fingerprint"

// completenessStore is the slice of *repo.Repo the boot hook needs.
type completenessStore interface {
	GetSetting(ctx context.Context, key string) (string, bool, error)
	PutSetting(ctx context.Context, key, value string) error
	MarkAllCompletenessDirty(ctx context.Context) error
}

// markCompletenessDirtyIfInputsChanged is ADR-099 D4's boot hook, narrowed by
// ADR-112 D1: every stored score is flagged for recompute only when the build
// or the config files it is scored against differ from the ones that computed
// the store. The binary stands in for everything compiled in (registry
// criticality, the resolver, the scoring formula) with nothing to remember to
// bump. Any failure to fingerprint falls back to the old always-dirty
// behaviour, and the fingerprint is recorded only after the dirty mark lands,
// so a failed mark is retried next boot.
func markCompletenessDirtyIfInputsChanged(ctx context.Context, s completenessStore, log *slog.Logger, exe string, configFiles ...string) {
	fp, err := completenessInputsFingerprint(exe, configFiles...)
	if err != nil {
		log.Warn("completeness inputs fingerprint failed; re-scoring everything", "err", err)
	} else if prev, ok, err := s.GetSetting(ctx, completenessInputsKey); err == nil && ok && prev == fp {
		return // same build, same config: the stored scores stand
	}
	if err := s.MarkAllCompletenessDirty(ctx); err != nil {
		log.Warn("mark completeness dirty at boot failed", "err", err)
		return
	}
	if fp == "" {
		return // the Warn above already says why
	}
	log.Info("completeness inputs changed; re-scoring every entity in the background")
	if err := s.PutSetting(ctx, completenessInputsKey, fp); err != nil {
		log.Warn("record completeness inputs fingerprint failed", "err", err)
	}
}

// completenessInputsFingerprint hashes the executable and each config file's
// bytes. A config path that is empty or names a missing file hashes as absent
// (both load as "no mappings"/"no providers"), so creating or deleting the file
// still changes the fingerprint.
func completenessInputsFingerprint(exe string, configFiles ...string) (string, error) {
	h := sha256.New()
	for _, p := range append([]string{exe}, configFiles...) {
		if err := hashFile(h, p, p == exe); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashFile(h io.Writer, path string, required bool) error {
	if path == "" && required {
		return errors.New("executable path unknown")
	}
	var f *os.File
	err := fs.ErrNotExist
	if path != "" {
		f, err = os.Open(path)
	}
	if !required && errors.Is(err, fs.ErrNotExist) {
		_, err = h.Write([]byte{0}) // absent
		return err
	}
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	// Length-prefix each file so the concatenation is unambiguous.
	if _, err := h.Write([]byte{1}); err != nil {
		return err
	}
	if err := binary.Write(h, binary.BigEndian, info.Size()); err != nil {
		return err
	}
	_, err = io.Copy(h, f)
	return err
}
