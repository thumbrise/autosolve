# #9 — The First Visible Output

> Eight PRs of infrastructure. One comment on a GitHub issue. That's the whole story.

## The Moment

March 29, 2:53 AM. The system posted its first comment on a real GitHub issue. Not in logs. Not in a dev dashboard. On GitHub, where a user would see it.

```
## AI Analysis
**Model:** qwen2.5-coder:7b
...
Priority: High
Component: GitHub Integration / Automation
```

A 7B model running on a MacBook M4. Analyzing [its own PR](https://github.com/thumbrise/autosolve/pull/179). The irony wasn't lost on me.

## What It Took

The pipeline that produced that comment:

```
IssuePoller → IssueSyncer → OutboxRelay → goqite → IssueExplainer → Ollama → GitHub API
```

Seven components. Four database tables. One queue. Two external APIs. Built across eight PRs over weeks. And the user sees: one comment.

That's the point. The user shouldn't see the pipeline. They should see the result.

## The Infinite Loop That Almost Was

The first version worked perfectly — once. Then it posted the comment again. And again.

The bug: posting a comment updates the issue's `updated_at` on GitHub. The poller sees a "new" update. Syncs the issue. Creates a job. The explainer runs Ollama. Posts another comment. `updated_at` changes. Poller picks it up. Forever.

### Attempt 1: Check the issue body

```go
if strings.Contains(issue.Body, autosolveMarker) {
    // skip
}
```

Wrong. The marker is in the *comment*, not the issue body. The issue body is what the user wrote when they created the issue. The check never matches. Loop continues.

### Attempt 2: Check the latest comment

```go
latestComment, _ := github.GetLatestComment(ctx, owner, repo, number)
if strings.Contains(latestComment.GetBody(), marker) {
    // skip
}
```

Better. But fragile. If a user comments between the bot's comment and the next poll — the latest comment is the user's, not the bot's. The marker isn't found. Bot posts again.

### Attempt 3: Check ALL comments

```go
alreadyResponded, _ := github.HasCommentWithMarker(ctx, owner, repo, number, marker)
if alreadyResponded {
    // skip — we already posted, regardless of what happened after
}
```

This is what shipped. Scan every comment on the issue. If *any* contains `<!-- autosolve -->`, we've already responded. Skip. Delete the job. Move on.

One response per issue. Simple rule. No races.

## The Invisible Marker

```html
<!-- autosolve -->
```

An HTML comment. Invisible when rendered. Present in the raw markdown. The user sees a clean AI analysis. The bot sees its own fingerprint.

This is how every GitHub bot works — Dependabot, Copilot, CodeRabbit. They all embed invisible markers in their comments. I didn't know this before building autosolve. Now I can't unsee it.

You can put anything in there. JSON, timestamps, job IDs:

```html
<!-- autosolve {"version":1,"jobId":"abc","analyzedAt":"2026-03-29T01:53:33Z"} -->
```

Free metadata storage in plain sight. We'll use this for idempotency later (#177).

## The GitHub App Rabbit Hole

The "proper" solution for feedback loops: use a GitHub App. The bot gets its own identity (`autosolve[bot]`), you filter by `author.login`, done.

But autosolve is self-hosted. Every user would need to create their own GitHub App. That's:

1. Go to Developer Settings → New GitHub App
2. Set permissions
3. Download a `.pem` private key
4. Install on your repos
5. Configure `appId`, `installationId`, `privateKeyPath`

For a tool that promises "just run and forget," that's too much friction. A PAT is one env variable. A GitHub App is a ceremony.

We chose PAT + marker. The marker is a hack. It works. GitHub App is a separate issue for when we need it.

## The Label Gate

During development, every synced issue triggered an Ollama call. Sixty-five open issues. All getting analyzed. All getting comments. Chaos.

Fix: `requiredLabel` in config.

```yaml
github:
  issues:
    requiredLabel: "autosolve"
```

No label → skip. The explainer checks via GitHub API (`Issues.Get` → read labels), not the local DB (labels aren't synced yet, #76). One extra API call per job. Worth it.

Now the inner loop is: create issue → add label → wait → see comment. Remove label to stop. Clean.

## What I Learned

1. **The feedback loop is the hardest bug in polling systems.** Your own writes trigger your own reads. Every polling-based bot has this problem. The solution is always some form of "recognize yourself."

2. **HTML comments are the duct tape of GitHub automation.** Invisible, durable, parseable. Every bot uses them. Now we do too.

3. **Self-hosted means no ceremony.** Users won't create GitHub Apps, configure OAuth, or manage private keys. They'll paste a token and expect it to work. Design for that.

4. **A 7B local model is surprisingly useful for triage.** It won't write code. But "classify this, suggest priority, name the component" — it nails it. The prompt matters more than the model size.

5. **Three iterations is normal.** Issue body → latest comment → all comments. Each attempt revealed a new edge case. Ship the first version, discover the bug, fix it. That's not failure — that's engineering.

---

*PR: [#179](https://github.com/thumbrise/autosolve/pull/179) — the one where the system finally spoke.*
