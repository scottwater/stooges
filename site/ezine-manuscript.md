# Stooges Four-Page Ezine Manuscript & Page Prompts

Use this manuscript to generate a **four-page ezine** for **Stooges**. The generation process may not have access to the Stooges source repository, website files, images, or CSS, so every page prompt is written to be self-contained.

Generate the ezine **one page at a time**. For each page, paste the **Master Style Prompt** first, then paste the specific page prompt.

---

## Master Style Prompt

Generate one portrait-oriented ezine page for a developer tool called **Stooges**.

The page must feel like part of a four-page set. Use a vintage silent-film/playbill style: dramatic title cards, early cinema texture, theatrical typography, typewriter command blocks, ornamental dividers, and high-contrast cream-on-black printing.

### Product summary

Stooges is a command-line tool for creating multiple independent Git workspaces.

It creates **copy-on-write clones** of a Git repository. These clones behave like full independent repositories but use near-zero extra disk space until files diverge.

Core message:

> Multiple independent copies of your repository — with near-zero disk overhead. No stashing. No conflicts. No nonsense.

Stooges is useful when a developer wants separate repo spaces for:

- AI coding agents
- long-running tests
- hotfixes
- pull request review
- branch experiments
- context switching without stashing

Stooges is an alternative to Git worktrees for people who want full repository isolation. Do not say Git worktrees are bad. Say they are great until shared Git internals — shared indexes, lock files, or tooling confusion — get in the way.

### Concepts to preserve

- The hidden `.stooges/` directory is the locked read-only base repo.
- Default workspaces are named `larry`, `curly`, and `moe`.
- Each workspace is a full independent Git repository with its own `.git`, index, locks, and branches.
- Copy-on-write cloning keeps the clones cheap: they share disk blocks until files diverge.
- Stooges supports macOS/APFS and Linux filesystems with reflink support.
- Installation and documentation should be referred to as: `stooges.dev`.

Do **not** include long installation URLs. This is a printed/visual ezine; readers should be directed to `stooges.dev` instead.

### Visual style

Use this palette:

- true black: `#0a0a0a`
- near black: `#1a1a1a`
- dark gray: `#2a2a2a`
- mid gray: `#666666`
- light gray: `#b0b0b0`
- cream: `#e8e0d0`
- off-white: `#f0ead6`
- warm white: `#f5f1e8`

Typography direction:

- Main display type: high-contrast serif, like Playfair Display, Georgia, or old theatre-poster lettering.
- Small caps / intertitles: old book or playbill small caps, similar to IM Fell English SC.
- Commands and technical captions: typewriter monospace, similar to Special Elite or Courier Prime.
- Use wide letter spacing for labels such as `THE PICTURE`, `FEATURE PRESENTATION`, `NOW SHOWING`, and `A TYPICAL SCENE`.

Required recurring motifs:

- black or near-black background
- cream/off-white text
- subtle film grain/noise overlay
- soft vignette around the page edges
- optional side film-strip sprocket borders
- thin ornamental dividers using symbols such as `✥ ✥ ✥`, `◆`, `✦`, or `✧`
- bordered playbill boxes with inset borders
- command snippets in black typewriter blocks with a `$` prompt
- page footer styled like a film credit card

### Illustration guidance

If real Larry/Curly/Moe artwork is unavailable, draw or imply three simple grayscale vintage cameo portraits or silhouettes. They should be labeled:

- `Larry` — `Workspace One`
- `Curly` — `Workspace Two`
- `Moe` — `Workspace Three`

The cameos represent workspace names. Do not rely on any external image files.

### Layout requirements

Each page should stand alone because it will be generated separately.

Use this hierarchy:

1. small production/intertitle label
2. large page title
3. concise subtitle or thesis line
4. one central visual, table, or diagram
5. short supporting copy blocks
6. footer with page number and `stooges.dev`

Keep the page readable. Use generous margins. Avoid dense paragraphs.

### Things to avoid

- Do not make it look like a modern SaaS landing page.
- Do not use bright colors, neon, glassmorphism, emoji, generic stock tech imagery, or futuristic dashboards.
- Do not imply Stooges modifies Git itself.
- Do not claim zero disk usage. Say **near-zero disk overhead** or **extra space only as files diverge**.
- Do not include a long install command or raw GitHub install URL.
- Do not mention needing access to the source repo, website files, or local assets.

---

# Page 1 of 4 — Cover / Title Card

## Goal

Introduce Stooges as a theatrical “feature presentation.” This page should sell the promise immediately and establish the silent-film visual identity.

## Page prompt

Generate **Page 1 of 4** for the Stooges ezine.

Make this page a full cover/title card.

### Required text

Small top line:

`SCOTT WATERMASYSK PRESENTS`

Main title:

`STOOGES`

Subtitle bar:

`GIT WORKSPACES, THE SMART WAY`

Tagline:

`Multiple independent copies of your repository — with near-zero disk overhead.`

Punchline:

`No stashing. No conflicts. No nonsense.`

Cast label:

`STARRING`

Cast cameos:

- `Larry` — `Workspace One`
- `Curly` — `Workspace Two`
- `Moe` — `Workspace Three`

Footer:

`Page 1 of 4 · stooges.dev`

### Visual composition

- Use a huge centered `STOOGES` title in an old movie-poster serif.
- Put `GIT WORKSPACES, THE SMART WAY` in a cream rectangle with black type, like a subtitle placard.
- Place three circular grayscale cast cameos below the title.
- Label the cameos Larry, Curly, and Moe.
- Use side film-strip sprockets if space allows.
- Add `✥ ✥ ✥` between the title area and cast area.
- Add visible but subtle film grain and a soft vignette.

### Copy tone

This is the opening credits page. Keep it iconic, sparse, and theatrical. Do not include command details yet.

---

# Page 2 of 4 — The Problem / Why It Exists

## Goal

Explain the pain: agents, test suites, hotfixes, and context switching all want independent repo copies. Compare plain copies, worktrees, and Stooges.

## Page prompt

Generate **Page 2 of 4** for the Stooges ezine.

### Required text

Small label above title:

`A DEVELOPMENT MELODRAMA IN THREE REPOS`

Page title:

`THE PICTURE`

Primary copy block:

> You run an AI coding agent in one workspace. A long test suite in another. A hotfix in a third. Each one needs room to work without stepping on the others.
>
> Plain copies waste disk. Git worktrees are built in, but share Git internals. Stooges gives each workspace its own full `.git`, its own index, its own locks, and its own branch.

Aside/callout:

> If Git worktrees already work for you, keep using them. Stooges is for the moments when shared `.git` behavior gets in the way.

Comparison table:

| Method | Disk Cost | Independent? | Tooling |
|---|---|---|---|
| Plain Copies | Full duplicate | Yes | You manage it |
| Git Worktrees | Shared `.git` | Shared index & locks | Built into Git |
| Stooges | Copy-on-write | Yes — full repo clones | Purpose-built CLI |

Small warning label:

`THE VILLAIN: SHARED LOCK FILES`

Footer:

`Page 2 of 4 · No stashing. No conflicts. No nonsense. · stooges.dev`

### Visual composition

- Put the title and primary copy in a bordered playbill frame with an inset border.
- Make the comparison table the central visual.
- Highlight the Stooges row with brighter cream/white text.
- Use typewriter styling for `.git`.
- Add theatrical divider symbols above and below the table.
- Maintain the black, cream, gray, film-grain look.

---

# Page 3 of 4 — How It Works / The Scene

## Goal

Show the workspace structure and explain copy-on-write in one glance.

## Page prompt

Generate **Page 3 of 4** for the Stooges ezine.

### Required text

Small label above title:

`COPY-ON-WRITE CLONES TAKE THE STAGE`

Page title:

`A TYPICAL SCENE`

Central directory diagram:

```text
myproject/
├── .stooges/    ← locked read-only base repo
├── larry/       ← independent clone workspace
├── curly/       ← independent clone workspace
└── moe/         ← independent clone workspace
```

Supporting copy:

> `stooges init` moves the original repo into `.stooges/`, locks it read-only, then creates workspace clones beside it.
>
> Each workspace is a complete Git repository. You can edit, commit, push, rebase, and run tests independently.
>
> Copy-on-write keeps the clones cheap: they share disk blocks with the base until files diverge.

Feature cards:

`REEL I — Copy-on-Write`  
Near-zero overhead. Extra space only as files change.

`REEL II — Fully Independent`  
Every workspace has its own `.git`, index, locks, and branches.

`REEL III — Built for Agents`  
Run multiple AI coding agents without repo collisions.

`REEL IV — Sync & Rebase`  
Keep workspaces current with a small set of commands.

Footer:

`Page 3 of 4 · Each workspace is a full Git repo. · stooges.dev`

### Visual composition

- Use a large bordered central diagram in monospace/typewriter style.
- Treat the directory diagram like a stage plan or old technical plate.
- Arrange the four reel cards around or below the diagram.
- Include small Larry, Curly, and Moe labels or cameo icons if space allows.
- Use ornamental divider `◆` above and below the diagram.
- Keep the page less text-heavy than Page 2.

---

# Page 4 of 4 — Commands / Try It / Closing Credits

## Goal

Give the reader enough commands to understand how Stooges is used, then direct them to `stooges.dev` for installation and full documentation. End like a film credit card.

Do **not** include the full install URL. Do **not** include a long curl command. This page should be useful in print: the reader should remember the site, not copy a long command by hand.

## Page prompt

Generate **Page 4 of 4** for the Stooges ezine.

### Required text

Small label above title:

`NOW SHOWING IN YOUR TERMINAL`

Page title:

`THE FULL REPERTOIRE`

Intro line:

`The commands are short. The workspaces are independent. The base stays protected.`

Command scenes:

Scene `I.` — `Check the projector`

```bash
$ stooges doctor
```

Caption:

`Confirm Git and copy-on-write support.`

Scene `II.` — `Initialize the stage`

```bash
$ stooges init
```

Caption:

`Create the locked base repo and default workspaces.`

Scene `III.` — `Add a workspace`

```bash
$ stooges add auto-cd -b
```

Caption:

`Create a new workspace and branch.`

Scene `IV.` — `Branch, fork, track, or review a PR`

```bash
$ stooges branch scott/auto-cd
$ stooges fork scott/auto-cd
$ stooges track feature/shell-init
$ stooges pr 37
```

Caption:

`Spin up workspaces for real development flows.`

Scene `V.` — `Keep the cast current`

```bash
$ stooges sync
$ stooges rebase
$ stooges clean
$ stooges list
```

Caption:

`Sync the base, rebase workspaces, prune refs, and inspect the set.`

Optional shell integration note:

`Optional shell integration can auto-cd into new workspaces after add, branch, fork, track, or pr.`

Installation / documentation callout:

`Install Stooges and read the full guide at stooges.dev.`

Closing credits:

- `Written in Go`
- `Requires macOS/APFS or Linux with reflink support`
- `stooges.dev`
- `FIN`

Footer:

`Page 4 of 4 · No stooges were harmed in the making of this software.`

### Visual composition

- Use stacked command cards with typewriter styling.
- Make `stooges.dev` the most prominent callout near the bottom.
- Do not show a long install command.
- End with a centered `FIN` in large serif type.
- Make the footer feel like silent-film credits.

---

# Final Quality Checklist

Before accepting each generated page, verify:

- It matches the vintage silent-film/playbill style.
- It is self-contained and does not depend on local source files or website assets.
- Text is readable at print and screen size.
- Commands are exact.
- `copy-on-write` and `near-zero disk overhead` are used accurately.
- Workspaces are described as full independent Git repositories.
- The page does not include a long install URL.
- The page points readers to `stooges.dev` for installation and documentation.
- The design does not look like a generic modern SaaS landing page.
- The page number is correct for the four-page edition.
