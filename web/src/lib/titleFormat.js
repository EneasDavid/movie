// Real-world video filenames from scene/release-style sources look like
// "Corrida.dos.Bichos.2026.1080p.WEB-DL.NACIONAL.5.1.mkv" — technically
// accurate, but nothing like the clean title a streaming catalog shows.
// This strips the release-tag noise (resolution, source, codec, audio,
// language, group tags) and dots/underscores, keeping just the title and
// its year when present.

const EXTENSION_RE = /\.(mkv|mp4|avi|mov|webm|m4v)$/i;

// Anything from the first release-tag word onward gets dropped. Ordered
// roughly by how early these tend to appear in real filenames.
const TAG_WORDS = [
  "1080p", "720p", "2160p", "480p", "4k", "uhd",
  "web-dl", "webdl", "webrip", "web", "bluray", "blu-ray", "bdrip", "brrip",
  "hdtv", "dvdrip", "hdrip", "cam", "telesync", "ts",
  "x264", "x265", "h264", "h265", "hevc", "avc",
  "aac", "ac3", "eac3", "dts", "flac", "mp3",
  "5", "5.1", "7.1", "2.0", "dual", "dublado", "legendado", "nacional",
  "extended", "unrated", "remux", "proper", "repack",
];

const YEAR_RE = /\b(19\d{2}|20\d{2})\b/;

export function formatTitle(rawName) {
  if (!rawName) return "";

  let name = rawName.replace(EXTENSION_RE, "");
  name = name.replace(/[._]+/g, " ").replace(/\s+/g, " ").trim();

  const yearMatch = name.match(YEAR_RE);
  const year = yearMatch ? yearMatch[1] : null;

  const words = name.split(" ");
  const tagSet = new Set(TAG_WORDS);
  const kept = [];
  for (const word of words) {
    const lower = word.toLowerCase();
    if (tagSet.has(lower)) break; // everything from here on is release-tag noise
    if (year && word === year) break; // stop right before the year too
    kept.push(word);
  }

  let title = kept.join(" ").trim();
  // Fallback: if stripping left nothing (unusual naming), just clean
  // punctuation from the original rather than showing an empty title.
  if (!title) title = name.replace(YEAR_RE, "").trim() || rawName;

  return year ? `${title} (${year})` : title;
}
