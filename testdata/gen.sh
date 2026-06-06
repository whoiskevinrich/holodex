#!/usr/bin/env bash
#
# Generates the Holodex media fixture corpus (docs/testing-strategy.md §3).
# Deterministic, tiny (1-second) synthetic clips with metadata written by
# DIFFERENT tools to reproduce real encoder/tag fragmentation (ADR-004/010/012).
#
# Requires: ffmpeg, exiftool, mkvtoolnix (mkvpropedit).  See ADR-007 (test image).
#
#   ./testdata/gen.sh
#
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
mp4="$here/mp4"
mkv="$here/mkv"
mkdir -p "$mp4" "$mkv"

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing required tool: $1" >&2; exit 1; }; }
need ffmpeg
need exiftool
need mkvpropedit

vsrc() { # vsrc <size> -> 1s test video stream args
	echo "-f lavfi -i testsrc=duration=1:size=$1:rate=1"
}

echo "==> MP4: full core metadata (iTunes atoms via ffmpeg)"
# shellcheck disable=SC2046
ffmpeg -y $(vsrc 1920x1080) \
	-metadata title="Blade Runner" \
	-metadata artist="Harrison Ford" \
	-metadata genre="Sci-Fi" \
	-metadata date="2019-10-04" \
	"$mp4/fhd_full.mp4" -loglevel error

echo "==> MP4: Publisher tag only (mapping source key, ADR-013)"
# shellcheck disable=SC2046
ffmpeg -y $(vsrc 1280x720) "$mp4/publisher.mp4" -loglevel error
exiftool -overwrite_original -Publisher="Acme Pictures" "$mp4/publisher.mp4" >/dev/null

echo "==> MP4: no metadata (filename/mtime fallback, F2.7)"
# shellcheck disable=SC2046
ffmpeg -y $(vsrc 1280x720) "$mp4/nometa.mp4" -loglevel error

echo "==> MP4: cinematic 4K scope 3840x1606 (width-based 4K+, ADR-012)"
# shellcheck disable=SC2046
ffmpeg -y $(vsrc 3840x1606) "$mp4/scope4k.mp4" -loglevel error

echo "==> MP4: near-miss FHD 1888x1062 (10% tolerance -> FHD, ADR-012)"
# shellcheck disable=SC2046
ffmpeg -y $(vsrc 1888x1062) "$mp4/nearmiss_fhd.mp4" -loglevel error

echo "==> MP4: unicode/diacritics title (FTS folding, ADR-017)"
# shellcheck disable=SC2046
ffmpeg -y $(vsrc 1280x720) -metadata title="Amélie" "$mp4/unicode.mp4" -loglevel error

echo "==> MP4: embedded cover art (Tier-1 thumbnail, ADR-009)"
ffmpeg -y -f lavfi -i color=c=teal:s=320x180:d=1 -frames:v 1 "$here/cover.png" -loglevel error
# shellcheck disable=SC2046
ffmpeg -y $(vsrc 1280x720) -i "$here/cover.png" \
	-map 0 -map 1 -c copy -disposition:v:1 attached_pic \
	"$mp4/withart.mp4" -loglevel error
rm -f "$here/cover.png"

echo "==> MKV: tags at multiple target levels + a track-level title to IGNORE (ADR-010)"
# Video + silent audio so we have an audio track to name.
ffmpeg -y -f lavfi -i testsrc=duration=1:size=1280x720:rate=1 \
	-f lavfi -i anullsrc=channel_layout=stereo:sample_rate=48000 -t 1 \
	-c:v mpeg4 -c:a aac "$mkv/multilevel.mkv" -loglevel error

cat > "$mkv/multilevel_tags.xml" <<'XML'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE Tags SYSTEM "matroskatags.dtd">
<Tags>
  <Tag>
    <Targets><TargetTypeValue>50</TargetTypeValue></Targets>
    <Simple><Name>TITLE</Name><String>Episode 1</String></Simple>
    <Simple><Name>ACTOR</Name><String>Jane Doe</String></Simple>
    <Simple><Name>GENRE</Name><String>Drama</String></Simple>
  </Tag>
  <Tag>
    <Targets><TargetTypeValue>70</TargetTypeValue></Targets>
    <Simple><Name>TITLE</Name><String>The Collection</String></Simple>
  </Tag>
</Tags>
XML
mkvpropedit "$mkv/multilevel.mkv" --tags global:"$mkv/multilevel_tags.xml" >/dev/null
# Track-level title that extraction MUST ignore (becomes streams[].tags.title).
mkvpropedit "$mkv/multilevel.mkv" --edit track:a1 --set name="Director Commentary" >/dev/null

echo "==> Edge cases: corrupt + zero-byte"
head -c 2048 /dev/urandom > "$mp4/corrupt.mp4" || true
: > "$mp4/empty.mp4"

echo "Done. Fixtures in $mp4 and $mkv."
echo "Update goldens with:  go test ./internal/metadata -run Extract -update"
