# AGENT-MIRROR: 48h Validation & Architecture Plan

**Vizyon bağı:** Bu iş, local-first AI kodlama araçlarında "kendi verinle kendi öngörünü çıkar" güvenini ve farkındalığını kurmaya hizmet ediyor — çünkü geliştiriciler artık aynı pencerede hem üretkenlik hem de mahremiyet istiyor.

**Product:** `npx agent-mirror` — a local-first dashboard that reads your AI coding agent history (Claude Code, Codex, Cursor, etc.) and generates private "AI Coding Wrapped" insights.

**Immediate question:** Should you code now or validate first?
**Recommendation:** Validate first, build a *tiny* artifact only to enable validation, and decide by Day 2.

**48h plan:**
- **Day 0–24:** Run 5–7 user interviews + post 5 X/Reddit variants.
- **Day 24–48:** Analyze intent signals, finalize wedge, and build the *smallest* possible demo (Claude-only, 2 insights, local, one PNG export card).
- **End of 48h:** Kill, pivot, or commit to MVP.

---

# Track A — Technical Architecture

Assumptions:
- TypeScript, Next.js App Router, local-only execution.
- MVP starts with **Claude Code only**; architecture makes adapters pluggable.
- No network egress for data processing. All parsing and aggregation happen in the CLI process.
- Avoid heavy infrastructure. No database server; SQLite or flat JSON cache.

## 1. Folder/file discovery strategy per tool

Use a per-tool `DiscoveryProfile` instead of hard-coding paths. The scanner reads these profiles and globs user-configurable roots.

| Tool | Typical locations | File patterns | Notes |
|---|---|---|---|
| **Claude Code** | `~/.claude/`, project `.claude/` | `*.json`, `*.jsonl`, `history*` | JSONL conversation logs. Project hint often in metadata. |
| **Codex CLI** | `~/.codex/`, project `.codex/` | `*.jsonl`, `sessions/*`, `history*` | Similar to Claude; OpenAI format likely. |
| **Gemini CLI** | `~/.gemini/`, `~/.google/` | `*.jsonl`, `conversations/*` | Unknown; expose as adapter config. |
| **Cursor** | `~/.cursor/`, `.cursor/` | `*.json`, `composer/*`, `chat-history*` | May include composer and tab edits. |
| **Aider** | project root | `.aider.chat.history.md`, `.aider.input.history` | Markdown history per project; easy to parse. |
| **Continue** | `~/.continue/`, `.continue/` | `config.json`, `sessions/*` | Config-driven; session format may vary. |
| **OpenCode** | `~/.opencodesessions/`, project `.opencodesessions/` | `*.json` | Inspect real app for exact schema. |

**Discovery function signature:**
```ts
type FileDiscovery = {
  name: string;
  roots: string[];                  // default roots to scan
  globs: string[];                  // file patterns
  exclude?: string[];               // regex or glob exclusions
  projectHint?: (absPath: string) => string | null;
};
```

## 2. Adapter interface

```ts
export type AgentAdapter = {
  id: string;                       // e.g. 'claude-code'
  name: string;                     // human name

  isAvailable: () => Promise<boolean>;

  discover: (opts: ScanOptions) => AsyncIterable<RawSessionFile>;

  parse: (file: RawSessionFile) => Promise<NormalizedSession | null>;

  enrich: (raw: RawSessionFile, parsed: NormalizedSession) => Promise<NormalizedSession>;
};

export type ScanOptions = {
  roots: string[];
  since?: Date;
  until?: Date;
  excludePaths?: string[];
};
```

**Ponytail shortcut:** Adapters are plain objects, not a plugin marketplace. For trusted tools only, hard-coded initially.

## 3. Normalized session schema

```ts
export type NormalizedSession = {
  id: string;
  adapterId: string;
  projectId: string | null;
  startedAt: string;
  endedAt: string | null;
  model: string | null;
  title: string | null;

  messages: NormalizedMessage[];
  stats: {
    messageCount: number;
    userMessageCount: number;
    assistantMessageCount: number;
    toolCalls: number;
    tokens?: { input: number; output: number };
  };

  rawFingerprint: string;
};

export type NormalizedMessage = {
  id: string;
  role: 'user' | 'assistant' | 'system' | 'tool';
  timestamp: string;
  content: string;
  toolCalls?: ToolCall[];
};

export type ToolCall = {
  tool: string;
  args: Record<string, unknown>;
  result?: unknown;
};
```

## 4. Project detection

Three-tier heuristic:
1. **Explicit** — adapter metadata contains a working directory or project name.
2. **Git root** — walk up from the session file path until `.git/`.
3. **Path clustering** — group sessions whose file paths share a common ancestor depth ≥ N.

```ts
export async function detectProject(filePath: string): Promise<ProjectRef | null> {
  const gitRoot = await findGitRoot(filePath);
  if (gitRoot) {
    return { id: slugify(gitRoot), name: basename(gitRoot), path: gitRoot };
  }
  const packageRoot = await findPackageRoot(filePath);
  if (packageRoot) {
    return { id: slugify(packageRoot), name: basename(packageRoot), path: packageRoot };
  }
  return null;
}
```

## 5. Privacy-safe aggregation layer

Principle: The UI never receives raw messages unless the user explicitly opens a single session.

```ts
export type Aggregates = {
  totalSessions: number;
  totalMessages: number;
  topProjects: { projectId: string; sessions: number; messages: number }[];
  hourlyHeatmap: number[];
  modelBreakdown: Record<string, number>;
  topTools: { tool: string; count: number }[];
  failureSignals: FailureSignal[];
};

export type FailureSignal = {
  type: 'repeat-edit' | 'error-loop' | 'rollback-pattern';
  projectId: string;
  count: number;
  description: string;
};
```

Privacy rules:
- Aggregations only.
- Regex redaction for tokens/keys before display.
- No derived text snippets longer than 50 chars unless clicked through.
- All computation in the CLI Node process.

## 6. Wrapped-card generation flow

```txt
[ scan ] -> [ cache normalized sessions ] -> [ aggregate per range ] -> [ insight engine ] -> [ card template ] -> [ render ] -> [ PNG export ]
```

Default range: 30 days.

**Insight engine (rule-based MVP):**
- "Most active project"
- "Loop of the week"
- "Peak coding hour"
- "Model mix"
- "Total agent interactions"

No cloud LLM for v1.

## 7. PNG export approach

**Option A — Client-side (recommended for MVP):**
- Render card as a React component.
- Use `html-to-image` to generate PNG.
- Trigger download with `URL.createObjectURL`.

**Option B — Server-side:**
- Serve `/card.png` and use Playwright/Puppeteer.
- Better quality, but adds heavy binary.

Verdict: Start with Option A.

## 8. Local cache strategy

SQLite via `better-sqlite3` in `~/.agent-mirror/cache.db`.

```sql
CREATE TABLE sessions (
  id TEXT PRIMARY KEY,
  adapter_id TEXT NOT NULL,
  project_id TEXT,
  started_at TEXT,
  fingerprint TEXT NOT NULL,
  payload JSON NOT NULL
);
CREATE INDEX idx_sessions_adapter ON sessions(adapter_id);
CREATE INDEX idx_sessions_project ON sessions(project_id);
CREATE INDEX idx_sessions_started ON sessions(started_at);

CREATE TABLE scans (
  path TEXT PRIMARY KEY,
  fingerprint TEXT NOT NULL,
  scanned_at TEXT NOT NULL
);
```

Invalidation: check `scans.fingerprint` against current (path + mtime + size).

## 9. CLI startup flow

```bash
npx agent-mirror
```

1. Check Node ≥ 18.
2. Resolve config: `~/.agent-mirror/config.json`.
3. Ask permission to read agent history directories (first run only).
4. Register available adapters.
5. Scan + cache sessions (background while UI loads).
6. Start Next.js server on random localhost port.
7. Open browser.
8. Watch for file changes; optionally re-scan on interval.

## 10. Security risks

| Risk | Mitigation |
|---|---|
| Reading private prompts/code | Adapter boundaries; exclude regex; first-run consent; no cloud upload. |
| Local web server CSRF/SSRF | Bind to `127.0.0.1` random port; require token in header. |
| Malicious adapter code | Ship only reviewed, open-source adapters. No third-party adapter loading in v1. |
| PNG export leaks | Default export strips message content. |
| Dependency compromise | Pin deps; prefer small deps; audit in CI. |
| Cache file exposure | Set `~/.agent-mirror/` permissions to `0o700`. |

## 11. Test strategy

| Layer | How |
|---|---|
| Adapter parsers | Unit tests using fixture files. |
| Normalization | Snapshot tests on parsed fixtures. |
| Aggregation | Property-based checks. |
| CLI startup | Smoke test: `--version` and `--help`. |
| E2E | Playwright: dashboard loads, one card renders. |

## 12. Package structure

```
agent-mirror/
├── apps/
│   └── web/                    # Next.js App Router
├── packages/
│   ├── cli/                    # npx entry point
│   ├── core/                   # scan, normalize, cache, aggregate
│   ├── ui/                     # shared React components
│   └── adapters/
│       ├── claude-code/
│       ├── codex/
│       ├── cursor/
│       ├── aider/
│       ├── continue/
│       ├── gemini/
│       └── opencode/
└── tooling/
    └── tsconfig.base.json
```

**Simpler alternative:** flat monorepo until second adapter stabilizes.

## 13. Minimal implementation plan

### Phase 0 — Validation artifact (0–2 days)
- One CLI command starts a web server.
- Reads only Claude Code history.
- Shows a single "Last 7 days" Wrapped card.
- PNG export via html-to-image.
- No cache, no multi-agent, no coach.

### Phase 1 — MVP (1 week)
- Adapter interface + Claude adapter hardened.
- SQLite cache + incremental scan.
- Project detection via git root.
- 3 insight types: total activity, top project, loop signal.

### Phase 2 — Differentiation (2–4 weeks)
- Second adapter (Aider or Codex).
- Failure loop detector.
- Prompt quality coach (rule-based).

### Phase 3 — Scale
- Multi-agent Wrapped merge.
- Optional local LLM summarization (Ollama/LM Studio).
- Sharing / persona cards.

---

# Track B — Validation Plan

## 1. Five fake landing page variants

**Variant A — "Spotify Wrapped for all your AI coding agents"**
- Headline: "Your AI Coding Wrapped — private, local, shareable."
- Sub: "Claude Code, Codex, Cursor, Aider. One dashboard. Zero cloud."
- CTA: "Get early access"

**Variant B — "Debug loop detector for vibe coders"**
- Headline: "Stop looping. Start shipping."
- Sub: "Find the broken feedback loops hiding in your agent history."
- CTA: "Find my loops."

**Variant C — "Local AI coding coach"**
- Headline: "The coach that watches every agent session — privately."
- Sub: "Get better prompts, fewer loops, and clearer retros."
- CTA: "Train my agent IQ."

**Variant D — "Private project autopsy"**
- Headline: "Autopsy your last project. In 30 seconds."
- Sub: "Where did your AI agent waste the most time? Local-first answers."
- CTA: "Run an autopsy."

**Variant E — "Multi-agent usage mirror"**
- Headline: "See every AI agent you used this month."
- Sub: "One mirror for Claude, Codex, Cursor, and the rest."
- CTA: "Mirror my agents."

## 2. Five demo screenshot concepts

1. Wrapped card: "147 agent interactions this week," top project, peak hour, model mix.
2. Loop detector: Red timeline showing 8 repeated edits on `api.ts`.
3. Project autopsy: Time spent per file, repeated failures, final outcome.
4. Multi-agent dashboard: Claude Code vs. Codex vs. Aider usage.
5. Persona card: "The Architect" badge with funny stats.

## 3. Five X launch post drafts

**A:**
> I built a private "Spotify Wrapped" for Claude Code/Cursor/Codex usage. No cloud. No telemetry. Just a local dashboard that reads your agent history and roasts your coding habits. Who wants to see their Wrapped?

**B:**
> The #1 reason vibe coding stalls: asking the same agent the same thing 6 times and not noticing. I made a local tool that finds those loops in your Claude/Cursor history. Would you use it?

**C:**
> Most devs don't review their AI prompts. I made a local-first "prompt coach" that reads your Claude Code history and tells you where your asks broke down. Useful or creepy?

**D:**
> Just finished a feature. Took 40 agent turns. What if you could autopsy exactly where the time went — entirely locally? I'm building that. What should it surface first?

**E:**
> I use Claude Code, Codex, and Aider in the same week and have no idea which is actually helping. So I'm building a local multi-agent mirror. What stats would you want to see?

## 4. Five Reddit post drafts

**r/ClaudeCodeApp:**
> [Showoff Saturday] I made a local dashboard that reads Claude Code history and generates weekly retros. No cloud. The first insight: I spend 30% of my turns fixing loops I created. Feedback welcome.

**r/programming:**
> Show HN-ish: local-first analytics for AI coding agents (Claude Code, Cursor, Codex, Aider). "Spotify Wrapped for your coding agents." Privacy-first because nothing leaves your machine. Would you actually use this?

**r/opensource:**
> I'm thinking of open-sourcing a local "AI Coding Wrapped" tool. It scans your agent history and surfaces loops, peak hours, and model usage. What features would make it worth installing?

**r/cursor:**
> How many of you review your Cursor composer sessions after? I'm building a local autopsy tool that finds repeated failed edits. Curious if anyone else feels this pain.

**r/webdev:**
> Devs spend hours with AI agents but never measure the signal. I'm prototyping a private local dashboard. Before I build, what metric would make you go "huh, I needed to know that"?

## 5. Twenty interview questions

1. Walk me through the last coding task where you used an AI agent.
2. Which agents do you use weekly? For what kinds of tasks?
3. How many agent sessions do you think you had last week?
4. Do you ever go back and read old agent conversations? When?
5. What does your current workflow look like after an agent session ends?
6. Describe the last time an agent got stuck in a loop. How did you notice?
7. How do you currently figure out if an agent session was "good"?
8. What is the most frustrating part of working with AI agents right now?
9. Have you ever realized you were asking the agent the same thing repeatedly?
10. Do you review which model/agent performed better for different tasks?
11. If you had a weekly summary of your agent usage, what would you want to know?
12. Would you want to see stats on your prompts (length, repetition, success)?
13. Have you ever shared an agent session summary with a teammate?
14. Would you want to know your "peak productivity hours" with agents?
15. Would you install a tool that reads your agent history locally?
16. What would make you uncomfortable about a tool reading your prompts?
17. How important is "no cloud" versus "works offline" for this?
18. Have you heard of cc-lens? Tried it? What stopped you?
19. What would make you switch from cc-lens or any dashboard you currently use?
20. If you could get only ONE insight from your agent history, what would it be?

## 6. Ten questions to detect politeness

1. "If this were free today, would you install it right now?" → "Yes, but..." = polite.
2. "What specific task would you use this for in your next session?" → Vague = not real.
3. "When did you last have this problem?" → "Sometimes" = low urgency.
4. "Have you tried to solve this already, even with a spreadsheet or script?" → No = low pain.
5. "Would you pay $5/month for this?" → "$5 is fine" but no usage = polite.
6. "Can I send you a build and you try it for a week?" → "Sure" without follow-through = polite.
7. "Who else on your team has this problem?" → Can't name anyone = niche pain.
8. "What would make you delete it after one use?" → Can't answer = hasn't thought about it.
9. "How does this compare to what you do now?" → "It's interesting" = no comparison.
10. "Would you introduce this to your company Slack?" → Hesitation = no social proof value.

## 7. Success criteria

| Evidence | Threshold |
|---|---|
| Qualitative | 5+ users describe a specific pain in their own words without prompting. |
| Quantitative | 30+ target users join waitlist from organic posts. |
| Intent | 5+ users say "send me a build" and actually install it. |
| Retention | 3+ users open the tool twice in one week without us asking. |
| Differentiation | 3+ users name a reason they'd use this over cc-lens. |

## 8. Kill criteria

| Signal | Action |
|---|---|
| < 10 waitlist signups after 5 posts | Angle wrong or market absent. |
| 0 users can name a specific use case | This is a "nice idea," not a problem. |
| Users care more about privacy than insights | cc-lens already wins; no wedge. |
| No one wants to install a local-history reader | Trust/install friction too high. |
| All interest comes from "fun cards" but no repeat use | Novelty-only product. |

## 9. Fastest way to get 30 target users

1. **X:** Post 1 variant per 6 hours. Reply genuinely in threads.
2. **Reddit:** Post in r/ClaudeCodeApp, r/cursor, r/programming, r/webdev, r/opensource.
3. **Discord/Slack:** Drop in AI-coding communities with feedback-first framing.
4. **Direct outreach:** DM 20 active Claude Code / Cursor users.
5. **Demo video:** 30-second screen recording of a fake card. No product needed.

## 10. What to build only after validation

Do **not** build before validation:
- Multi-agent adapters beyond Claude Code.
- PNG export server.
- Local LLM summarization.
- Paid tier.
- Persona cards.
- Cursor/Codex/Aider adapters.

Build **only** after validation confirms:
- Users want a specific insight type.
- They will install a tool that reads local history.
- They prefer this over cc-lens for a reason we can defend.

---

# Track C — Positioning Against cc-lens & Verdict

cc-lens: *"Local analytics dashboard for Claude Code. No cloud, no telemetry."*

## Direction evaluations

### 1. Spotify Wrapped for all your AI coding agents
- **Positioning:** A private, shareable retrospective for every AI coding agent you use.
- **Tagline:** "Your year in AI coding. Local-first."
- **Who it's for:** Devs using 2+ AI agents who like quantified-self and shareable dev memes.
- **Who it's not for:** Devs who only use one agent or hate performance theater.
- **Why different from cc-lens:** Multi-agent + shareable cards vs. Claude-only analytics.
- **First feature:** Weekly Wrapped card with model mix and top project.
- **First screenshot:** Dark Wrapped card showing "187 turns this week."
- **First launch post:** "Spotify Wrapped for your AI agents. No cloud."
- **Biggest risk:** Novelty wears off; users open once, share, delete.
- **Verdict:** Medium.

### 2. Debug loop detector for vibe coders
- **Positioning:** Find the repeated failure patterns your AI agent sessions hide.
- **Tagline:** "Stop looping. Start shipping."
- **Who it's for:** Vibe coders / AI-assisted devs stuck in 20-turn spirals.
- **Who it's not for:** Senior devs with strict workflows that rarely loop.
- **Why different from cc-lens:** Actionable failure diagnosis, not just stats.
- **First feature:** Highlight the file/edit that caused the longest loop this week.
- **First screenshot:** Red timeline with repeated edits on the same file.
- **First launch post:** "I analyzed 100 Claude Code sessions. The biggest waste was repeating the same fix 5 times."
- **Biggest risk:** Detection accuracy; false positives kill trust.
- **Verdict:** Strong.

### 3. Local AI coding coach
- **Positioning:** A private coach that reviews your agent sessions and improves your prompts.
- **Tagline:** "Get better at asking."
- **Who it's for:** Devs who want to level up their AI prompting.
- **Who it's not for:** Devs who just want quick answers and don't want homework.
- **Why different from cc-lens:** Prescriptive improvement, not descriptive stats.
- **First feature:** After each session, one concrete prompt rewrite suggestion.
- **First screenshot:** Side-by-side original vs. rewritten prompt.
- **First launch post:** "Your Claude Code sessions are full of teachable moments."
- **Biggest risk:** Coaching feels condescending or requires cloud LLM.
- **Verdict:** Weak.

### 4. Private project autopsy
- **Positioning:** After a project, automatically answer "where did all that agent time go?"
- **Tagline:** "Autopsy every build."
- **Who it's for:** Freelancers, indie hackers, small-team leads billing projects.
- **Who it's not for:** Large teams with existing project management tooling.
- **Why different from cc-lens:** Project-level narrative, not dashboard metrics.
- **First feature:** One-page autopsy: files touched, loops, final outcome.
- **First screenshot:** Project autopsy report with time spent per concern.
- **First launch post:** "I just shipped a feature. My agent spent 60% of the time on one file."
- **Biggest risk:** Users don't do post-project reviews.
- **Verdict:** Medium.

### 5. Funny wrapped / persona cards
- **Positioning:** AI-generated developer persona cards from your agent usage.
- **Tagline:** "Are you the Architect or the Looplord?"
- **Who it's for:** Devs who love GitHub Wrapped and Spotify Wrapped.
- **Who it's not for:** Devs who find performance memes cringe.
- **Why different from cc-lens:** Emotional + social value, not analytical.
- **First feature:** Generate one persona card per week.
- **First screenshot:** Card with nickname and funny stats.
- **First launch post:** "I built AI Coding Wrapped. My persona is 'The Yak Shaver.'"
- **Biggest risk:** Pure novelty; zero retention.
- **Verdict:** Weak.

### 6. Multi-agent usage mirror
- **Positioning:** One dashboard for every AI agent you use.
- **Tagline:** "See all your agents at once."
- **Who it's for:** Power users running Claude, Codex, Cursor, Aider in parallel.
- **Who it's not for:** Devs committed to one agent.
- **Why different from cc-lens:** Coverage breadth vs. Claude depth.
- **First feature:** Aggregate usage across two agents.
- **First screenshot:** Bar chart comparing agents by messages/tokens.
- **First launch post:** "I use 4 AI agents a week. I had no idea which was actually helping."
- **Biggest risk:** Multi-agent power users may be a tiny minority.
- **Verdict:** Medium.

### 7. Personal AI coding memory
- **Positioning:** Searchable memory across all your agent conversations.
- **Tagline:** "Never re-explain your codebase."
- **Who it's for:** Devs working across many projects who want long-term agent memory.
- **Who it's not for:** Devs who start fresh per project.
- **Why different from cc-lens:** Memory/recall, not analytics.
- **First feature:** Search past agent sessions by topic.
- **First screenshot:** Search bar + results from old sessions.
- **First launch post:** "Your AI agent has amnesia. I fixed it locally."
- **Biggest risk:** Cursor memory, Claude projects already solve this.
- **Verdict:** Medium/Weak.

## Strongest 2

1. **Debug loop detector for vibe coders** — strongest wedge.
2. **Multi-agent usage mirror** — strong if validation proves audience exists.

## Arguments against building

| Argument | How to test | What disproves it | Wedge that survives |
|---|---|---|---|
| Market too small | Poll agent-stack usage | 30+ signups in 48h with multi-agent stack | Dominate the niche; loop detector wins small market |
| Users don't care about retros | Ask "when did you last review an old session?" | 5+ users describe wanting to optimize | Shift to real-time loop interrupt |
| Existing tools enough | Survey cc-lens dissatisfaction | Users name concrete cc-lens gaps | Be the actionable coach layer on top |
| Data formats unstable | Parse Claude sessions from 3 versions | Schema stable for 2-week MVP | Adapter abstraction isolates breakage |
| Privacy concerns | Ask if they'd install history reader | 5+ users install and use | More local, auditable, no telemetry |
| Hard to support many tools | Build Claude adapter | 80% of users only need Claude + one | Start with one tool, be best there |
| Low retention | Track re-opens in first week | 50% of installers open twice in 7 days | Weekly local-only ritual |
| Novelty wears off | Compare card vs. loop interest | Users ask for actionable features more | Kill persona; double loop detection |
| Hard to monetize | Ask pricing questions | 3+ users say they'd pay | Open core + paid team features |
| Devs won't install | npx-ability test | 5+ non-friends install in 60 seconds | Single binary or Homebrew tap |

## Final verdict

> **Build tiny MVP — but only as a validation vehicle.**

**Specific instructions:**
1. Do not build the full architecture. Strip to: one CLI, Claude Code only, one loop detector insight, one shareable card.
2. Spend the first 24 hours validating, not coding.
3. Build the tiny demo on Day 2 only if you see 10+ waitlist signups or 3+ users naming the loop pain unprompted.
4. If validation fails: Kill or pivot to multi-agent mirror and re-test.
5. If validation succeeds: Commit to 1-week MVP focused solely on loop detection.

**Bottom line:** The differentiated wedge is not "Wrapped cards" — it's **interrupted loops**. Cards are marketing. Loop detection is the product.
