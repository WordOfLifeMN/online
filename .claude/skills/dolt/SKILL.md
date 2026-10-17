---
name: dolt
description: Lightweight prompt → questions → plan → implement workflow for a change described by a prompt.md in openspec/changes/<date>_<name>/. Use when the user asks to plan a change from a prompt file, says they've answered the open questions, asks to implement a plan.md, or invokes /dolt.
argument-hint: "[change directory or name fragment] [plan|implement]"
---

# dolt

A lightweight development loop. The user writes what they want in a `prompt.md`. You turn it into an agreed `plan.md` through rounds of written questions, and later, usually in a fresh session, implement that plan.

The name is an old joke: type "DoIt" in a sans-serif font, where a capital I looks like a lowercase l, and it reads as "dolt".

## The change directory

Each change lives in `openspec/changes/<YYYY-MM-DD>_<kebab-name>/`:

| File | Written by | Purpose |
|---|---|---|
| `prompt.md` | user | Goal, requirements, out of scope, and any testing specific to this change |
| `questions-01.md`, `questions-02.md`, … | you, answered by the user | One file per round of open questions |
| `plan.md` | you | The agreed, self-contained plan, used for implementation |

Start a new change from [prompt-template.md](prompt-template.md).

Find the change directory from the argument: a path, or a fragment of a directory name under `openspec/changes/`. With no argument, use the change whose file the user has open in the IDE. If that isn't clear, ask.

If `prompt.md` contains its own instructions that conflict with this skill, follow the prompt for that change.

## Pick the stage

Look at what's in the directory, unless the user named a stage:

1. **No questions files and no `plan.md`.** Start planning: go to **Round 1**.
2. **The newest `questions-NN.md` has unanswered questions.** Tell the user which ones are blank and stop.
3. **The newest `questions-NN.md` is fully answered.** Go to **Next round**.
4. **`plan.md` exists with unchecked tasks**, or the user said "implement": go to **Implement**.

## Planning (no code changes)

While planning you change no code. The only files you write are `questions-NN.md` and `plan.md` in the change directory.

### Round 1

1. Read `prompt.md`, then read enough of the codebase to ground the plan: the code being changed, its callers, the existing tests, and the conventions you should match.
2. List every ambiguity, contradiction, or decision the prompt leaves open. Include catches the user may not have seen, such as output that goes through `log` instead of stdout, or edge cases like very long values. Don't ask anything the code already answers. Settle it yourself and note it in the plan.
3. Write `questions-01.md` in the format below.
4. Tell the user how many questions there are, give one line per question, and ask them to fill in each `Answer:` and tell you when they're done. Then stop.

If there are no real ambiguities, skip the questions file. Say so, and go straight to **Write the plan**.

### Questions file format

```markdown
# Open questions: round 1

Fill in each `Answer:` with an option number, plus any notes. Tell Claude when you're done.

## Q1. <Short title>

<What is ambiguous and why it matters. Quote the prompt or point to the code (`path/file.go:123`) when that helps.>

1. <Option>  **(recommended)**
2. <Option>
3. <Option>

Answer:

## Q2. …
```

- Number questions across all rounds: if round 1 ends at Q7, round 2 starts at Q8. Then "Q9" always means the same question.
- Give 2–4 options, one per line, and mark exactly one as recommended. If one option clearly wins, still say what the alternative would be.
- Keep each question to one decision. Split compound questions.

### Next round

1. Read the answers in the newest questions file, including the notes on each answer, not just the option number. A note can add a requirement, change a chosen option, or point at a different question. Fold each such note into the plan, or ask about it in the next round if it's unclear.
2. Write `questions-NN.md` (the next number) only if something is still open: an answer that is unclear, one that conflicts with another answer, or one that raises a new decision. It contains only those follow-ups, each one referring back to the question it follows from (e.g. "Follows Q3").
3. When a round leaves nothing open, the rounds are done: go to **Write the plan**. The user can also end the rounds at any time ("go ahead with your recommendations"). In that case, use the recommended option for every blank answer and list those choices in the plan.

Never edit an answered questions file. They are the record of what was decided.

### Write the plan

Write `plan.md` so that a new session can implement it without reading `prompt.md` or the questions files. Every decision goes into the plan itself. Structure:

```markdown
# <Change title>

## Goals
<What this change achieves, and why.>

## Requirements
<The prompt's requirements with every answered question folded in, written as decided facts, not as Q&A. Tag a requirement with the question that settled it, e.g. "(Q4)", so the reason can be traced.>

## Out of scope
<What this change deliberately does not touch.>

## Architecture
<New and changed packages, types, and functions; how they fit together; and sample output or API shapes where they help. Name real files and identifiers.>

## Tasks

### Phase 1: <name>
- [ ] 1.1 <Task>. Files: `path/a.go` (`FuncName`), `path/a_test.go`
- [ ] 1.2 …
- [ ] **Checkpoint (you):** <what the user runs, and what they should see>

### Phase 2: …

## Test plan

### Automated (Claude)
- [ ] <Unit test or command, and what it proves>

### Manual (you)
- [ ] <Step the user performs, and the expected result>
```

Write tasks so that:
- each one names the files and functions it touches, and is small enough to finish and verify in one step;
- each phase leaves the code building and its tests passing;
- manual checkpoints sit at the end of the phase where something becomes visible to the user, not only at the very end.

Then tell the user the plan is ready to review. Suggest implementing it in a new session with `/dolt <change> implement`, and stop.

## Implement

1. Read `plan.md` and the code it names. Treat `plan.md` as the spec. Don't re-plan, and don't reopen settled decisions.
2. Work through the tasks in order. Change a task's `[ ]` to `[x]` as soon as it is done and verified, not in a batch at the end. The plan is how the user sees your progress.
3. At a **Checkpoint (you)** line, stop. Tell the user what to run and what to look for, and wait for their report. Tick the checkpoint only when they confirm it works. If they report a problem, fix it before moving on.
4. If the plan turns out to be wrong or incomplete, stop and ask. Don't quietly improvise. Once agreed, record the change in a `## Deviations` section at the end of `plan.md`, with the reason.
5. At the end, run the full automated test plan, tick those items, and hand the manual test plan to the user.
6. Never commit. The user reviews diffs in VSCode first. When everything is ticked, offer to commit and wait for their go-ahead.
