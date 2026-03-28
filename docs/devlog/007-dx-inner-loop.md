# #7 — Why I Built a Dev Dashboard Instead of Writing Tests

> You can have the cleanest architecture in the world. If you can't *feel* it working, you'll never trust it.

## The Mood

Six PRs in. Outbox pattern landed. Ollama integration wired. The pipeline works — I know because the logs say so.

But logs are not *feeling*. Logs are scrolling text in a terminal. You squint, you grep, you piece together what happened. It's forensics, not feedback.

I was tired. The event bus over-engineering burned me out (devlog #6). An AI reviewer told me the project is all architecture and no proof. He was right. We had clean abstractions, retry policies, graceful shutdown — and zero visible output. The system did things, but I couldn't *see* it doing things.

## The Terminology

There's a concept in software development called **inner loop** — the cycle between making a change and seeing the result.

```
Edit code → Build → Run → See result → Edit code → ...
```

The shorter this loop, the faster you learn. The faster you learn, the better the product. Game developers figured this out decades ago — they build **editor tools**, debug overlays, in-game consoles, parameter sliders. Not for users. For themselves. To *feel* the system while building it.

The formal name is **Developer Experience (DX)**. But DX sounds corporate. What I'm talking about is more primal — it's the difference between driving a car with a windshield and driving one with only a rearview mirror.

## The Decision

I needed to see issues flowing through the AI pipeline. Not in logs. In a browser. With my prompt. Right now.

So instead of writing tests, refactoring the outbox reader, or designing the broker architecture — I built a throwaway dev dashboard.

```
go run . dev
→ http://localhost:8080
```

One HTML file. One Go file. Embedded in the binary. No npm, no React, no build step.

## What It Does

Two modes:

**Batch** — hit "Replay & Run", watch every synced issue get classified by Ollama in real-time. Cards appear one by one. Yellow while processing, green when done. SSE streaming, no polling.

**One-shot** — pick any issue from a dropdown, write a custom prompt, hit "Analyze". See the AI response for that one issue. Change the prompt, hit again. Instant iteration.

The prompt persists in `localStorage`. Reload the page — it's still there. Change it, replay, compare. The inner loop is seconds, not minutes.

## What I Learned in 10 Minutes of Playing

Things I would never have noticed from logs:

1. **qwen2.5-coder:7b is too polite** — it doesn't hold character. You need aggressive few-shot examples to make it play a role.
2. **Long issue bodies slow everything down** — some issues have 5000-character markdown bodies. The model chokes. Future optimization: summarize first, classify second.
3. **The outbox has duplicates** — when the poller re-syncs an issue that was already synced, it writes another outbox event. Not a bug (idempotent ack), but visible waste.
4. **The prompt matters more than the model** — switching from "classify this issue" to a noir detective prompt completely changed output quality. Not because the model got smarter — because the prompt gave it a voice.

None of this shows up in unit tests. All of it shows up in 10 minutes of clicking buttons.

## The Anti-Pattern I'm Okay With

This code is ugly. Raw SQL in HTTP handlers. Inline HTML. No separation of concerns. The linter caught 11 issues on the first run.

I marked every file with `// Throwaway PoC — NOT production code. See #152.`

Because the point was never clean code. The point was **shortening the inner loop** until I could feel the system working. And it worked. In one afternoon I went from "logs say it works" to "I can see it working, and here are four things to fix."

The dashboard will be deleted. The insights won't.

## The Principle

> **Build tools for yourself before building features for users.**

Not always. Not for everything. But when you're in the "does this even work?" phase — when the architecture is ahead of the proof — stop architecting and start *seeing*.

The best code I wrote today wasn't the Ollama client or the outbox reader. It was the 200-line HTML file that let me play with both.

---

*PR: #153 — the one where we stopped reading logs and started watching.*
