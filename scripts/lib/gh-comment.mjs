// Shared "sticky issue/PR comment" upsert — find an existing comment carrying `marker` and
// PATCH it, or POST a new one. Used by any advisory CI script that wants one comment that
// updates in place instead of accumulating one per push (release-candidate-comment.mjs,
// commit-type-scope-check.mjs).

import { run, runStrict } from "./imagetools.mjs";

/** The id of the existing comment carrying `marker` on `issueNumber`, or null. */
export async function findCommentId(repo, issueNumber, marker) {
  const existing = await run("gh", [
    "api",
    `repos/${repo}/issues/${issueNumber}/comments`,
    "--jq",
    `.[] | select(.body | contains("${marker}")) | .id`,
  ]);
  return existing ? existing.split("\n")[0] : null;
}

/** PATCH `id` if given, else POST a new comment on `issueNumber`. */
export async function upsertIssueComment({ repo, issueNumber, id, body }) {
  const [method, path] = id
    ? ["PATCH", `repos/${repo}/issues/comments/${id}`]
    : ["POST", `repos/${repo}/issues/${issueNumber}/comments`];
  await runStrict("gh", ["api", "--method", method, path, "-f", `body=${body}`]);
}
