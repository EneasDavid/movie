#!/usr/bin/env bash
#
# fix-audio-codec.sh — batch-remux source videos with a browser-incompatible
# audio codec (most commonly E-AC-3 / "eac3", the root cause repeatedly
# confirmed for this catalog) into a browser-safe MP4 with AAC audio.
#
# WHY THIS SCRIPT EXISTS INSTEAD OF A SERVER-SIDE FIX:
# The app's /api/stream endpoint proxies whatever bytes are in the Google
# Drive file as-is — that part works correctly, confirmed live on Vercel.
# The failure happens *after* the correct bytes reach the browser: the
# <video> element itself refuses to decode an E-AC-3 audio track (no
# browser ships a decoder for it; this isn't fixable in JS/CSS/React).
#
# Real-time server-side transcoding was considered and rejected: Vercel's
# Go Framework Preset deploys a plain compiled binary with no ffmpeg in
# the runtime image, so the server can't shell out to it in production —
# and even if it could, re-encoding a feature-length video in real time
# per request would blow past Vercel's execution/memory limits by a wide
# margin. The only correct fix is upstream of the app: re-encode the
# source file once, here, before it's uploaded to Drive.
#
# WHAT IT DOES:
# Recursively scans INPUT_DIR for video files. For each one, inspects the
# first audio stream with ffprobe. If it's already AAC/MP3 *and* the
# container is already .mp4/.m4v, the file is left alone (nothing to
# fix). Otherwise it remuxes into OUTPUT_DIR, preserving the relative
# folder layout: video stream is stream-copied (no re-encode — lossless,
# fast, seconds not hours) and audio is re-encoded to AAC. Subtitle
# streams are dropped (the player has no subtitle UI to use them, and
# MKV subtitle formats often aren't valid in MP4 anyway).
#
# USAGE:
#   ./scripts/fix-audio-codec.sh <input_dir> [output_dir]
#
# Requires ffmpeg + ffprobe on PATH (macOS: `brew install ffmpeg`).
#
# After it finishes, upload the files from output_dir to the catalog's
# Google Drive folder in place of the originals (same filenames, minus
# the extension change to .mp4 — delete the old .mkv/.avi/etc. so the
# catalog doesn't list both).

set -euo pipefail

INPUT_DIR="${1:?Usage: $0 <input_dir> [output_dir]}"
OUTPUT_DIR="${2:-${INPUT_DIR%/}/fixed}"

if ! command -v ffmpeg >/dev/null 2>&1 || ! command -v ffprobe >/dev/null 2>&1; then
  echo "error: ffmpeg/ffprobe not found on PATH — install with 'brew install ffmpeg'" >&2
  exit 1
fi
if [ ! -d "$INPUT_DIR" ]; then
  echo "error: input dir does not exist: $INPUT_DIR" >&2
  exit 1
fi

mkdir -p "$OUTPUT_DIR"

# Extensions worth scanning. Add more here if the catalog uses them.
VIDEO_EXTS=(mkv mp4 avi mov m4v webm wmv flv ts)

fixed_count=0
skipped_count=0
failed_count=0

log_ok()   { echo "  ok: $*"; }
log_skip() { echo "  skip: $*"; }
log_fail() { echo "  FAIL: $*" >&2; }

is_video_ext() {
  local ext="${1##*.}"
  local lower
  lower="$(echo "$ext" | tr '[:upper:]' '[:lower:]')"
  for e in "${VIDEO_EXTS[@]}"; do
    [ "$lower" = "$e" ] && return 0
  done
  return 1
}

echo "Scanning $INPUT_DIR ..."
echo

# find -print0 / read -d '' handles filenames with spaces safely, which
# matters — these are movie/show titles with punctuation.
while IFS= read -r -d '' file; do
  rel="${file#"$INPUT_DIR"/}"
  is_video_ext "$file" || continue

  audio_codec="$(ffprobe -v error -select_streams a:0 -show_entries stream=codec_name \
    -of default=noprint_wrappers=1:nokey=1 "$file" 2>/dev/null || true)"

  if [ -z "$audio_codec" ]; then
    log_fail "$rel — no audio stream detected, skipping"
    failed_count=$((failed_count + 1))
    continue
  fi

  ext_lower="$(echo "${file##*.}" | tr '[:upper:]' '[:lower:]')"
  if { [ "$audio_codec" = "aac" ] || [ "$audio_codec" = "mp3" ]; } \
     && { [ "$ext_lower" = "mp4" ] || [ "$ext_lower" = "m4v" ]; }; then
    log_skip "$rel — already $audio_codec in .$ext_lower, no fix needed"
    skipped_count=$((skipped_count + 1))
    continue
  fi

  out_path="$OUTPUT_DIR/${rel%.*}.mp4"
  mkdir -p "$(dirname "$out_path")"

  echo "  fixing: $rel  (audio: $audio_codec -> aac)"
  if ffmpeg -y -v error -i "$file" \
      -map 0:v:0 -map 0:a:0 \
      -c:v copy -c:a aac -b:a 192k \
      -movflags +faststart \
      "$out_path" 2>&1 | sed 's/^/    ffmpeg: /'; then
    log_ok "$rel -> $(basename "$out_path")"
    fixed_count=$((fixed_count + 1))
  else
    log_fail "$rel — ffmpeg exited with an error"
    rm -f "$out_path"
    failed_count=$((failed_count + 1))
  fi
done < <(find "$INPUT_DIR" -type f -not -path "$OUTPUT_DIR/*" -print0)

echo
echo "Done. fixed=$fixed_count skipped=$skipped_count failed=$failed_count"
echo "Fixed files are in: $OUTPUT_DIR"
echo "Upload these to Drive in place of the originals, then delete the old ones so the catalog doesn't list duplicates."
