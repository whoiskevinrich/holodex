import { test } from "node:test";
import assert from "node:assert/strict";
import { classify, summarize, titleKey } from "./probe-edition-filenames.mjs";

test("classify: the four marker grammars and none", () => {
  assert.equal(classify("Blade Runner (1982) {edition-Final Cut}").cls, "plex");
  assert.equal(classify("Blade Runner (1982) {edition-Final Cut}").marker, "Final Cut");
  assert.equal(classify("Blade Runner (1982) (Director's Cut)").cls, "paren");
  assert.equal(classify("Blade Runner (1982) [Unrated]").cls, "paren");
  assert.equal(classify("Blade Runner (1982) - Final Cut").cls, "dash");
  assert.equal(classify("Blade.Runner.1982.Extended.Remastered.1080p").cls, "bare");
  assert.equal(classify("Blade Runner (1982)").cls, "none");
  // A year in parentheses is identity, not a marker.
  assert.equal(classify("Superman II (1980)").cls, "none");
  // "cut" inside another word is not a keyword.
  assert.equal(classify("Executive Decision (1996)").cls, "none");
});

test("classify: the stripped title groups editions of one film together", () => {
  const a = classify("Blade Runner (1982) {edition-Final Cut}");
  const b = classify("Blade Runner (1982) (Director's Cut)");
  const c = classify("Blade Runner (1982) - Theatrical Cut");
  const d = classify("Blade Runner (1982)");
  assert.equal(a.title, d.title);
  assert.equal(b.title, d.title);
  assert.equal(c.title, d.title);
  assert.equal(titleKey("Blade.Runner_1982"), "blade runner 1982");
});

test("summarize: counts, keywords, and multi-edition groups; nothing else", () => {
  const r = summarize([
    "/m/Blade Runner (1982) {edition-Final Cut}.mkv",
    "/m/Blade Runner (1982) (Director's Cut).mp4",
    "/m/Blade Runner (1982).mkv",
    "/m/Dune (1984).mkv",
    "/m/Dune (1984) - Extended Edition.mkv",
    "/m/Alien (1979).mkv",
    "/m/scenes/Alien (1979) - scene 3.mkv",
    "/m/notes.txt",
  ]);
  assert.equal(r.total, 8); // the walker filters by extension; summarize trusts its input
  assert.deepEqual(r.byClass, { plex: 1, paren: 1, dash: 1, bare: 0, none: 5 });
  assert.equal(r.byKeyword.cut, 2);
  assert.equal(r.byKeyword.extended, 1);
  assert.equal(r.byKeyword.edition, 1);
  assert.equal(r.multiEditionTitles, 2); // Blade Runner (3 forms), Dune (2)
  assert.equal(r.multiFileTitles, 2);
  assert.equal(r.samples.plex.length, 1);
});
