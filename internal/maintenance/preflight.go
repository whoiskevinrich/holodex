package maintenance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
)

// Refusal reasons decided before any probe (see encoders.go for the stream ones).
const (
	ReasonFileMissing     = "file_missing"
	ReasonUnsupportedFile = "unsupported_container"
	ReasonToolMissing     = "tool_missing"
	ReasonReadOnly        = "read_only"
	ReasonNoSpace         = "no_space"
)

// Preflight is what the confirm dialog shows: whether the operation can run, and if not why.
type Preflight struct {
	OK        bool   `json:"ok"`
	Reason    string `json:"reason,omitempty"` // a Reason* code
	Detail    string `json:"detail,omitempty"` // codec, tool name…
	NeedBytes uint64 `json:"need_bytes"`
	FreeBytes uint64 `json:"free_bytes"`
}

// spaceFactor: a re-encode at constant quality can come out larger than the source; this is
// the only size assumption (file-maintenance.md, encoder table).
const spaceNum, spaceDen = 3, 2

// CheckFlip decides whether a flip of path can succeed, cheapest checks first, without
// changing anything (the writability probe creates and removes its own uniquely named file).
// Refusals come back as a Preflight with OK false; err is for failures to check at all.
func CheckFlip(ctx context.Context, t Tools, encoders map[string]bool, path string) (Preflight, error) {
	fi, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return refused(ReasonFileMissing, ""), nil
	} else if err != nil {
		return Preflight{}, err
	}
	family, err := familyOf(path)
	if err != nil {
		return refused(ReasonUnsupportedFile, filepath.Ext(path)), nil
	}
	tools := []string{t.FFmpeg, t.FFprobe, t.Exiftool}
	if family == FamilyMKV {
		tools = []string{t.FFmpeg, t.FFprobe, t.Mkvmerge, "mkvextract"}
	}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			return refused(ReasonToolMissing, tool), nil
		}
	}

	dir := filepath.Dir(path)
	if err := probeWritable(dir); err != nil {
		return refused(ReasonReadOnly, ""), nil
	}
	pf := Preflight{NeedBytes: uint64(fi.Size()) * spaceNum / spaceDen}
	if pf.FreeBytes, err = diskFree(dir); err != nil {
		return Preflight{}, err
	}
	if pf.FreeBytes < pf.NeedBytes {
		pf.Reason = ReasonNoSpace
		return pf, nil
	}

	p, err := RunProbe(ctx, t.FFprobe, Demuxer(family), path)
	if err != nil {
		return Preflight{}, err
	}
	v, ok := p.MainVideo()
	if !ok {
		return refused(ReasonNoVideo, ""), nil
	}
	if _, err := EncoderArgs(v, "v:0", encoders); err != nil {
		var re *RefusalError
		if errors.As(err, &re) {
			return refused(re.Reason, re.Detail), nil
		}
		return Preflight{}, err
	}
	pf.OK = true
	return pf, nil
}

// diskFree is freeBytes; a test simulates a full disk through it.
var diskFree = freeBytes

func refused(reason, detail string) Preflight { return Preflight{Reason: reason, Detail: detail} }

// probeWritable creates and removes a uniquely named file in dir. O_EXCL means it never opens
// (and so never truncates through) something already there, symlink or stale probe alike.
// That catches a :ro mount, permissions and EROFS on every OS without reading mount tables.
func probeWritable(dir string) error {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return err
	}
	name := filepath.Join(dir, ".holodex-probe-"+hex.EncodeToString(b[:]))
	f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	f.Close()
	return os.Remove(name)
}
