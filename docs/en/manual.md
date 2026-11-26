# lagent User Manual

This manual is for installing lagent, using it day to day, and setting it up
to suit your environment. Read it from the start and you can begin using
lagent; once you know your way around, use the contents to look up just the
chapter you need.

This manual is updated in the same commit as the code in the same repository.
You can check the version you have with `lagent --version`.

Japanese: [manual.ja.md](../ja/manual.ja.md)

## How to read this manual

| Reader | What to read |
|---|---|
| Using it for the first time | Part 1 (chapters 1–3), in order |
| Want to check everyday operation | Part 2 (chapters 4–10) |
| Want to add MCP servers or skills, change settings, or choose a different model | Part 3 (chapters 11–16) |
| Want to know what is protected and what is not | Part 4 (chapter 17) |
| Want to look something up by the name of a command or setting | The appendices |

Conventions:

- "You" is the person operating lagent, "the model" is the local language
  model lagent calls, and "the LLM server" is the program that runs that
  model (LM Studio, for example).
- A line starting with `>` is something you type into lagent's input box. A
  line starting with `$` is a command you type in the shell.
- The screen examples are copied from the real display, but long paths and
  the like are shortened.

## Contents

**Part 1 Getting started**

1. [What lagent is](#1-what-lagent-is)
2. [Installing](#2-installing)
3. [Your first session](#3-your-first-session)

**Part 2 Using it**

4. [The screen and the keys](#4-the-screen-and-the-keys)
5. [Giving the model material](#5-giving-the-model-material)
6. [Answering approvals](#6-answering-approvals)
7. [Balancing approval against safety](#7-balancing-approval-against-safety)
8. [Continuing and cleaning up sessions](#8-continuing-and-cleaning-up-sessions)
9. [Working in directories outside the project](#9-working-in-directories-outside-the-project)
10. [One-shot runs and pipelines](#10-one-shot-runs-and-pipelines)

**Part 3 Setting it up**

11. [Project instruction files and trust](#11-project-instruction-files-and-trust)
12. [Connecting MCP servers](#12-connecting-mcp-servers)
13. [Using skills](#13-using-skills)
14. [Building in your own rules with hooks](#14-building-in-your-own-rules-with-hooks)
15. [Configuration reference](#15-configuration-reference)
16. [Tuning the LLM server and the model](#16-tuning-the-llm-server-and-the-model)

**Part 4 The safety model**

17. [The safety model and its limits](#17-the-safety-model-and-its-limits)

**Appendices**

- [Appendix A Commands and flags](#appendix-a-commands-and-flags)
- [Appendix B Slash commands](#appendix-b-slash-commands)
- [Appendix C Built-in tools](#appendix-c-built-in-tools)
- [Appendix D Files and locations](#appendix-d-files-and-locations)
- [Appendix E Environment variables](#appendix-e-environment-variables)
- [Appendix F Troubleshooting](#appendix-f-troubleshooting)
- [Appendix G Glossary](#appendix-g-glossary)
- [Appendix H Differences from gem-agent](#appendix-h-differences-from-gem-agent)
- [Appendix I Where to learn more](#appendix-i-where-to-learn-more)

---

# Part 1 Getting started

## 1. What lagent is

### 1.1 What the tool does

lagent is an AI agent that runs in the terminal. You start it in the
directory you want to work in and ask, in plain sentences, for what you want
done. The model then reads the files in that directory, edits them if
needed, runs commands, and carries the work forward.

The model is called from **an LLM server running on your Mac** (LM Studio,
mlx-serve, Ollama and so on). Neither the content of the conversation nor the
contents of the files the model reads are sent to any outside service.

It has four characteristics.

- **Nothing leaves the machine.** It is a tool for work that cannot go to a
  cloud API, work in environments with no network connection, and work you
  do not want to pay for per token.
- **It always checks before a change.** Operations such as rewriting a file,
  changing something with a command, or calling an external tool are shown
  to you before they run, and wait for your approval.
- **It does not leave the directory it was started in.** File reads and
  writes are limited to the directory it was started in (the project), and
  shell commands run inside the macOS sandbox, so they cannot write outside
  the project.
- **It uses the configuration you already have.** It loads a project's
  `AGENTS.md` and `CLAUDE.md`, and its Claude Code-format `.mcp.json` and
  skills, without their being rewritten for lagent.

### 1.2 How it works

```mermaid
flowchart LR
    U["You"] -->|"requests and approvals"| A["lagent"]
    A -->|"the conversation and tool results"| M["a model on the local LLM server"]
    M -->|"answers and tool calls"| A
    A --> F["reading and writing files"]
    A --> S["shell commands (inside the sandbox)"]
    A --> C["MCP servers"]
```

One request proceeds in the following order.

1. You type a request.
2. lagent sends the conversation so far, and the request, to the model.
3. The model either answers in text or replies "I want to call this tool
   with this content". A tool is an individual operation such as reading a
   file, editing a file, or running a command.
4. lagent examines the call. A read-only operation runs on the spot. An
   operation that changes something is shown to you, and waits for your
   approval.
5. The result is returned to the model. The model reads it and either makes
   the next call or puts together its answer.
6. Steps 3–5 repeat until the model has finished answering in text.

The model itself touches neither files nor commands. It is always lagent
that executes, and it is lagent's rules and you that decide whether
something may run.

### 1.3 Words to know first

| Word | Meaning |
|---|---|
| Project | The directory lagent was started in. File reads and writes are limited to it |
| Session | One conversation, from start to exit. It is recorded and can be resumed later |
| Turn | From your sending one request until the model has finished answering |
| Round | One call to the model within a turn. A turn usually takes from a few rounds to a few dozen |
| Tool | An individual operation the model can call (`read_file`, `edit_file`, `shell_exec` and so on) |
| Approval | Your allowing a tool call that changes something, before it runs |
| Lane | The level of privilege given to a shell command. There are three: `read` (read only), `write` (write to the project) and `operator` (only you can allow it) |
| Sandbox | The macOS mechanism that confines shell commands. The OS enforces the limits of each lane |
| Work directory | A place outside the project, prepared for each session. Intermediate files and large results go there |
| Instruction file | A file of instructions to the model, such as `AGENTS.md` or `CLAUDE.md` |
| MCP server | A way to add external tools to lagent (Model Context Protocol). Servers provide search, databases, browser control and so on |
| Skill | A directory that packages a procedure. The model loads it when it is needed |

The terms are collected in [Appendix G](#appendix-g-glossary).

### 1.4 Before you use it

- **A local model is weaker and slower than a large cloud model.** On
  involved work it makes mistakes, or stops partway, more often. Asking for
  work in small pieces and checking the content in the approval dialog
  matter even more than they do with a cloud model.
- **There is waiting.** The model takes time to read what it is sent from
  the beginning. The first response of a session, and the response right
  after the conversation has been summarised, can take tens of seconds to
  several minutes. From the second time on, when a continuation of the same
  content is sent, it is faster ([16.4](#164-shortening-the-wait)).
- **The size of the conversation it can handle at once is limited.** The
  size is set by the model and by your Mac's memory. For a large file,
  lagent automatically has the model read only part of it
  ([8.4](#84-long-conversations-and-compaction)).
- **It runs only on macOS on Apple silicon.** The safety model depends on
  the macOS sandbox, so there is no Linux version and no Windows version.
- **It does not have the following.** Web search and page fetching, reading
  PDF and Office documents, audio and video input, sending audit logs. If
  you need web search or the like, connect an MCP server that provides it
  ([chapter 12](#12-connecting-mcp-servers)).

The sister tool [gem-agent](https://github.com/nlink-jp/gem-agent), which
uses Gemini on Google Cloud, is used in the same way and suits larger work
([Appendix H](#appendix-h-differences-from-gem-agent)).

## 2. Installing

### 2.1 What you need

| What you need | Notes |
|---|---|
| A Mac with Apple silicon | It does not run on an Intel Mac. You need enough memory to hold the model you use |
| [Homebrew](https://brew.sh/) | Used to install. You can also use the archive from the releases page directly |
| An LLM server | One of [LM Studio](https://lmstudio.ai/), [mlx-serve](https://github.com/ddalcu/mlx-serve) or Ollama. Other servers with an OpenAI-compatible API work too |
| A model | One that supports tool calls ([16.2](#162-choosing-a-model)) |

No account and no credentials are needed.

### 2.2 Install

```sh
brew install nlink-jp/tap/lagent
lagent --version
```

If the version is printed, it is installed. What is distributed is a signed
binary notarised by Apple. If you do not use Homebrew, take the `darwin-arm64`
archive from the
[release page](https://github.com/nlink-jp/lagent/releases) and put the
`lagent` inside it somewhere on your `PATH`.

### 2.3 Prepare an LLM server

This section uses LM Studio as the example. For other servers, see
[chapter 16](#16-tuning-the-llm-server-and-the-model).

1. Install LM Studio and download a model. The one that has been tested is
   `google/gemma-4-26b-a4b-qat`.
2. Load the model. The "context length" you specify when loading it becomes
   the size of the conversation lagent can handle at once.
3. Start LM Studio's local server. By default it listens on
   `http://localhost:1234/v1`.
4. Check that the server responds. If it returns a list of models, it is
   running.

   ```sh
   curl http://localhost:1234/v1/models
   ```

### 2.4 Create the configuration file

The configuration file is `~/.config/lagent/config.toml`. The minimum
content is as follows.

```sh
mkdir -p ~/.config/lagent
cat > ~/.config/lagent/config.toml <<'EOF'
[llm]
provider = "lmstudio"
base_url = "http://localhost:1234/v1"
model    = "google/gemma-4-26b-a4b-qat"
EOF
```

- `[llm].provider` is the kind of LLM server. Write one of `lmstudio`,
  `mlxserve`, `ollama` or `openai` (any other OpenAI-compatible server).
- `[llm].base_url` is where the server's API is.
- `[llm].model` is the ID of the model to use. Write the name exactly as the
  server lists it (the `id` in the result of the `curl` in
  [2.3](#23-prepare-an-llm-server)). It has no default, and lagent does not
  start without it.

A template with a description of every key is in the repository as
[config.example.toml](../../config.example.toml). The list of keys is in
[chapter 15](#15-configuration-reference). If the configuration file contains a key
lagent does not know, lagent stops at startup and names it. A typing mistake
is never silently ignored.

### 2.5 Check that it works

In any directory, run it in the form that asks one question and exits.

```sh
mkdir -p ~/work/hello && cd ~/work/hello
lagent -p "What is the sum of 1 to 10?"
```

If an answer is printed, the installation is complete. The first response
can take a while. If it does not work, look at the following.

| Symptom | Cause and fix |
|---|---|
| An error saying it cannot connect (`connection refused`) | The LLM server is not running, or `[llm].base_url` is wrong |
| An error saying no model is specified | Write `[llm].model` |
| An error saying the model was not found | `[llm].model` differs from the name in the server's list |
| An error saying the context window is unknown | With `provider = "openai"`, write `[model].context_window` yourself ([15.3](#153-model--the-context-window)) |
| An error naming a key in the configuration file | The key name is mistyped. Fix the name shown |

Other symptoms are in [Appendix F](#appendix-f-troubleshooting).

### 2.6 Update and uninstall

```sh
brew upgrade nlink-jp/tap/lagent     # update
brew uninstall lagent                # uninstall
```

After uninstalling, the following two directories remain. Delete them yourself
if you do not need them.

| Directory | Contents |
|---|---|
| `~/.config/lagent/` | Configuration, MCP server definitions, skills, approval settings |
| `~/.local/state/lagent/` | Session records, memory, work directories |

### 2.7 Shell completion (optional)

If you want Tab to complete flags and subcommands, generate a completion script
and have your shell load it. In zsh, for example:

```sh
lagent completion zsh > "${fpath[1]}/_lagent"
```

Other shells are described in `lagent completion --help`.

## 3. Your first session

Now use it once for real. The screens in this chapter are from running it in
a small directory that contains only `AGENTS.md` and `notes.txt`.

### 3.1 Start in a project

Change to the directory you want to work in and type `lagent`.

```sh
cd ~/work/demo
lagent
```

The directory you start it in becomes the **project** of this session. File
reads and writes are limited to it, so the usual practice is to start at the top
of the repository you want to work on. If you try to start somewhere broad, such
as your home directory or `/`, it explains that the scope is too broad and asks
whether you really want to start
([11.6](#116-starting-somewhere-too-broad)).

### 3.2 Answer the trust question

The first time you start in a directory, if the project has instruction files,
a `.mcp.json` or skills (`.claude/skills/`), it asks the following.

```
new project: /Users/you/work/demo
this project provides:
  AGENTS.md (loaded as your instructions)
trust this project? These files will be treated as YOUR instructions and its MCP servers will run. [y/N]:
```

The contents of an instruction file become "instructions from you" as far as
the model is concerned. The servers written in `.mcp.json` are started as
programs on your Mac. Trust only what you wrote yourself or have checked the
contents of.

- If you answer `y`, the listed files are loaded. The answer is recorded and it
  does not ask again.
- If you just press Enter (`N`), it starts without loading the project's files.
  In a repository you have only just fetched from somewhere else, it is safest
  to start this way first and look at the contents. This answer is recorded
  too, and it does not ask again. How to trust the project after you have
  looked is in [11.2](#112-the-trust-question).

How this question works in detail is in
[chapter 11](#11-project-instruction-files-and-trust).

### 3.3 Look at the screen

After startup, you see the following screen.

```
lagent v0.27.3 — qwen/qwen3.6-35b-a3b
instructions: AGENTS.md
project trust: 1 file(s) recorded as trusted: AGENTS.md

(the conversation scrolls here)

┃ message…  Enter send · Ctrl+J newline · /help · !shell
qwen/qwen3.6-35b-a3b · ctx –/262.1k · total 0 · ~/work/demo
```

- The few lines at the top are the startup banner. They show the version and
  the model, the instruction files that were loaded, and a note for anything
  you should be told about.
- The second line from the bottom is the input box. You type your request
  here.
- The bottom line is the status bar. It shows the model in use, the size of
  the conversation (`ctx`) and its limit, the number of tokens used so far
  (`total`), and the project directory.

### 3.4 Ask for something read-only

First, try a request that changes nothing.

```
> Tell me what is in this directory
```

The model calls a tool that lists files and a tool that reads a file. Each call
is shown as a line that starts with `⚙`.

```
⚙ list_tree (no arguments)
⚙ read_file path=notes.txt
```

Tools that only read run without asking you. At the end, the model's answer is
shown.

### 3.5 Ask for a change and approve it

Next, ask for a change to a file.

```
> Append a line saying "line 2" to the end of notes.txt
```

When the model tries to edit the file, the call and its purpose are shown in the
conversation area, followed by the approval dialog.

```
⚙ edit_file new_string=hello
line 2 old_string=hello path=notes.txt
  ↪ Append "line 2" to the end of notes.txt
```

```
╭──────────────────────────────────────────────────────────────
│ approval required: edit_file
│ ↪ Append "line 2" to the end of notes.txt
│ ┊ path=notes.txt
│ ┊ old_string=hello
│ ┊ new_string:
│ ┊   hello
│ ┊   line 2
│ ▶ allow (y)     deny (n)     deny with reason (N)     allow this session (a)     allow permanently (p)
│ ←→/Tab select · Enter confirm · y/n/N/a/p direct · Esc denies
╰──────────────────────────────────────────────────────────────
```

- The first line is the name of the tool it is about to call.
- The `↪` line is what the model wrote as "why I want to do this".
- The `┊` lines are the content of the operation. Here it is about to replace
  the part of `notes.txt` that reads `hello` with two lines, `hello` and
  `line 2`.
- Below that are the choices.

If the content is what you asked for, press `y`. The operation runs and the
result is shown.

```
  ↳ approved
Appended "line 2" to the end of notes.txt.
```

### 3.6 Approve a command

Try a request that runs a command as well.

```
> Make a directory called out and copy notes.txt into it
```

```
╭──────────────────────────────────────────────────────────────
│ approval required: shell_exec
│ ↪ Create the out directory and copy notes.txt into it
│ ┊ [write] mkdir out && cp notes.txt out/
│ ▶ allow (y)     deny (n)     deny with reason (N)     allow this session (a)     allow permanently (p)
│ ←→/Tab select · Enter confirm · y/n/N/a/p direct · Esc denies
╰──────────────────────────────────────────────────────────────
```

The `[write]` before the command shows that this command will run in the
**write lane**. Commands that only read, such as `ls` or `git status`, run in
the `[read]` lane, and those run without asking. Lanes are explained in
[6.4](#64-the-three-lanes-of-a-shell-command).

### 3.7 Deny and redirect

When what the dialog shows is not what you wanted, deny it. There are two ways
to deny.

- `n` simply denies. The model stops the operation and asks you what to do
  instead.
- `N` (capital) denies with a reason. A one-line input opens; write what you
  want instead.

```
│ deny reason:
│ > call it backup, not out
│ Enter send · empty Enter denies without a reason · Esc back
```

The reason reaches the model as written, and the model corrects the call as told
and proposes it again.

```
  ↳ denied — reason: call it backup, not out
⚙ shell_exec [write] mkdir backup && cp notes.txt backup/
```

### 3.8 Run a command yourself

Type `!` followed by a command in the input box and it runs on the spot, without
going through the model.

```
> !git status
```

It is a command you typed yourself, so it does not ask for approval. The command
and its output are passed to the model too, so you can follow up with "fix
this".

### 3.9 Quit and resume

Type `/quit` or press Ctrl+D to exit. On exit, it prints how to get back to
this session.

```
session 6badd651-8868-402b-b515-6c31891b177e — resume: lagent -c (or --resume 6badd651-8868-402b-b515-6c31891b177e)
4 rounds · prompt 25.4k · output 312
```

Type `lagent -c` in the same directory and it starts from where the last
session left off. The ID you pass to `--resume` can be just its first few
characters. The conversation and the tool results come back as they were
([8.2](#82-resuming)).

### 3.10 Where to go next

That is the end of the basics. Here are the chapters worth reading next.

- To learn the screen and the keys in full → [chapter 4](#4-the-screen-and-the-keys)
- To show the model files or images → [chapter 5](#5-giving-the-model-material)
- To be asked for approval less often → [chapter 7](#7-balancing-approval-against-safety)
- To use it from scripts → [chapter 10](#10-one-shot-runs-and-pipelines)

---

# Part 2 Using it

## 4. The screen and the keys

### 4.1 Three ways to run

lagent runs in three ways. Which one you get depends on how you start it.

| Way of running | How to start it | When it fits |
|---|---|---|
| Interactive (with the screen) | `lagent` in a terminal | Everyday work. This is the screen described in this chapter |
| Interactive (line by line) | `lagent` with its input connected to a pipe | Feeding in prepared input. There is no screen decoration, and it reads one line at a time |
| One-shot run | `lagent -p "request"` | Getting one answer and exiting. For scripts and pipelines ([chapter 10](#10-one-shot-runs-and-pipelines)) |

If you pass the request as an argument, as in `lagent "run the tests"`, it
starts interactively and sends that request as the first message. After that it
is an ordinary interactive session.

### 4.2 Screen layout

The interactive screen is made of three parts, from the top down.

- **The conversation area.** Your input, the tool calls and the model's
  answers flow from top to bottom. The terminal's ordinary scrolling and
  copying work as they are.
- **The input box.** It is fixed at the bottom of the screen. You can type
  while the model is working.
- **The status bar.** The single line at the very bottom.

The status bar shows the following, from the left. Anything that does not
apply is not shown.

| Shown | Meaning |
|---|---|
| `⚡auto` | Auto-approve mode is ON ([7.3](#73-auto-approve-mode)) |
| `🔒read-only` | Read-only mode is ON ([7.5](#75-read-only-mode)) |
| Model name | The model in use |
| `ctx 6.3k/262.1k (2%)` | The current size of the conversation, and the size the model can handle at once ([8.4](#84-long-conversations-and-compaction)) |
| `cache 87%` | The share of what was sent that the server did not have to re-read, because it continued from the previous request ([8.5](#85-usage)) |
| `total 12.5k` | The total number of tokens this session has used on the main exchange with the model |
| Directory | The project directory |

### 4.3 Typing

| What you want to do | How |
|---|---|
| Send | Enter |
| Break the line partway | Ctrl+J, or type `\` at the end of the line and press Enter |
| Recall what you sent before | ↑ / ↓ |
| Complete a file name or command name | Tab |
| Clear what you have typed so far | Ctrl+C |

- **A multi-line paste** becomes one message as it is. It is not sent line by
  line.
- **A newline with Option+Enter** works only when the terminal is set to send
  Option as the Meta key (in Terminal.app, "Settings → Profiles → Keyboard →
  Use Option as Meta key"; in iTerm2, "Settings → Profiles → Keys → Left Option
  key → Esc+"). Without that setting, both Option+Enter and Shift+Enter arrive
  as a plain Enter and the message is sent. Ctrl+J is the reliable one.
- **Tab completion** works on three things: a file path after `@`, a command
  that starts with `/`, and a skill name after `/skill `. With one candidate it
  completes it; with several it fills in the common part; and when it can go no
  further it lists the candidates.

A line you send is shown in the conversation area with `> ` in front of it. A
command run with `!` is shown with `! ` in front of it.

### 4.4 While the model is working

While the model is working, its progress is shown on one line above the
input box.

```
⠸ thinking… 1m07s · 34 chunks · last 0s  (Ctrl+C interrupts)
```

These are the elapsed time, the number of chunks of data received from the
model, and the number of seconds since the last one arrived.

- **The first wait is long.** The LLM server returns nothing until it has
  finished reading what it was sent. At the start of a session, right after
  the conversation has been summarised, and right after an MCP server's
  tools have been newly loaded, nothing may arrive for tens of seconds to
  several minutes. This is normal.
- **After 90 seconds with nothing arriving, it turns into a warning.** It
  shows `no data for 95s — the stream may be stalled`. The model may simply
  be reading a large input, so check on the LLM server's side, in its window
  or its logs, whether it is still working. If it has stopped, stop the turn
  with Ctrl+C.
- **A running view of the model's thinking.** When the server sends what the
  model is thinking, it is shown as dim text starting with `✦`. It is only
  displayed, and is not kept in the record. `[tui].show_thoughts = false`
  turns it off.
- **Retries.** When the server fails temporarily, lagent shows something
  like `retry 2/5 (503) — waiting 1s` and retries automatically. It tries up
  to 5 times.
- **Empty responses.** The model sometimes ends a response without saying
  anything. lagent then shows "the model returned an empty response —
  sending the same request again" and resends automatically, up to twice.

**Typing while it works.** If you type and press Enter while the model is
working, what you typed is **queued as the next message**. It is sent when
the current turn ends normally. If the turn ends with an error or an
interrupt, the queued text is not sent and goes back to the input box.
Commands starting with `!` or `/` cannot be queued. Stop the turn with
Ctrl+C first.

**Stopping.** Pressing Ctrl+C stops the running turn.

| Ctrl+C presses | What happens |
|---|---|
| First | Stops the turn. A running shell command is stopped together with the child processes it started |
| Second | Warns you if there is a tool that is not stopping |
| Third | Quits lagent. The conversation up to that point is already recorded |

After you stop a turn, the conversation up to that point is still there. You
can go on and give a different instruction. When nothing is running, Ctrl+C
clears the input box if it has content, and quits lagent if it is empty.

### 4.5 How answers are shown

The model's answers are rendered as Markdown. Tables, headings and code are
wrapped to the width of the terminal.

**Diagrams.** When an answer contains a diagram in mermaid notation, it is
drawn as a diagram on the spot.

| Terminal | How it is drawn |
|---|---|
| iTerm2, kitty | Drawn as a picture. Flowchart, sequence, ER diagram, pie chart, state diagram, Gantt chart, mind map. Japanese labels are drawn too |
| Others (Terminal.app and so on) | Drawn with box-drawing lines. Flowchart, sequence, ER diagram. Other kinds are shown in the original notation |

A diagram that cannot be drawn correctly is shown in the original notation,
with one line giving the reason. Inside tmux or screen, pictures are not drawn.

**Images.** In iTerm2, kitty and Ghostty, images can be shown inside the terminal.

- The model sometimes shows an image with a tool called `show_image`, for
  example when you have it make a chart.
- When you type `/show <path>`, it shows that image. The path may contain
  spaces, be wrapped in quotes, or be a file dragged onto the window.

It can show PNG and JPEG, up to 2 MiB. The mechanism it uses can be changed
with `[tui].images` ([15.9](#159-tui--the-screen)).

### 4.6 The notice when a dialog is waiting

When an approval or question dialog has been up for 6 seconds with no key
pressed, it rings the terminal bell once. Even if you are working in another
window, you notice that lagent is waiting for you.

- It rings once per dialog.
- If you are pressing keys, it is put off until the next 6 seconds. Just
  reading the dialog without pressing a key does not put it off, so it can
  ring while you are partway through reading something long.
- With `[tui].notify = "osc9"`, it also raises a desktop notification in
  iTerm2 and similar terminals. `"off"` makes it ring nothing.

### 4.7 Key reference

| Key | Where | What it does |
|---|---|---|
| Enter | Input box | Send. While the model is working, queue as the next message |
| Ctrl+J | Input box | Newline |
| ↑ / ↓ | Input box | Input history |
| Tab | Input box | Completion |
| Shift+Tab | Input box, while working | Toggle auto-approve mode. While a dialog is up, it moves between the choices |
| Ctrl+C | While working | Stop the turn |
| Ctrl+C | Input box | Clear the content. Exit if it is empty |
| Ctrl+D | Input box (when empty) | Exit |
| ←→ / Tab | Dialog | Move between the choices |
| Enter | Dialog | Confirm the selected answer |
| `y` `n` `N` `a` `p` | Approval dialog | Confirm that answer ([6.3](#63-the-five-answers)) |
| 1–9 | Question dialog | Confirm the choice with that number |
| Esc / Ctrl+C | Dialog | Deny, or decline to answer. While a dialog is up, Ctrl+C means this rather than stopping the turn |
| ↑↓ PgUp PgDn Home End | Approval dialog | Scroll long content |

### 4.8 Slash commands

A line in the input box that starts with `/` is a command to lagent, not a
request to the model. The commonly used ones are below. The full list is in
[Appendix B](#appendix-b-slash-commands).

| Command | What it does |
|---|---|
| `/help` | List the commands and keys |
| `/tools` | The available tools, and whether each one asks for approval |
| `/settings` | Show every setting together with where it was decided. Some can be changed on the spot |
| `/auto` | Toggle auto-approve mode |
| `/readonly` | Read-only mode. `/readonly on` and `/readonly off` switch it; on its own it shows the current state |
| `/usage` | The breakdown of tokens used in this session |
| `/compact` | Summarise the older part of the conversation to shorten it |
| `/clear` | Wipe the conversation and start a new session |
| `/quit` | Exit |

### 4.9 What is printed at startup

The lines printed at startup are kept to things you cannot find out anywhere
else.

```
lagent v0.27.3 — qwen/qwen3.6-35b-a3b
instructions: ~/.config/lagent/AGENTS.md, ../CLAUDE.md, AGENTS.md
mcp: 20 servers, 243 tools (/mcp)
```

| Line | Meaning |
|---|---|
| Line 1 | The lagent version and the model in use |
| `instructions:` | The instruction files that were loaded. They change the model's behaviour, so they are always shown |
| `mcp: …` | The number of connected MCP servers and tools. `/mcp` shows the details. Not printed when no server is connected |
| `resumed: …` | When a session was resumed |
| `auto-approve: ON at start` | When it started in auto-approve mode from the outset |
| `read-only: ON at start` | When it started in read-only mode from the outset |

When something is out of the ordinary, a line saying so is added. Where
there is a command for dealing with it, the line names it.

| Notice | Meaning |
|---|---|
| `sandbox: DISABLED`, `sandbox: enabled (read lane unverified …)`, `warning: sandbox unverified: …`, `warning: read lane disabled: …` | The sandbox is not in effect, or part of it could not be verified ([17.3](#173-checking-the-state-of-the-sandbox)) |
| `project trust: …` | The project's files were not loaded, or their content changed after you trusted them ([chapter 11](#11-project-instruction-files-and-trust)) |
| A line naming MCP servers | Some servers could not be started ([12.7](#127-when-something-is-wrong)) |
| `note: changed since your previous session: …` | Instruction files, the git configuration or the like have changed since last time ([17.2](#172-the-layers-of-defence)) |
| A line giving the size of the work directories | The work directories of past sessions exceed 10 MiB ([8.7](#87-work-directories)) |

### 4.10 Appearance and language

- **Colour.** Decided by `[tui].theme`. `auto` (the default) checks at startup
  whether the terminal's background is light or dark. If it does not suit,
  specify `dark` or `light`. In a terminal where nothing suits, `plain` turns
  off all colour and decoration.
- **Language.** Decided by `[tui].language`. `auto` (the default) looks at the
  environment variables `LC_ALL`, `LC_MESSAGES` and `LANG`, and uses Japanese
  if the value starts with `ja` and English otherwise. `ja` or `en` fixes it.
  What becomes Japanese is the dialogs, `/help` and the guidance messages. The
  output of `--help`, some of the startup lines, and the tool descriptions in
  `/tools` stay in English.

## 5. Giving the model material

### 5.1 Attach files and directories

Write `@` followed by a path in a message and that file is sent to the model
together with the request.

```
> @src/main.go fix the bug in this function
> @docs/ look at the list of documents here and name what is missing
```

- Pointing at a file sends its contents.
- Pointing at a directory (a trailing `/`) sends a listing of it.
- The path ends at the first space. For a file whose name has a space in it
  (a macOS screenshot, for one), rename it, or attach an image with
  `@clipboard`.
- The path can be completed with Tab.
- What can be attached is a file in the project, in the session's work
  directory, or in an added directory
  ([chapter 9](#9-working-in-directories-outside-the-project)). A reference that
  follows a symbolic link out of those is refused.
- A text file is sent up to 64 KiB each, and up to 256 KiB in total per
  message. These are upper limits: with a smaller context window they shrink
  to fit it (about 12 KB a file for a 32k-token model). Anything beyond that is cut off, and a note says so. For a large
  file, having the model read the part it needs suits better than attaching it.
- A directory listing has up to 200 entries.
- A reference that could not be attached is not silently dropped: you are told,
  with the reason.

Only the input you typed is scanned for `@`. If `@` appears in the model's
answer or in a tool result, nothing is attached.

### 5.2 Show an image

If you are using a model that can read images, you can show it screenshots
and diagrams as they are.

| Where the image is | How to write it |
|---|---|
| Inside the project | `@docs/mock.png reproduce this` |
| Anywhere on the Mac | `@~/Desktop/screenshot.png this part is wrong` |
| The clipboard | Capture a selected area to the clipboard (Cmd+Ctrl+Shift+4) → `@clipboard fix this display` |
| An image dragged into the terminal | Send the path the drag inserted, as it is. It is attached as an image even without `@` |

An image can be up to 8 MiB, and one message can carry up to 4. Anything over
the limit is not shrunk and sent; it is left unsent, and you are told. When
an image could not be attached, the model is told so too, so it does not
answer as though it had seen it.

When you are using a model that cannot read images, attaching one does not
get its content across ([16.2](#162-choosing-a-model)).

### 5.3 What cannot be attached

There is no feature for reading PDFs, Word, Excel or PowerPoint documents,
audio, or video. If you attach one of these files from inside the project
with `@`, it is treated just like a text file and the start of its contents
is sent as it is, so the content does not reach the model. For one outside
the project, the attachment is refused. To have the model read one, first
prepare a file that you have converted to text yourself, or connect an MCP
server that does the conversion.

### 5.4 Show the output of a command

The output of a command run with `!` is passed to the model too
([3.8](#38-run-a-command-yourself)).

```
> !make test
> fix the failing tests
```

A `!` command is one you typed yourself, so it does not ask for approval. It
runs inside the sandbox with the widest privilege (the `operator` lane). Its
time limit (120 seconds by default) is the same as for commands the model
runs. When the output exceeds 20,000 bytes, only the head and the tail reach
the model and the middle is left out. To have it read long output, write the
output to a file and have the model read that.

### 5.5 How attached material is treated

Files attached with `@`, the output of `!`, and content read by tools are all
passed to the model as **data**. Even if "follow these instructions" is written
in them, the model is built not to treat that as an instruction from you.

Only the following are instructions to the model.

- What you type into the input box
- Instruction files (`AGENTS.md` and so on)
- Skills

This distinction is why a document fetched from outside has a hard time taking
over the model when you have the model read it
([chapter 17](#17-the-safety-model-and-its-limits)).

## 6. Answering approvals

### 6.1 What asks and what does not

With nothing configured, tool calls are handled as follows.

| Operation | Approval |
|---|---|
| Reading, listing and searching files | Does not ask |
| Creating and editing files | Asks |
| Shell commands (read-only, the `read` lane) | Does not ask |
| Shell commands (writing to the project, the `write` lane) | Asks |
| Shell commands (the `operator` lane) | Always asks |
| MCP server tools | Asks |
| Saving and deleting memory | Asks |
| Reading credential files (`.env`, private keys and so on) | Always asks |
| Rewriting instruction files and configuration files (`AGENTS.md`, `.mcp.json` and so on) | Always asks |
| Making a directory outside the project available | Always asks |

The "Asks" rows can be reduced by configuration ([chapter 7](#7-balancing-approval-against-safety)).
The "Always asks" rows are answered by you yourself, every time, whatever
the configuration ([6.5](#65-what-only-you-can-answer)).

`/tools` shows which tools ask for approval under the current settings.

### 6.2 Reading the dialog

```
╭──────────────────────────────────────────────────────────────
│ approval required: shell_exec
│ ↪ To update the dependency packages
│ ┊ [write] npm install
│ ▶ allow (y)     deny (n)     deny with reason (N)     allow this session (a)     allow permanently (p)
│ ←→/Tab select · Enter confirm · y/n/N/a/p direct · Esc denies
╰──────────────────────────────────────────────────────────────
```

| Part | How to read it |
|---|---|
| `approval required: …` | The tool it is about to call |
| The `↪` line | The purpose of this operation, as written by the model. It helps you decide, but **it was written by the very model that proposed the operation**. Check whether the stated purpose is true against the content below. When none was written, it shows "(no purpose declared)" |
| The `┊` lines | The content of the operation. For a command, the whole command; otherwise, every argument. A long value is shown indented under its name |
| The `⚠` lines | Cautions. They tell you why the call came to you, that the content is long and part of it is out of view, and so on |

- When the content does not fit in the dialog, a line shows where you are
  looking now. Scroll with ↑↓, PgUp, PgDn, Home and End, and answer **after you
  have seen all of it**.
- An operation that rewrites an existing file in full shows the old size and
  the new size, as in `replaces existing file: 42KB → 8KB`. With a rewrite
  that shrinks the file a lot, check that content has not been lost.

**You can answer with an input method (IME) on.** With an input method (IME)
that is on, `y` and `n` can be swallowed by the conversion and never arrive. In
that case choose with ←→ or Tab and confirm with Enter.

### 6.3 The five answers

| Key | Answer | Scope |
|---|---|---|
| `y` | Allow | This one time only |
| `n` | Deny | This one time only. The model stops the operation and asks you what to do |
| `N` | Deny with a reason | This one time only. The reason you write reaches the model, and the model corrects the call accordingly |
| `a` | Allow this session | Until you exit lagent, calls to **the same tool** do not ask (this survives `/clear`) |
| `p` | Allow permanently | Written to the settings; calls to **the same tool** do not ask in later sessions either |

Esc is the same as deny (`n`).

When you use `a` and `p`, keep these two points in mind.

- **The unit of an allow is the tool.** If you answer `a` to `shell_exec`,
  every later `write` lane command runs without asking, whatever it contains.
  If you answer `a` to an MCP server tool, calls to that tool with different
  arguments do not ask either.
- **It does not apply to dangerous operations.** The operations listed in
  [6.5](#65-what-only-you-can-answer) ask every time, even after you have
  answered `a` or `p`.

An allow written with `p` can be reviewed and revoked in the `/settings` list
([7.2](#72-per-tool-policy)).

The answer selected at first is normally "allow". In a dialog with a `⚠` line
that says why it came to you, it comes up with "deny" selected: the operations
in [6.5](#65-what-only-you-can-answer), operations that came to you in
auto-approve mode, the question about lifting read-only, and so on. This is so
that a dangerous operation does not go through if you press Enter out of
habit. A tool you set to "always ask" (`always`) in the per-tool policy gets no
such line, so its question comes up with "allow" selected. A `⚠` that gives
the scroll position or the number of hidden lines is not a line of this kind.

### 6.4 The three lanes of a shell command

Shell commands are not judged safe or unsafe by looking at their text. For
each command the model declares "which privilege I want to run this with" (the
lane), and the macOS sandbox enforces the bounds of that privilege.

| Lane | What it can do | What it cannot do | Approval |
|---|---|---|---|
| `read` | Read files. Write to the scratch directory dedicated to the session | Write to the project, use the network, control other apps, read credential files, read external and network disks (`/Volumes`, `/Network`) and most of `~/Library` (if the project or an added directory is there, it can be read) | Does not ask |
| `write` | Write to the project, the work directory, the scratch directory and directories added for writing ([chapter 9](#9-working-in-directories-outside-the-project)). Use the network | Write to instruction files, configuration files, or the git configuration and hooks; read credential files; write anywhere else | Asks |
| `operator` | Everything `write` can, plus write to instruction files and the git configuration, and read credential files | Write anywhere `write` cannot | Always asks |

In the dialog and in the conversation, the command is prefixed with `[read]`,
`[write]` or `[operator]`.

Some things that are useful to know:

- **A false declaration gains nothing.** If a command that writes is declared
  as `read`, the OS refuses the write and the command fails. When it fails, the
  model is told which lane is needed and resubmits the command as `write`. That
  is when you are asked for approval.
- **More runs in the `read` lane than you might expect.** Besides `ls`, `cat`,
  `grep`, `git status`, `git diff` and `git log`, Go compilation, vetting and
  tests (`go build`, `go vet`, `go test`) also run in the `read` lane, because
  their working caches are pointed at the scratch directory (a build that
  writes an executable into the project is in the `write` lane). Caches for
  other languages can be pointed the same way
  ([15.4](#154-sandbox--the-sandbox)).
- **Some things are protected even in the `write` lane.** `AGENTS.md`,
  `CLAUDE.md`, `.mcp.json`, `.lagent.toml`, everything under `.claude/`,
  `.git/config`, `.git/hooks` and the like cannot be written from the `write`
  lane. These are files that can change the behaviour of the next session or
  have commands run outside the sandbox. For that reason `git init`,
  `git clone`, `git remote add` and `git submodule add` are in the `operator`
  lane.
- **In any lane, a command cannot touch your terminal.** A command cannot type
  characters into lagent's input box or answer an approval dialog.
- **`ps` and `top` do not work inside the sandbox.** This is a macOS
  restriction.

To be asked for approval on `read` lane commands too, set
`[sandbox].read_lane_prompts = true`.

### 6.5 What only you can answer

The following operations ask you every time: even after you have answered `a`
or `p`, even in auto-approve mode, and even when the per-tool policy says "do
not ask".

| Operation | Why |
|---|---|
| Dangerous command shapes. For example `sudo`, recursive deletion (`rm -rf`), `git push`, git operations that discard work or history (`git reset --hard`, `git clean -f` and so on), piping a download straight into a shell (`curl … \| sh`), writing directly to a disk (`dd`, `diskutil` and so on), a `chmod` that opens permissions wide, shutdown and reboot, registering something to stay resident (`launchctl`, `crontab`), copying to another machine (`scp`, `rsync`), and commands that name a place where credentials are kept | Cannot be undone, or the effect reaches outside the project |
| `operator` lane commands | They reach instruction files and credentials |
| Rewriting an instruction file or configuration file (`AGENTS.md`, `AGENT.md`, `CLAUDE.md`, `GEMINI.md`, `.mcp.json`, `.lagent.toml`, `.gem-agent.toml`, everything under `.claude/`) | What is written becomes the instructions and settings from the next session on |
| Writing under `.git/` with a tool that writes files | The git configuration and hooks are executed outside the sandbox by the next git command |
| Reading a credential file | The secret enters the conversation and its record |
| Writing outside the project, the session's work directory and directories added for writing | Out of scope |
| A request to make a directory outside the project usable | It widens the scope |
| A tool whose per-tool policy is "always ask" (`always`) | You decided so |
| Every shell command when the sandbox is not in effect | Nothing confines it |

For these operations themselves, it asks again the next time even if you
answer `a` or `p`. But an `a` or `p` answer does apply to that tool's other
calls. Answer `p` to a `git push` question, for example, and the write lane's
other shell commands run without asking, in later sessions too (see the
caution in [7.2](#72-per-tool-policy)). To allow just this operation, answer
`y`.

### 6.6 Credential files

The following files are treated as "credentials", and reading one asks you
every time.

- Places under your home directory where credentials are kept: `~/.ssh`,
  `~/.aws`, `~/.kube`, `~/.gnupg`, `~/.config/gcloud`, `~/.config/gh`,
  `~/.docker/config.json`, `~/.netrc`, `~/.npmrc`, `~/.git-credentials`, shell
  history, the keychains (`~/Library/Keychains`) and so on
- Files judged by name, wherever they are: `.env` (templates such as
  `.env.example` are excluded), the `id_rsa` family, `credentials.json`, and
  JSON files with `service-account` in the name

Read operations run in a separate process that is restricted so that it cannot
open these files. For a file whose name is not on the list, if the OS refuses
to open it, you are asked in the same way. Until you allow it, not one byte of
the content is read.

In file listings and searches, the names of credential files are still shown.
Files whose content was not searched are reported by name, as in
`[not read: sub/.env — reading one needs the operator's approval]`.

When you attach `@.env` yourself, that counts as your own permission and it is
read without asking.

A secret kept somewhere that is not on the list above (for example a token
written in a configuration file under `~/.config/<tool name>/`) is not
protected by this mechanism ([17.4](#174-what-is-not-protected)).

### 6.7 Questions from the model

When the model, partway through its work, wants you to choose something, it
raises a question dialog.

```
│ question: Which format should the output be in?
│ ▶ 1) A Markdown table
│   2) A CSV file
│ ←→/Tab select · 1-9 pick directly · Enter confirm · Esc declines
```

Pressing a number key confirms that choice. Esc means "decline to answer".
Declining is not an error: the model asks again in text, or explains its own
judgement and moves on. To tell it something that is not among the choices,
close the dialog with Esc and write it in the input box.

### 6.8 The check when a turn runs long

When the number of times the model has been called in one turn (rounds)
reaches `[agent].max_turns` (default 50), lagent lists the most recent tool
calls and asks you whether to continue.

```
│ question: round limit reached: 50 rounds used (hard cap 150)
│ recent calls:
│   read_file path=main.go
│   shell_exec [read] go vet ./...
│ . Continue?
│ ▶ 1) continue
│   2) stop here
```

- Look at the recent calls and judge whether the work is moving forward. If
  it looks as though the same call is being repeated, it is quicker to stop
  and give a new instruction.
- Choosing "continue" carries on for half of `max_turns` more (25 rounds by
  default) and then asks again. It can carry on up to 3 times `max_turns`.
- Choosing "stop here" ends the turn. The work up to that point is kept, so
  sending "carry on" resumes from where it stopped.
- When the model makes exactly the same call 3 times in a row, the same
  check appears without waiting for the limit ("possible loop").
- In a one-shot run there is no one to answer, so the turn stops at this
  check.

### 6.9 When an MCP server repeats the same error

When an MCP server tool returns the same error 3 times in a row within one
turn, lagent tells you as follows.

```
MCP server "bigquery": mcp__bigquery__execute_sql failed 3 times in a row with the same error — /mcp reload, or fix the server
```

At the same time it tells the model: "what you asked for is reaching the server
as it is. Do not hunt around for the cause; report to the operator and wait for
instructions." This prevents the model from starting a misguided investigation
over a temporary fault on the server's side. `/mcp reload` reconnects to the
server ([12.7](#127-when-something-is-wrong)).

## 7. Balancing approval against safety

With nothing configured, lagent asks at every change. That is safe, but on
long work it keeps bringing you to a halt. This chapter explains the means
of reducing how often it asks and, in the other direction, the means of
making it stricter.

### 7.1 The means at a glance

| Means | What it does | When it suits |
|---|---|---|
| `a` in the dialog | Stops asking about that tool for the rest of this session only | You want to hand things over just for the current piece of work |
| Per-tool policy ([7.2](#72-per-tool-policy)) | Decides in advance, tool by tool, "do not ask" or "always ask" | You want to use a read-only MCP server without being asked. You want to see certain tools with your own eyes every time |
| Auto-approve mode ([7.3](#73-auto-approve-mode)) | Runs, without asking, the operations the rules can judge safe. If configured, has commands reviewed by a separate model call | You want to hand over file edits |
| Risk rules ([7.4](#74-risk-rules)) | Tells the command review your policy in prose | You want the review's judgement to lean towards your own standards |
| Read-only mode ([7.5](#75-read-only-mode)) | Fixes the whole session at "change nothing" | Work that only investigates. Looking at a repository you do not know well |
| Hooks ([chapter 14](#14-building-in-your-own-rules-with-hooks)) | Your own script mechanically denies particular operations | You want a rule kept without leaving it to the model's judgement |
| MCP exclusion ([12.4](#124-remove-tools-you-do-not-use)) | Removes particular MCP servers, or tools within them, from the session | You do not want the model to see tools that can write in the first place |

Answering `a`, a per-tool policy of "do not ask" and auto-approve mode are means
of being asked less often. A per-tool policy of "always ask", read-only mode,
hooks and MCP exclusion are means of making it stricter. They can be combined. Examples of combinations
by situation are in [7.7](#77-combinations-by-situation).

### 7.2 Per-tool policy

For each tool name, you can decide one of the following in advance.

| Value | Meaning |
|---|---|
| `"never"` | Do not ask. In any mode, that tool runs without approval |
| `"always"` | Always ask. It asks every time, even in auto-approve mode and even after you have answered `a` in the dialog |

There are three ways to decide.

- **Answer `p` in the dialog.** That tool becomes `"never"`.
- **Change it in `/settings`.** Choose the tool under "approval policy" in the
  list and change the value with ←→. `s` switches between applying it to all
  projects and applying it to the current project only.
- **Write it in the configuration file.** Write it in
  `~/.config/lagent/config.toml`.

```toml
[approval.tools]
"mcp__tor-exit-lookup__*" = "never"    # do not ask for this server's tools
"shell_exec"              = "always"   # always ask for commands
```

A name can be written in two ways: the tool's name itself (`write_file`,
`mcp__<server>__<tool>`), or with a trailing `*` to mean a whole MCP server
(`mcp__<server>__*`). You cannot write just `"*"` to cover everything.

Changes made with `p` and in `/settings` are written to
`~/.config/lagent/policy.toml`. lagent manages this file. It does not
rewrite the `config.toml` you wrote by hand. When both hold a policy for the
same tool, `policy.toml` wins. Which file decides is shown at the right-hand
end of each row in `/settings`.

**What asks even with `"never"`.** The operations listed in
[6.5](#65-what-only-you-can-answer) ask even with `"never"`.

**A caution when setting `shell_exec` to `"never"`.** Every `write` lane
command, network use included, then runs without asking. The OS restricts only
"where it can write"; it does not restrict "what it does" or "what it sends
where". Dangerous command shapes (`sudo` and so on) and the `operator` lane
still ask, but nothing else is stopped. Use it only where you would be willing
to hand the work over in auto-approve mode.

**Bringing policy in from the project side.** Put a `.lagent.toml` at the
top of a project and you can write policy for that project only. How it is
treated, though, depends on the direction.

| Value written in the project's file | When it takes effect |
|---|---|
| `"always"` (more approval) | It always takes effect |
| `"never"` (less approval) | It takes effect only when that project is listed in `[approval].trusted_projects` in your `config.toml` |

This is because a repository fetched from somewhere else must not be able to
remove approvals just by the files it carries. A policy that did not take
effect is reported by name at startup. An example of the format is in the
repository as
[lagent.example.project.toml](../../lagent.example.project.toml).

When several policies match the same tool, the project's policy wins over the
global policy, and within each, one that spells the name out in full wins over
one that uses `*`.

### 7.3 Auto-approve mode

In auto-approve mode, among the operations that change things, **those that
fixed rules can judge safe** run without asking. For everything else, you
are asked as before.

**Switching.** The default is OFF.

| How | Scope |
|---|---|
| Shift+Tab, or `/auto` | The current session. You can switch in the middle of work too |
| `lagent --auto` | That start |
| `[agent].auto_approve = true` | Every interactive start (ignored in one-shot runs) |

While it is ON, the status bar shows `⚡auto`.

**Judgement by rules.** Operations that need approval are sorted into the
following three by fixed rules that do not use the model.

| Verdict | Example | In auto-approve mode |
|---|---|---|
| Safe | Creating and editing ordinary files inside the project, the work directory and the added directories | Runs without asking |
| Could be either | Commands in the `write` lane, MCP server tools, saving memory | Asks you |
| Always asks | The operations in [6.5](#65-what-only-you-can-answer) | Asks you |

In other words, what the default auto-approve mode saves you is **the
confirmation of file edits**. Commands and MCP server tools go on asking.

An operation that ran without asking carries its reason.

```
⚙ edit_file path=notes.txt …
  ↳ auto-approved (safe): edits a file inside the project
```

**Leaving commands to review as well (optional).** With the following
setting, commands in the `write` lane are judged by a separate model call
(the judge).

```toml
[approval]
model_tier = "shell"
```

- The judge judges a command twice, from two different angles. Only when
  both come out "approve, confident" does the command run without asking.
  Otherwise you are asked, with the judge's reason attached.
- It covers commands in the `write` lane only. MCP server tools, the
  `operator` lane and saving memory always ask you, even with this setting.
- What the judge sees is the command, the text of the request you typed, and
  the risk rules ([7.4](#74-risk-rules)). It is not shown
  the contents of files or the conversation history.
- Every review calls the model, so each one takes a few seconds.
- The judge's model can be set separately with `[llm].risk_model`. It must
  be a model on the same LLM server.

An operation that comes round to you has a `⚠` line in its dialog, which
tells you where it was stopped.

| `⚠` line | Meaning |
|---|---|
| `auto-approve blocked by rule (always asks): …` | An operation that, by rule, always asks you |
| `auto-approve escalated by risk review: …` | The judge did not approve it |
| `auto-approve escalated: …` | Passed to you without being reviewed (the `operator` lane, rewriting an instruction file, reading credentials, MCP server tools, saving memory) |

When auto-approve mode is OFF, the dialog for an operation that always asks
shows only the rule's reason (for example `⚠ recursive force delete`).

**What to know when you leave commands to review.** Commands in the `write`
lane can use the network. Once you leave them to review, the judge is the
only thing stopping a command that sends what was read to the outside. In a
measurement on one month of real approval prompts, about half of the
commands ran without asking, and not one command that carried data out or followed
planted instructions got through — but this is a matter of probability, not
a guarantee. On projects with confidential material, we recommend leaving
it OFF.

The sandbox stays in effect, unchanged, in auto-approve mode.

### 7.4 Risk rules

When you use command review (`[approval].model_tier = "shell"`), you can
tell the judge your policy in prose. Write it in ordinary sentences in
`~/.config/lagent/risk-rules.md`.

```markdown
- Installing packages from the network must always be checked by a person.
- Do not resolve or fetch a domain under investigation from this Mac.
- Running tests and formatting code may be approved.
```

- lagent reads this file at startup. It never rewrites it. What you change
  takes effect from the next start.
- It reads up to 4,000 characters. If the file is longer, it tells you and
  then cuts it.
- Risk rules **only lean the review's judgement**. They cannot let the
  operations in [6.5](#65-what-only-you-can-answer) through without asking.
  When you are not using review, they change nothing.
- They are not read from inside the repository. This is so that a
  repository you fetched cannot rewrite the review's standards in its own
  favour.

Things you want decided mechanically — "always ask about this", "never ask
about this" — belong in the per-tool policy ([7.2](#72-per-tool-policy)).

### 7.5 Read-only mode

Read-only mode fixes the whole session at "change nothing". It is a
separate axis from auto-approve mode. Auto-approve decides "who answers the
approval", whereas read-only decides "what this session can reach in the
first place".

**Switching.**

| How | Scope |
|---|---|
| `/readonly on`, `/readonly off` | The current session |
| `lagent --read-only` | That start |
| `[agent].read_only = true` | Every start. It applies to one-shot runs too |
| `lagent --writable` | Turns it OFF for that start only, when the configuration has it ON |

While it is ON, the status bar shows `🔒read-only`. lagent never turns it ON
or OFF on its own judgement. It is always you who switches it.

**What happens while it is ON.**

- Creating and editing files, commands in the `write` or `operator` lane,
  and saving memory are **denied without an approval dialog being shown**.
- Read operations, and commands in the `read` lane, work as before.
- When an operation that would be denied comes up, lagent asks instead
  whether to lift read-only.

```
│ Lift read-only?
│ ↪ create the out directory
│ ┊ [write] mkdir out
│ ⚠ this session is capped at the read lane, and the command declared write
│ Yes lifts read-only for the rest of this session.
│   allow (y)   ▶ deny (n)     deny with reason (N)
│ ←→/Tab to choose · Enter to answer · y/n/N · Esc refuses
```

This question is not "run this operation?" but "lift the mode?". If you
answer `y`, read-only is lifted for the whole of the rest of this session.
The operation itself then goes through ordinary approval afresh. If you
answer `n`, the operation is denied, and the same question is not repeated
for the rest of that turn.

**MCP server tools are not denied.** MCP servers run outside lagent's
sandbox, so lagent cannot tell whether a given tool only reads or also
writes. So while read-only is on, MCP server tools **ask you every time**.
An `a` answer and a `"never"` policy have no effect during this time.

**What read-only does not reach.** The commands you type with `!` and the
hooks you have configured are outside read-only. Both are your own
operations.

### 7.6 Leave the sandbox on

`--no-sandbox` and `[sandbox].enabled = false` turn the sandbox off. This is a
feature for investigating faults in the sandbox itself, not something to use in
everyday work. While it is off, every shell command asks you every time, under
any configuration. Neither auto-approve mode nor a `"never"` policy applies.

### 7.7 Combinations by situation

| Situation | Combination |
|---|---|
| Investigating a repository you do not know well | Answer `N` to the trust question and start with `lagent --read-only` |
| Everyday development | Leave the defaults. Set the read-only MCP servers you use often to `"never"` |
| Handing over file edits | Turn on auto-approve with `/auto`. Go on checking commands yourself |
| Wanting fewer command confirmations too | Set `[approval].model_tier = "shell"` and write risk rules. Do not use it on projects with confidential material |
| Not wanting an accidental change in the middle of an investigation | `/readonly on` |
| Making sure a rule is kept | Deny mechanically with a hook ([chapter 14](#14-building-in-your-own-rules-with-hooks)) |
| Running unattended from a script | See [10.3](#103-allowing-operations-that-change-things) |

## 8. Continuing and cleaning up sessions

### 8.1 The session record

lagent keeps a record of the conversation for each session, in one file.

- It is kept in
  `~/.local/state/lagent/sessions/projects/<per-project name>/`. Records
  and state are never placed inside the project directory.
- The record holds all of your input, the model's answers, and the tool calls
  and their results. **The contents of files the model read are in it too.**
  The file is created with permissions that let only you read it (`0600`).
- The record is appended to event by event. Even if lagent crashes partway,
  everything up to that point is still there.

### 8.2 Resuming

```sh
lagent -c                   # resume the last session in this directory
lagent sessions             # list the sessions that can be resumed
lagent --resume 2acb328c    # resume the given session (the first few characters of the ID will do)
lagent sessions --all       # list the sessions of every project
```

`lagent sessions` shows, newest first, the first 8 characters of the ID, the
date and time it started, the time since it was last used, the size of the
record, the model, and the first request.

```
f406db56  2026-11-17 00:09  just now  6KB  qwen/qwen3.6-35b-a3b  delete notes.txt
afc5de88  2026-11-17 00:08  just now  7KB  qwen/qwen3.6-35b-a3b  Append a line saying "line 2" to the end of notes.txt

resume with:  lagent --resume <id or its first characters>   (or --continue for the most recent)
```

A resumed session comes back with the conversation and the tool results as
they were, and appends what follows to the same record file. Things to know
about resuming:

- **It can be resumed only in the directory where it was recorded.** If you
  specify a session from another directory, it tells you so and stops.
- **It resumes with the same model it was recorded with.** To resume an
  earlier session after changing the model in the configuration, specify the
  original model with `--model`, as the message shows. To continue with a
  different model, start a new session.
- **Modes are not carried over.** Auto-approve and read-only are decided by
  the configuration and flags at the time you resume. Even if they were ON
  last time, they are OFF unless this start turns them on.
- **Directories added partway through a session are not carried over.** Add
  again whatever you added with `/add-dir` or `/read-dir`
  ([chapter 9](#9-working-in-directories-outside-the-project)).
- **The same session cannot be open twice at once.** If you try to resume a
  session that is already open, it tells you it is "in use" and stops. As long
  as the sessions are different, you can run any number at once in the same
  project.
- **The date given is today's.** The model is told that the session was
  resumed and the day the conversation began.

### 8.3 Starting over with /clear

`/clear` wipes the conversation and starts a new session. It is almost the same
as restarting lagent.

- The conversation up to then remains as a record and can be resumed later with
  `--resume`.
- The new session is given a new ID and a new work directory.
- MCP servers are reconnected.
- Memory is read again.
- Instruction files and skills are read again. One that belongs to the project
  and whose content has changed since you trusted it is not loaded, and you
  are told so ([11.3](#113-asked-again-when-content-changes)).
- Directories added with `/add-dir` and the like stay as they are.
- Allows given by answering `a` in a dialog, and the `/usage` totals, also
  stay as they are. These last until you exit lagent.

Use it when the subject changes, or when the conversation has grown long and
things have become slow.

### 8.4 Long conversations and compaction

There is a limit to the size of the conversation the model can handle at
once. `ctx 41.2k/262.1k (16%)` in the status bar is the current size of the
conversation and that limit. The limit is set by the settings used when the
model was loaded into the LLM server ([16.3](#163-setting-the-context-window)).

**Compaction.** When what is about to be sent looks set to exceed 80% of the
limit, lagent **replaces the older part of the conversation with a
summary**. The most recent exchanges stay as they are.

```
context is heading for 80% of the window — summarising the older part of the conversation (the next request re-reads the rest and may take a while)
compacted 42 earlier messages into a summary; 18 kept verbatim. Detail from the summarised part is now second-hand
```

- Both the summary and the first response after it take time. This is
  because the LLM server re-reads the content from the beginning.
- After compaction, the model knows the older part only through the
  summary. It does not remember exactly the contents of the files it read
  before the compaction either. It is often more reliable to `/clear` at a
  good stopping point and start afresh.
- If summarising fails twice in a row, automatic compaction is stopped for
  that session and you are told so. You can retry by hand with `/compact`.
- When the instruction files and the tool descriptions alone take up most of
  the limit, and summarising is not enough, you are told so. Reduce the MCP
  servers, or raise the context window.

**When the limit is unknown.** If lagent could not get the limit from the
LLM server at startup, it tells you "context window unknown (…) — set
[model].context_window to show the gauge", and the limit in `ctx` is shown
as `–`. In this state neither automatic compaction nor the truncation of
results described below works. Check the server and the model name and
start again, or write `[model].context_window`.

**Truncating large results.** Within one exchange, a tool result is
truncated so that it fits in the room that is left. When the conversation
limit is small and you have the model read a large file, only part of it
comes back, with how to read the rest attached. The model reads the rest in
pieces. A single exchange never overflows the conversation. A session that
got stuck on the verge of overflowing recovers if you send something once
more.

| What you want | How |
|---|---|
| Compact right now | `/compact` |
| Change the percentage at which compaction starts | `[agent].compact_at_pct` (default 80; 10–99) |
| Stop automatic compaction | `[agent].auto_compact = false` (`/compact` still works). `/settings` can switch it in the middle of a session too |

### 8.5 Usage

`/usage` shows a breakdown of the tokens used. The main exchange with the
model and the compaction summaries are shown separately. What command review
used does not appear here (it is kept in the session record). The figures
are running totals since lagent was started, and `/clear` does not reset
them to 0.

`total` in the status bar is the main exchange with the model. `cache 87%`
is the share of what was sent that the LLM server did not have to re-read,
because it continued from the previous request. lagent is built so that the first part of what it sends
each time is the same as the time before, so the higher this share, the
faster the responses. Note that a high share does not reduce the size of
the conversation (`ctx`). What makes the conversation smaller is compaction.

A local model costs nothing to run, but the number of tokens is a guide to
how much time things take. The record format is the same as gem-agent's, so
[gem-usage-lens](https://github.com/nlink-jp/gem-usage-lens) can total it.
By default gem-usage-lens reads gem-agent's records, so point it at lagent's
(`gem-usage-lens report --sessions-root ~/.local/state/lagent/sessions`).

### 8.6 Memory

Memory is short facts that you have the model remember across sessions. Use
it for things that are not written in any file but that you want known next
time too: decisions made, preferences, the quirks of the environment.

| Scope | Where it is recalled | Where it is stored |
|---|---|---|
| `project` | That project only | `~/.local/state/lagent/memory/projects/<per-project name>/<name>.md` |
| `global` | Every project | `~/.local/state/lagent/memory/global/<name>.md` |

**Saving it yourself.**

```
> /remember build-cmd Build this repository with make build. Do not use go build
> /remember global reply-lang Always write replies in Japanese
```

| Command | What it does |
|---|---|
| `/remember <name> <fact>` | Saves a memory for this project. The same name overwrites |
| `/remember global <name> <fact>` | Saves a memory that applies to every project |
| `/forget <name>`, `/forget global <name>` | Deletes a memory |
| `/memory` | Shows the saved memories and where the files are |

**The model proposes saving.** When the model notices something worth
remembering, it may propose saving it. An approval dialog appears when it
does. Even in auto-approve mode, saving memory does not go through without
asking. This is to prevent instructions planted in a document that was read
from settling in through memory. However, after you have answered `a` for
`save_memory`, or when it is set to `"never"`, it is saved without asking.

Some things to know:

- **Memory is loaded when a session starts and at `/clear`.** A memory you
  save takes effect from the next session.
- **Write it short.** One memory can be up to 2 KB, and all of them together
  up to 8 KB. Anything that did not fit is reported at startup.
- **Naming a file or a command works well.** Local models tend to overlook a
  rule written in the middle of a long instruction file. Putting a short
  pointer in memory, such as "read `PROCEDURE.md` before editing", makes the
  rule more likely to be kept.
- **The contents are plain Markdown files.** You can open them yourself at
  any time to fix them or delete them.

### 8.7 Work directories

For each session, one work directory is prepared outside the project. Things
that are not the project's deliverables go there.

- Intermediate files the model made
- Tool results and command output that were too large to carry in the
  conversation (the output of `operator` lane commands, of commands typed
  with `!`, and of commands run without the sandbox is not saved)
- Saved copies of the results MCP servers returned, and images

Its location is passed, as the environment variable `LAGENT_WORK_DIR`, to the
commands and MCP servers that lagent starts. `/dirs` shows it too.

A work directory with content in it is **not deleted automatically**. This is
so that you can look back at the results later. Delete them yourself when they
pile up (a work directory that nothing went into is cleared away on exit).

```sh
lagent workdirs              # list this project's work directories
lagent workdirs clean        # show what would be deleted, confirm, then delete
lagent workdirs clean <ID>   # delete only the given session's (the ID's first 4 or more characters will do)
```

- Before deleting, it shows the list of targets and their total size and asks
  for confirmation. Any answer other than `y` deletes nothing. When using it
  from a script, add `--yes`.
- The work directory of a session that is running now is not deleted.
- When the total of past work directories exceeds 10 MiB, you are told at
  startup.

### 8.8 Handling the records

The session records and the work directories hold the substance of your work
as it is. The record of a session in which you had it read a confidential file
needs the same handling as that file.

- To delete a record, delete the relevant file under
  `~/.local/state/lagent/sessions/projects/`.
- Check once whether `~/.local/state/lagent/` is included in what you back
  up or sync.

## 9. Working in directories outside the project

lagent can read and write only the project and the session's work
directory. When you want to work outside the project as well (a directory
where you keep notes, another repository next door, a place for reference
material), you **add** that directory.

### 9.1 Adding a directory

When you add one, you choose whether to make it writable or read-only.

| When | Make it writable | Make it read-only |
|---|---|---|
| At startup | `--add-dir <path>` | `--read-dir <path>` |
| Writing the ones you always use into the configuration | `[directories].write` | `[directories].read` |
| Partway through a session | `/add-dir <path>` | `/read-dir <path>` |

```sh
lagent --add-dir ~/notes --read-dir /Volumes/Archive/reference
```

```toml
[directories]
write = ["~/notes"]
read  = ["/Volumes/Archive/reference"]
```

- The flags can be repeated.
- `/add-dir` and `/read-dir` read the whole rest of the line as the path. A
  path containing spaces can be written without quotes. `~` is your home, and
  a relative path is relative to the project.
- `/dirs` shows what is added now. The project, the work directory and the
  added directories are listed, with whether each is writable and where it was
  added.
- What you add partway through a session stays in effect until you exit
  lagent (it survives `/clear`). To use it next time too, write it in the
  configuration or start with the flag.

**The model may also ask for one to be added.** When a file needed for the
work is outside the project, the model asks, with a reason, for the directory
to be added. You always answer this yourself. If you approve, it is added in
the form the model asked for (read-only or writable). If it asked for writable
but you want to add it read-only, deny, and type `/read-dir <path>` yourself.

### 9.2 What an added directory allows

| | Tools that read and write files | `read` lane | `write` lane | `operator` lane |
|---|---|---|---|---|
| Writable directory | Read and written like the project | Can read | Can write (except instruction files, the git configuration and the like) | Can write |
| Read-only directory | Can read. Writes are refused | Can read | Cannot write | Cannot write |

- **Configuration inside an added directory is not loaded.** An `AGENTS.md`,
  a `.mcp.json` or skills there are none of them loaded. They can only be read
  and written as files.
- **Credential files are treated the same inside an added directory**
  ([6.6](#66-credential-files)).
- **External disks and network locations can be added too.** They can be read
  even under `/Volumes` or under `~/Library`.
- **Cloud sync folders can also be added as writable directories.** That is,
  the folders of Google Drive, Dropbox, OneDrive and iCloud Drive.

### 9.3 Places that cannot be added

If you try to add one of the following places, it is refused with the reason.
When it was given by a flag, startup stops.

- `/`, the home directory, and any place that contains them (`/Users`, say)
- A place that contains the project or the work directory
- Somewhere credentials are kept, lagent's configuration or record
  directories, a place that contains one of them, or a place inside one
- Inside the shared temporary directories, or a place that contains them; a
  place that contains `/dev`
- `/Volumes`, `/Network` and `~/Library` themselves (directories inside
  `/Volumes` and `/Network` can be added)
- A place that contains the configuration file given with `--config` (as a
  writable directory)
- Anything under `~/Library` as a writable directory (except cloud sync
  folders)

If you type a path that goes through a symbolic link to `/add-dir`, nothing is
added and the real location is shown. Type that location afresh and it can be
added.

Of the paths written in `[directories]` in the configuration file, those that
do not exist at the moment (a disk that has been removed and so on), those
that contain the current project, and those already covered by something else
are skipped with a notice, without stopping startup. The same paths given with
a flag stop startup.

When an added directory becomes unusable partway through (the disk was removed
and so on), only that directory is dropped at the next `/clear`, and you are
told. After reconnecting, you can add it again with `/add-dir` or `/read-dir`.

### 9.4 Cautions

**Do not add, as a writable directory, the "parent" of directories that you
sometimes open as projects.** For example, if you keep your repositories side
by side under `~/work`, avoid adding `~/work` with `--add-dir`.

lagent reads instruction files by walking up from where it was started to
the parents. If a command creates an `AGENTS.md` inside a writable directory,
or moves in a directory that contains git hooks, a project later opened under
it will load that as instructions, or execute it.

lagent watches whether such files have been added or changed inside a
writable directory, and if there is a change it tells you at `/clear`, on exit,
and at the next start. But it only tells you; it does not stop it. When the
notice appears, check the files it names.

To verify that the sandbox takes effect correctly, lagent briefly creates a
small probe file in an added directory and deletes it at once. In a sync
folder, this can reach your other devices for a moment. If a file could not be
deleted, you are told its name.

## 10. One-shot runs and pipelines

### 10.1 Run once

With `-p`, it runs one request and exits.

```sh
lagent -p "explain the layout of this repository in 5 lines"
```

- The model's answer goes to standard output; everything else (the record of
  tool calls, notes, errors) goes to standard error. You can pass just the
  answer to a file or a pipe.
- There is nobody to answer, so **by default every operation that needs
  approval is denied**. A denied operation is written to standard error with
  the reason.
- Read-only tools and `read` lane commands work as they are.

### 10.2 Pipe data in

What you pass through a pipe is **attached as data**. It does not become the
text of the request.

```sh
git diff | lagent -p "review this diff"
cat error.log | lagent -p "name the lines that look like the cause"
```

Only the text of `-p` decides what is being asked. Even if "run the following
command" is written in what was piped in, that does not become an instruction.

- Up to 256 KiB is read. Anything beyond that is cut off, and the model is
  told that it was cut. The screen shows the number of bytes attached.
- Binary data is skipped, with a notice.
- It waits until the input is closed. If it is still not closed after 2
  seconds, it writes one line to standard error saying that it is waiting.

**When you pipe nothing in, add `< /dev/null`.** When it is started from cron
or another program, an input that nothing flows through stays connected, and
lagent keeps waiting for it to close.

```sh
lagent -p "tell me today's date" < /dev/null
```

### 10.3 Allowing operations that change things

There are three ways to have a one-shot run carry out operations that need
approval.

| How | What it does |
|---|---|
| Set the per-tool policy to `"never"` beforehand ([7.2](#72-per-tool-policy)) | Suits read-only MCP servers you always use. Once it is written in the configuration, no flag is needed |
| `--allow <name>` | For that run only, runs the named tools without asking. List them with commas, as in `--allow write_file,edit_file`, or repeat the flag. `mcp__<server>__*` covers a whole MCP server |
| `--auto` | For that run only, turns on auto-approve mode. The operations the rules can judge safe (file edits) run. With `[approval].model_tier = "shell"` set, commands in the `write` lane are reviewed and the ones that could be approved run. Everything else is denied instead of being passed to you |

```sh
# Let it use one MCP server for a read-only lookup
echo "203.0.113.5" | lagent -p "Look up this IP address" --allow "mcp__whois-lookup__*"

# Hand over rewriting a file as well
lagent -p "Fix the typos in the README" --allow edit_file
```

Whichever way you use, the operations in [6.5](#65-what-only-you-can-answer)
are not carried out. A one-shot run has no one to answer, so they are
denied. Denial by hooks stays in effect as well.

Shell commands are handled lane by lane, as follows.

| Lane | In a one-shot run |
|---|---|
| `read` | Runs with no flag |
| `write` | Needs `--allow shell_exec`. Or `--auto`, with command review configured |
| `operator` | Denied whatever the flag |

`--allow shell_exec` runs every command in the `write` lane, network use
included, without checking any of them. Do not use it in a run that reads
untrusted content.

`[agent].auto_approve = true` in the configuration file has no effect in a
one-shot run. The thinking is that whether an unattended run may change
things has to be written, and visible, in the command that starts it. A
denied operation is left on standard error as `[denied: …]`.

### 10.4 Piping your own input

If you connect a pipe without `-p`, the lines piped in are processed one by one
as **input you typed**.

```sh
printf '/usage\n' | lagent -c
```

In this form, a command in a line that starts with `!` is executed, and
commands that start with `/` take effect too. **Never feed text you received
from someone else, or data you fetched, in this form.** When you want to pass
content as data, always use `-p` ([10.2](#102-pipe-data-in)).

### 10.5 Common forms

```sh
# Have it investigate, with a guarantee that nothing changes
lagent --read-only -p "Explain what the scripts in this directory do" < /dev/null

# Run quickly, without starting any MCP server
git log --oneline -20 | lagent --mcp off -p "Summarise the recent changes in 3 lines"

# Take the answer into a file (the tool record stays on the screen)
lagent -p "Translate the latest CHANGELOG entry into Japanese" < /dev/null > latest.ja.md

# Use a different model for that run only
lagent --model <model ID> -p "List the weaknesses of this design" < /dev/null
```

- `--mcp off` starts no MCP server at all. Startup is faster. For a one-shot
  run this is usually enough.
- `--read-only` and MCP server tools cannot be used together. While
  read-only is on, MCP server tools are treated as asking you every time,
  and a one-shot run has no one to answer, so they are all denied. When you
  need MCP server tools, drop `--read-only`.
- The model given with `--model` has to be in a usable state on the LLM
  server's side.

### 10.6 Exit codes

| Code | Meaning |
|---|---|
| 0 | It ran to the end. Even if operations were denied, it is 0 as long as the model finished answering |
| 1 | It ended in an error, or was stopped partway |
| 130 | It was forced to exit by repeated Ctrl+C |

The exit code does not tell you whether an operation was denied. Look at the
`[denied: …]` lines on standard error.

### 10.7 Notes for scheduled runs

- **The LLM server.** At the time of the run, the LLM server has to be
  running and the model has to be in a usable state.
- **Input.** Always add `< /dev/null` ([10.2](#102-pipe-data-in)).
- **Long work stops partway.** In a one-shot run, the turn stops when the
  number of rounds reaches `[agent].max_turns`
  ([6.8](#68-the-check-when-a-turn-runs-long)).
- **Project trust.** In a project where you have not yet answered whether to
  trust it, the run goes ahead without loading that project's instruction
  files or `.mcp.json`. To have them loaded, start interactively once
  beforehand and answer, or, if it is your own project, list it in
  `[approval].trusted_projects`
  ([11.4](#114-projects-trusted-without-asking)).
- **Places that are too broad.** In the home directory or `/`, a one-shot
  run refuses to start. Move to the project directory and run it there.
- **Settings can be passed in environment variables too.**
  `LAGENT_PROVIDER`, `LAGENT_BASE_URL`, `LAGENT_MODEL` and so on
  ([Appendix E](#appendix-e-environment-variables)).

---

# Part 3 Setting it up

## 11. Project instruction files and trust

### 11.1 Which instruction files are read

lagent loads files with the following names as instruction files. Their
contents become "instructions from you" to the model.

| File | Original use |
|---|---|
| `AGENTS.md` | The name shared by various vendors' agents |
| `AGENT.md` | Its singular form |
| `CLAUDE.md` | Claude Code |
| `GEMINI.md` | Gemini CLI |

The places searched and the order of reading are as follows.

1. `~/.config/lagent/` (your own rules, which you want applied to every
   project)
2. The project's parent directories, from the outermost in (up to the home
   directory)
3. The project itself

The nearer a file is, the later it is read and the more specific an
instruction it is treated as. For example, if you write shared rules in
`~/work/AGENTS.md`, they are loaded whichever repository under `~/work/` you
start in.

- The walk up through the parents goes as far as the home directory.
  Instruction files in places you do not control, such as `/tmp`, are not
  picked up.
- Files with exactly the same content are loaded only once.
- The files that were loaded are listed on the `instructions:` line at
  startup.
- Up to 32 KiB is loaded per file, and up to 128 KiB in all. Anything beyond
  that is cut off or skipped, and you are told at startup.

### 11.2 The trust question

The following things that the project itself carries are loaded only after you
have answered that you **trust** the project.

| What the project carries | What happens when you trust it |
|---|---|
| Instruction files | They are loaded as instructions to the model |
| `.mcp.json` | The servers written in it are started on your Mac |
| Skills under `.claude/skills/` | The model becomes able to use them |

The project's `.lagent.toml`
([11.5](#115-the-per-project-configuration-file)) is always read, regardless of
this question. Policy that adds approval, and MCP exclusions, only ever work
in the tightening direction, so they take effect even in a project you have
not trusted.

The first time you start interactively in a directory, it lists what would be
loaded and asks ([3.2](#32-answer-the-trust-question)). The default
answer is "no".

- The answer is recorded per project in `~/.config/lagent/policy.toml`. It
  does not ask again.
- Even if you answer "no", lagent starts. It just does not load the
  project's own files; everything else works normally. It tells you so at
  startup.
- **To answer again**, open `policy.toml` and delete that project's
  `trust = "…"` line. It asks once more at the next start.
- **Instruction files in parent directories, and those in
  `~/.config/lagent/`, are outside this question.** They are loaded without
  asking, on the premise that they are your own. When you look at a repository
  you do not trust, start in its top directory. If you start in a subdirectory
  inside it, the `AGENTS.md` at the top of the repository is loaded without
  asking, as "an instruction file in a parent directory".
- In a start where there is nobody to ask, such as a one-shot run, a project
  you have not yet answered for runs without loading the project's files. No
  answer is recorded.

### 11.3 Asked again when content changes

Trust is given not to the name of a directory but to **the content of the
files**. When you answer that you trust it, the digest (SHA-256) of the content
of each file concerned is recorded. At every start it is compared with the
current content, and if it has changed you are asked before it is loaded.

```
AGENTS.md changed (1932 bytes) since you trusted it.
trust the new content? [y/N]:
```

When you bring in someone else's changes with `git pull`, when you switch
branches, when you edit it yourself: it asks in the same way each time. Files
newly added after you trusted the project (a new `CLAUDE.md`, a skill, a
`.mcp.json` and so on) are asked about in the same way. Files that have gone
are only reported.

- If you answer `y`, the new content is recorded as trusted.
- If you answer `N`, that file is not loaded this time. It asks again at the
  next start.
- Where it cannot ask, such as a one-shot run, `/clear` or `/mcp reload`, a
  changed file is not loaded and you are told so.
- A project's skills are compared only when the list of skills is built (at
  startup and `/clear`). A skill that changes partway
  through a session (after a `git pull`, say) is used as it now is until you
  do one of those.

To update the record after editing a file yourself, without waiting to be
asked, use the following commands.

```sh
lagent trust            # show the trust state, the recorded files and the changed files
lagent trust --accept   # in a trusted project, record the current content afresh
```

```
project: /Users/you/work/demo
trust: granted
trusted files: 1 recorded (2026-11-16T15:08:19Z)
  AGENTS.md                                unchanged
```

`--accept` can be used only in a project you already trust. You cannot use
this command to make trusted a project for which you have not yet answered the
trust question.

When a session is running, run `lagent trust --accept` in another terminal and
then type `/clear` in that session. The new content of instruction files,
skills and `.mcp.json` is loaded without a restart. `.lagent.toml` alone is
read only at startup, so for that file, restart.

When you approve the model editing a file in the record (an instruction file,
`.mcp.json`, `.lagent.toml`, or a file inside a skill) inside lagent, the
record for that one file is updated automatically. A file that had already
changed before the edit is not updated, and you are told so.

To stop this comparison and go back to trust per directory, set
`[approval].pin_trusted_files = false`.

### 11.4 Projects trusted without asking

For projects you manage yourself, write them in the configuration and it does
not ask the first trust question. The question when content changes
([11.3](#113-asked-again-when-content-changes)) still comes up as before.

```toml
[approval]
trusted_projects = ["/Users/you/work/my-repo"]
```

In a project written here, the following four things are allowed at once.

1. Instruction files are loaded
2. The servers in `.mcp.json` are started
3. The skills in `.claude/skills/` can be used
4. `"never"` in `.lagent.toml` (policy that **reduces** approval) takes
   effect (while `.lagent.toml` is unchanged since it was trusted)

The fourth is not allowed just by answering `y` to the question at startup.
Only projects written in this setting get it. Note that if you write a project
here meaning only to relax one approval, you allow the other three along with
it.

### 11.5 The per-project configuration file

Put a `.lagent.toml` at the top of a project and you can write settings for
that project only. Only the following two things can be written.

```toml
[approval.tools]
"write_file" = "always"     # in this repository, always ask before writing
"edit_file"  = "always"

[mcp]
exclude = ["chrome-pilot"]  # in this repository, do not use this server
```

Other settings, such as the model, where to connect, and the sandbox, cannot
be written. An unknown key is an error. This is so that a fetched repository
cannot change settings that bear on your safety.

### 11.6 Starting somewhere too broad

If you start in `/`, the home directory, or a parent of it, it asks the
following.

```
⚠ /Users/you is your home directory.
File tools and sandboxed shell writes would span this ENTIRE tree.
start anyway? [y/N]:
```

This is because the scope of the project becomes your whole home, and the
model becomes able to write anywhere under it. In almost every case the answer
is "no", and the right thing is to change to the directory you want to work in
and start again. A one-shot run refuses to start, without asking.

## 12. Connecting MCP servers

MCP (Model Context Protocol) servers are the way to add external tools to
lagent. The model can call the tools a server provides — search, database
queries, browser control and so on — in the same way as the built-in tools.
lagent itself has no web search or similar features, so you add the features
you need with MCP servers.

### 12.1 Define a server

Definitions are written in a JSON file. The format is the same as Claude
Code's `.mcp.json`, so you can copy the definitions you already have as they
are.

| Scope | File | Use |
|---|---|---|
| Global | `~/.config/lagent/mcp.json` | Servers used in every project |
| Project | `<project>/.mcp.json` | Servers used in that repository only |

```json
{
  "mcpServers": {
    "tor-exit-lookup": {
      "command": "tor-exit-lookup",
      "args": ["mcp"]
    },
    "with-env": {
      "command": "some-mcp-server",
      "args": ["--flag", "value"],
      "env": { "SOME_API_KEY": "${SOME_API_KEY}" }
    }
  }
}
```

- What is supported is servers of the kind that lagent starts as a child
  process and talks to over standard input and output (stdio).
- `${NAME}` and `${NAME:-default}` are replaced with the value of the
  environment variable.
- When both files have a server with the same name, the project's is used.
- The project's `.mcp.json` is loaded only after you have trusted the project
  ([11.2](#112-the-trust-question)).

### 12.2 Check the connection

The startup banner shows the number of connected servers and tools. `/mcp`
shows the servers one by one, with the file each was defined in.

An MCP server tool is named `mcp__<server>__<tool>`. It appears under this name
in the `/tools` list too.

If a server could not be started, you are told its name at startup. The other
servers and lagent itself can be used as they are.

### 12.3 Fewer approvals

By default, an MCP server tool asks for approval on every call. This is
because lagent has no way of knowing what a server's tool does inside. You
do know, so you can write that in the configuration.

```toml
[approval.tools]
"mcp__tor-exit-lookup__*" = "never"   # a read-only server: do not ask
"mcp__github__create_issue" = "always"  # a tool that writes outside: always ask
```

See [7.2](#72-per-tool-policy) for details. A tool set to `"never"`
becomes usable in a one-shot run too.

### 12.4 Remove tools you do not use

You can remove a whole server, or particular tools within a server, from the
session.

```toml
[mcp]
exclude = [
  "chrome-pilot",                # do not use the whole server
  "obsidian/patch_vault_file",   # do not use just this tool of this server
]
```

- A removed server is not started.
- A removed tool becomes invisible to the model. The server itself keeps
  running for its other tools. A server that mixes tools that read and tools
  that write can be used for its reading side only.
- Only names that match exactly are covered. You cannot write something like
  `search_*`.
- A name that matches nothing (the name changed on the server's side and so
  on) is reported at startup.
- Built-in tools cannot be removed.

You can do the same in `/settings`. The servers are listed under the MCP
section; open a server with Enter to show its tools, and switch each on and
off with ←→. The change is written to `policy.toml`, only that server is
reconnected, and it takes effect at once.

Exclusions can also be written in the project's `.lagent.toml`. From the
project side, however, they **can only be added to**. What you removed cannot
be brought back from the project side.

All an exclusion can do is "lagent does not call the tool with that name".
The server's capability itself does not go away, and if a tool's name changes
on the server's side, the tool with the new name becomes visible again.

### 12.5 Limiting the tools the model sees

When you connect many MCP servers, the tool descriptions alone make what is
sent to the model each time large (in one case 243 tools came to about
60,000 tokens, and the model took about 2 minutes to read them the first
time). That is too heavy for a local model, so by default lagent **shows the
model only the tools of the servers it has come to need**.

- At startup, all the model can see is a catalog for each server (the
  server's name, the names of its tools, and the first sentence of the
  server's description).
- Once the model has settled on a server it wants to use, it loads that
  server with a tool called `mcp_load`. From then on it can call that
  server's tools. `mcp_load` needs no approval.
- The response right after a load takes longer, by the time it takes to read
  the new tool descriptions.

You can load from your side too.

| How | What it does |
|---|---|
| `/mcp load <server>` | Shows that server's tools to the model in the current session |
| `[mcp].preload = ["server", …]` | Servers to show from the outset, every time. List the ones you always use |
| `--allow mcp__<server>__*` | For that start, lets that server be used without asking, and shows it from the outset |

`/mcp` shows, server by server, whether it is loaded.

- When a session is resumed, the servers that were loaded before are loaded
  again.
- After `/clear`, the session starts from a state in which no server is
  loaded (except `preload`).
- Loading changes only what the model can see. How approval is handled does
  not change.

Setting `[mcp].advertise = "all"` shows every tool from the outset. When you
connect only a few servers, this is less trouble.

### 12.6 Large results and saved results

**Large results.** When the result an MCP server tool returns is too large
(over 20,000 bytes), the whole of it is saved in the session's work directory,
and the model is given part of the head and the tail, and the location of the
saved file. The model reads the part it needs from that file. This prevents it
from looking at part of a large result and believing it has seen all of it.

**Saved copies of results.** Every text result an MCP server returns is saved
in the work directory, whatever its size. The file's name includes the digest
(SHA-256) of its content. The same digest is left in the session record too.

When you want to keep a server's result as evidence in an investigation, do
not ask the model to "write out the result"; ask it to "copy the saved result".
The model uses the copy feature of `write_file` to copy the saved file without
changing a single byte. If you have the model retype a result, values can
change. With a copy, you can check afterwards against the digest in the record
that it is exactly what the server returned.

### 12.7 When something is wrong

| Symptom | What to do |
|---|---|
| A server's responses are odd, or it has stopped | `/mcp reload` reconnects every server. The conversation is not lost |
| You added a server to `mcp.json` | `/mcp reload` brings it into the current session |
| Startup says a server "could not be started" | Check that the server's command can be run from a shell. Start with `LAGENT_MCP_STDERR=1 lagent` and the server's error output appears in the terminal |
| A server is slow to start and gets cut off | Raise `[mcp].startup_timeout_sec` (30 seconds by default) |
| A tool call times out | Raise `[mcp].call_timeout_sec` (60 seconds by default). A server that timed out is stopped, and started again on the next call |
| A notice says the same error happened 3 times in a row | See [6.9](#69-when-an-mcp-server-repeats-the-same-error) |

### 12.8 Starting without MCP

```sh
lagent --mcp off
```

For that start only, no MCP server is started at all. This suits a one-shot
run that does not need MCP. If you never use it, set `[mcp].enabled = false`.

### 12.9 Passing session facts to a server

lagent passes the following three things, as environment variables, to the
servers it starts. You can also write them inside `args` in `mcp.json`, as in
`${LAGENT_SESSION_ID}`.

| Variable | Contents |
|---|---|
| `LAGENT_SESSION_ID` | The ID of the session |
| `LAGENT_PROJECT_DIR` | The project directory |
| `LAGENT_WORK_DIR` | The session's work directory |

For a server that writes files, pass the work directory as its output
location: the project stays clean and the model can read the results as well.

## 13. Using skills

A skill is a directory that packages the procedure for one kind of work. At
its centre is a file called `SKILL.md`; the text of the procedure and the
scripts and reference material it uses can be kept together. The format is the
same as Claude Code's skills.

### 13.1 Install a skill

| Scope | Where it goes |
|---|---|
| Global (every project) | `~/.config/lagent/skills/<name>/SKILL.md` |
| Project | `<project>/.claude/skills/<name>/SKILL.md` |

To use a skill installed for Claude Code in lagent as well, **copy** it.

```sh
mkdir -p ~/.config/lagent/skills
cp -R ~/.claude/skills/meeting-notes ~/.config/lagent/skills/meeting-notes
```

- lagent does not read `~/.claude/`. This avoids lagent's behaviour
  changing every time your Claude Code environment changes.
- Do not make a symbolic link. `~/.claude` is protected as a place where
  credentials are kept, so a linked skill appears in the list but its scripts
  cannot be read inside the sandbox and fail.
- A project's skills can be used only after you have trusted the project.
- When both have a skill with the same name, the project's is used.

### 13.2 Use a skill

When a session starts, the model is given only the name and description of
each skill. The body is loaded when the skill is used.

- **The model uses one by itself.** When what you ask for fits a skill's
  description, the model loads that skill with a tool called `load_skill` and
  follows the procedure.
- **You name one.** Type `/skill <name> [extra instructions]` and the body of
  that skill is sent as the request, as it is. The name can be completed with
  Tab.

```
> /skill meeting-notes @transcript.txt
```

`/skills` shows the list of installed skills, and where they are kept. A skill
put into `~/.config/lagent/skills/` partway through a session can be used after
`/clear`, or after a restart. A skill added to the project can be used once you
have updated the trust record
([11.3](#113-asked-again-when-content-changes)).

### 13.3 Put procedures you want followed in a skill

Of what reaches the model, some is treated as **instructions** and some as
**data**. Which it is depends on the route by which it arrives.

| Route | Treatment |
|---|---|
| Instruction files (`AGENTS.md` and so on), a skill's `SKILL.md`, and the files inside a skill | Instructions. The model follows them |
| Everything a tool returns: files read, command output, MCP server results | Data. The model takes it into account but does not follow it as instructions |

For that reason, **write a procedure that must be followed as a skill.** For
example, if you put a note called "analysis procedure" inside an MCP server
that manages notes, and write "read that procedure and follow it" in an
instruction file, the procedure arrives as data and is not certain to be
followed. Make the procedure a skill, and in the instruction file write "load
the `qa-protocol` skill before analysing".

Conversely, **reference material** such as the specification of a data source,
descriptions of fields, and known pitfalls can stay as notes or files. It is
material for the analysis, and being treated as data is correct.

There is no setting that says "treat the contents of this directory, or of
this MCP server, as instructions". If a place the model can write to were
treated as instructions, something the model wrote by mistake, or text that
slipped in from outside, would become a rule as it stands.

### 13.4 Limits when writing a skill

| What | Limit |
|---|---|
| The description written at the top of `SKILL.md` (`description`) | 1,024 characters. Anything beyond is cut, and you are told at startup |
| The body of `SKILL.md` | 64 KB. Anything beyond is cut, and a note says so |
| Other files inside the skill | 96 KB. A file over this cannot be loaded |
| The number of skills | 100. Any beyond that are left out of the list, and you are told |

Split a long procedure so that `SKILL.md` names files under `references/` and
has them read at that stage.

Of the settings at the top of `SKILL.md`, lagent reads only `name`,
`description` and `argument-hint`. It does not read `allowed-tools`, which
specifies the tools that may be used. Approval is decided by lagent's own
mechanism. When a script inside a skill is run as a command, too, the sandbox
and approval apply to it the same as to any other command.

## 14. Building in your own rules with hooks

A hook is a mechanism by which lagent runs a command of yours at a fixed
point. Use one when you want rules kept mechanically, without relying on the
model's judgement, or when you want to give the model the same information
every time.

Hooks work to the same contract as Claude Code's hooks, so a script written
for Claude Code can be registered as it is.

### 14.1 Kinds of hooks

| Name in the configuration | When it runs | What it can do |
|---|---|---|
| `[[hooks.pre_tool_use]]` | Just before the model calls a tool | Deny the call |
| `[[hooks.session_start]]` | When a session starts, is resumed, or at `/clear` | Add information for the model |
| `[[hooks.user_prompt_submit]]` | Just before your request is sent to the model | Add information. Stop the request |
| `[[hooks.session_end]]` | When a session ends, or at `/clear` | Cleaning up, record keeping and so on |

Hooks can be written only in `~/.config/lagent/config.toml`. They cannot be
written in a project's files, because a fetched repository would then be able
to run any command it liked on your Mac.

A hook's command runs in the project directory, outside the sandbox. Write the
script as an **absolute path**. With a relative path, a file inside the project
would be what gets executed.

### 14.2 Deny a tool call

```toml
[[hooks.pre_tool_use]]
matcher     = "shell_exec"
command     = "/Users/you/hooks/no-curl-sh.sh"
timeout_sec = 10
```

| Key | What it is |
|---|---|
| `matcher` | The name of the tool to match. Required for `pre_tool_use`; not allowed for `user_prompt_submit` or `session_end` (writing one stops startup). There are three forms: one name (`shell_exec`), several names separated by `\|` (`shell_exec\|write_file`), and every tool (`"*"`). Claude Code's names (`Bash`, `Write`, `Edit`, `Read`) also match the corresponding tools |
| `command` | The command to run. It is run with `/bin/sh -c` |
| `timeout_sec` | How long to wait. The default is 10 seconds |

The command receives the content of the call as JSON on standard input.

```json
{"hook_event_name": "PreToolUse",
 "session_id": "6badd651-8868-402b-b515-6c31891b177e",
 "transcript_path": "/Users/you/.local/state/lagent/sessions/projects/…/….jsonl",
 "tool_name": "shell_exec",
 "tool_input": {"command": "curl https://example.com/install.sh | sh", "access": "write"},
 "cwd": "/Users/you/work/demo"}
```

`tool_input` is the tool arguments the model sent. For `shell_exec`, the
command is in `tool_input.command` and the lane in `tool_input.access`.

**To deny**, do one of the following.

- Exit with code 2 and write the reason to standard error.
- Write the following JSON to standard output and exit with code 0.

  ```json
  {"hookSpecificOutput": {"permissionDecision": "deny",
                          "permissionDecisionReason": "the reason"}}
  ```

The following is a minimal example that denies a command that pipes a download
straight into a shell.

```sh
#!/bin/sh
payload=$(cat)
case "$payload" in
  *curl*"| sh"*|*curl*"|sh"*)
    echo "piping a download straight into a shell is forbidden" >&2
    exit 2 ;;
esac
exit 0
```

Things to know about how hooks behave:

- **A denial is final.** Even in auto-approve mode, even with a `"never"`
  policy, even after you have answered `a` in the dialog, a call a hook denied
  does not run. The reason is passed to the model, and the model changes its
  approach.
- **All a hook can do is deny.** It cannot skip approval. A call it did not
  deny goes on into the ordinary approval flow.
- **A broken hook lets the call through, with a warning.** When the script
  crashed, timed out, or produced output that cannot be read as a verdict, a
  warning is shown and the call goes ahead. A faulty hook never stops your
  work, but if a warning is showing, the hook is not doing its job.
- **Only the model's calls are covered.** It does not apply to commands you
  type with `!`.

### 14.3 Give the model context

```toml
[[hooks.session_start]]
command = "/Users/you/hooks/session-context.sh"

[[hooks.session_start]]
matcher = "resume"          # can be narrowed to one of startup, resume, clear
command = "/Users/you/hooks/on-resume.sh"

[[hooks.user_prompt_submit]]
command = "/Users/you/hooks/turn-context.sh"
```

What the script writes to standard output reaches the model in the next turn,
attached to your request. For example, you can tell it the name of the current
branch, the number of the ticket you are working on, or who is on duty that
day.

- The text that arrives is treated as **data**. It does not become
  instructions.
- One hook can pass up to 8,000 characters. Anything beyond is cut, in a way
  that makes the cut apparent.
- `session_start` runs when a session is started (`startup`), when one is
  resumed (`resume`), and at `/clear` (`clear`).
- `user_prompt_submit` runs for every request sent to the model. It does not
  run for slash commands or `!` commands, except `/skill`, which becomes a
  request to the model: then `prompt` holds the text with the skill's body
  expanded. The text of the request is in `prompt` in the JSON it receives.

**Stopping a request.** A `user_prompt_submit` hook can stop the request
itself, in the same way as denying a tool call (exit code 2, or the deny
JSON). A stopped request is not sent to the model and is not kept in the
record. The reason is shown on the screen.

### 14.4 Run at the end of a session

```toml
[[hooks.session_end]]
command = "/Users/you/hooks/session-end.sh"
```

It runs on exit (`reason` is `exit`) and at `/clear` (`reason` is `clear`).
Its output is ignored, and it cannot stop anything. The JSON it receives
contains `session_id` and `transcript_path`, so it can be used, for example,
to keep a copy of the session record somewhere else.

## 15. Configuration reference

### 15.1 Files and precedence

| File | Contents | Written by |
|---|---|---|
| `~/.config/lagent/config.toml` | The settings described in this chapter | You |
| `~/.config/lagent/policy.toml` | The per-tool policy, project trust and MCP exclusions that were changed on screen | lagent. You may edit it by hand |
| `~/.config/lagent/mcp.json` | MCP server definitions ([chapter 12](#12-connecting-mcp-servers)) | You |
| `~/.config/lagent/risk-rules.md` | Risk rules ([7.4](#74-risk-rules)) | You |
| `~/.config/lagent/AGENTS.md` and so on | Instruction files that apply to every project | You |
| `~/.config/lagent/skills/` | Skills ([chapter 13](#13-using-skills)) | You |
| `<project>/.lagent.toml` | Per-project policy ([11.5](#115-the-per-project-configuration-file)) | The project's owner |

When the same setting is in more than one place, the one higher up in the
following order is used.

1. Command flags (`--model` and so on)
2. `LAGENT_*` environment variables
3. The configuration file
4. Default values

- To use a different configuration file, add `--config <path>`.
- An unknown key in the configuration file is an error at startup. When a
  value is not valid, lagent likewise stops and names the key.
- `/settings` shows the settings now in effect and where each was decided
  ([15.12](#1512-viewing-and-changing-settings-on-screen)).

In the tables below, "Default" is the value when nothing is written.

### 15.2 [llm] — the server and the model

| Key | Default | Description |
|---|---|---|
| `provider` | `lmstudio` | The kind of LLM server. `lmstudio`, `mlxserve`, `ollama`, `openai`. It changes only where the context window is queried; how the conversation is sent is the same |
| `base_url` | `http://localhost:1234/v1` | Where the server's API is |
| `model` | (required) | The model ID. The name the server lists |
| `api_key` | (none) | Write it only when the server requires one. A local server normally does not |
| `reasoning_effort` | (none) | How deeply the model is to think. The value is sent to the server as written ([16.5](#165-letting-the-model-think)) |
| `risk_model` | (same as `model`) | The model used for command review. It must be one on the same server ([7.3](#73-auto-approve-mode)) |
| `risk_reasoning_effort` | `none` | How deeply the judge is to think. With `""`, nothing is sent |

### 15.3 [model] — the context window

| Key | Default | Description |
|---|---|---|
| `context_window` | `0` | The size the model can handle at once (tokens). `0` queries the server at startup. With `provider = "openai"` there is nowhere to query, so write it yourself |

Details are in [16.3](#163-setting-the-context-window).

### 15.4 [sandbox] — the sandbox

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | Use the sandbox. With `false`, every shell command asks every time ([7.6](#76-leave-the-sandbox-on)) |
| `read_lane_prompts` | `false` | With `true`, commands in the `read` lane ask for approval too |
| `read_lane_deny_exec` | (none) | Names of programs that must not be started from the `read` lane. They are added to the built-in list (`osascript`, `open`, `launchctl`, `defaults`, `security`, `pbcopy` and so on) |

`[sandbox.scratch_caches]` is the setting that points build tools' caches at
the session's scratch directory. The `read` lane cannot write to the usual
cache locations, so pointing them there lets builds and tests run in the
`read` lane (without asking).

```toml
[sandbox.scratch_caches]
GOCACHE       = "go-build"   # the line that is there by default
PIP_CACHE_DIR = "pip"        # an example of adding one
UV_CACHE_DIR  = "uv"
```

The left side is the name of the environment variable, the right side the
name of the directory. Setting a value to `""` cancels that line. List only
caches that can be rebuilt.

### 15.5 [directories] — directories outside the project

| Key | Default | Description |
|---|---|---|
| `write` | (none) | Directories always added as writable directories |
| `read` | (none) | Directories always added as read-only directories |

Details are in [chapter 9](#9-working-in-directories-outside-the-project).

### 15.6 [agent] — behaviour

| Key | Default | Description |
|---|---|---|
| `max_turns` | `50` | The guide figure for the number of model calls in one turn. When it is reached, you are asked whether to continue. It can continue up to 3 times this ([6.8](#68-the-check-when-a-turn-runs-long)) |
| `shell_timeout_sec` | `120` | The time limit for one shell command (seconds) |
| `auto_approve` | `false` | Start in auto-approve mode. It has no effect in one-shot runs ([7.3](#73-auto-approve-mode)) |
| `read_only` | `false` | Start in read-only mode ([7.5](#75-read-only-mode)) |
| `auto_compact` | `true` | Compact automatically when the conversation grows large ([8.4](#84-long-conversations-and-compaction)) |
| `compact_at_pct` | `80` | The percentage at which compaction starts (%). 10–99 |

### 15.7 [mcp] — MCP servers

| Key | Default | Description |
|---|---|---|
| `enabled` | `true` | With `false`, no MCP server is started at all |
| `call_timeout_sec` | `60` | The time limit for one tool call (seconds) |
| `startup_timeout_sec` | `30` | How long to wait for a server to start and respond (seconds). Servers start at the same time, so this is not a total but the time the slowest one may take |
| `exclude` | (none) | Servers and tools not to use ([12.4](#124-remove-tools-you-do-not-use)) |
| `advertise` | `deferred` | `deferred` shows the model only the tools of the servers that have been loaded. `all` shows everything from the outset ([12.5](#125-limiting-the-tools-the-model-sees)) |
| `preload` | (none) | With `deferred`, the servers to show from the outset |

### 15.8 [approval] — approval

| Key | Default | Description |
|---|---|---|
| `model_tier` | `off` | With `shell`, in auto-approve mode, commands in the `write` lane are reviewed by a separate model call ([7.3](#73-auto-approve-mode)) |
| `trusted_projects` | (none) | Projects trusted without asking ([11.4](#114-projects-trusted-without-asking)) |
| `pin_trusted_files` | `true` | Ask again when the content of a trusted file has changed ([11.3](#113-asked-again-when-content-changes)) |

`[approval.tools]` is where the per-tool policy is written
([7.2](#72-per-tool-policy)).

### 15.9 [tui] — the screen

| Key | Default | Description |
|---|---|---|
| `theme` | `auto` | Colours. `auto`, `dark`, `light`, `plain` |
| `language` | `auto` | The language of the screen. `auto`, `ja`, `en` |
| `show_thoughts` | `true` | Show the model's thinking, as sent by the server, while it works |
| `images` | `auto` | Image display. `auto` (asks the terminal at startup), `off`, `iterm`, `kitty` |
| `notify` | `bell` | The notice when a dialog is waiting. `bell`, `osc9`, `off` ([4.6](#46-the-notice-when-a-dialog-is-waiting)) |

`[tui.diagram]` sets the typeface of the text when a diagram is drawn as a
picture. The default is Hiragino Kaku Gothic.

| Key | Description |
|---|---|
| `font`, `font_name` | The font file used for body text (`.ttf`, `.otf`, `.ttc`), and the name of the face inside it |
| `bold_font`, `bold_font_name` | The typeface used for headings |

If you specify a typeface that cannot be loaded, lagent tells you at startup
and uses the default one.

All the settings for the screen take effect from the next start.

### 15.10 [hooks]

`[hooks]` is described in [chapter 14](#14-building-in-your-own-rules-with-hooks).

### 15.11 Settings from the environment

The following keys can be set from environment variables too.

| Setting | Environment variable |
|---|---|
| `[llm].provider` | `LAGENT_PROVIDER` |
| `[llm].base_url` | `LAGENT_BASE_URL` |
| `[llm].model` | `LAGENT_MODEL` |
| `[llm].api_key` | `LAGENT_API_KEY` |
| `[llm].reasoning_effort` | `LAGENT_REASONING_EFFORT` |

The other environment variables are in [Appendix E](#appendix-e-environment-variables).

### 15.12 Viewing and changing settings on screen

`/settings` shows every setting, with its current value and where that value
was decided. In a terminal it opens a panel where you can select and change
values; the sample below is the plain-text form printed when the input is
piped.

```
[session]
  agent.auto_approve    false     (default)      (editable in the TUI)
  agent.read_only       false     (config.toml)  (editable in the TUI)
[approval policy]
  shell_exec            always    (policy.toml)  (editable in the TUI)
```

The parentheses on the right say where the value was decided.

| Shown | Meaning |
|---|---|
| `flag` | A flag at startup |
| `env:…` | An environment variable |
| `config.toml`, `policy.toml` | That file |
| The name of a project file | The project's `.lagent.toml`. When it was ignored because it is not trusted, that is shown too |
| `pattern` | It matched a policy that uses `*` |
| `session` | Changed within this session |
| `measured` | The result actually verified at startup (the state of the sandbox) |
| `default` | The default value |

| Key | Action |
|---|---|
| ↑↓ | Select a row |
| ←→, Enter | Change the value. On an MCP row, Enter opens or closes the server |
| `s` | Switch whether a per-tool policy applies globally or to this project only |
| Esc | Close |

What can be changed on the spot is the per-tool policy, MCP exclusions,
auto-approve, read-only and automatic compaction. For the model, the server
to connect to and the like, edit the configuration file and start again. A
row that cannot be changed shows the reason.

## 16. Tuning the LLM server and the model

How lagent feels to use changes a great deal with the choice of LLM server
and model. This chapter explains how to tune them.

### 16.1 Supported LLM servers

lagent can talk to any server that has an OpenAI-compatible API
(`chat/completions`). When you tell it the kind with `[llm].provider`, it
queries the context window in the way that suits that server.

| `provider` | Server | Example `base_url` | Notes |
|---|---|---|---|
| `lmstudio` | LM Studio | `http://localhost:1234/v1` | The combination that has been tested. The default |
| `mlxserve` | mlx-serve | Depends on the server's settings | See the cautions in [16.6](#166-using-mlx-serve) |
| `ollama` | Ollama | `http://localhost:11434/v1` | |
| `openai` | Any other OpenAI-compatible server | Depends on the server | Write `[model].context_window` yourself |

### 16.2 Choosing a model

- **Choose a model that supports tool calls.** lagent assumes that the model
  replies "I want to call this tool" in a fixed form. A model that does not
  support this tries to answer in text alone, without calling tools.
- **If you want to show images, choose a model that can read images.**
- **The larger the size it can handle at once, the longer the work it can
  do.** The instruction files and the tool descriptions alone use at least
  a few thousand tokens. When the instruction files are long, or when many
  MCP server tools are shown, that becomes tens of thousands of tokens.
- **The models that have been tested** are `google/gemma-4-26b-a4b-qat` on
  LM Studio and Qwen 3.6 35B-A3B on mlx-serve.

After changing the model, first check on a small piece of work (reading one
file and fixing it, say) that it calls tools correctly.

### 16.3 Setting the context window

The size the model can handle at once (the context) is set by the model's
own limit and by the settings used when the model is loaded into the LLM
server. The larger you make it, the more of the Mac's memory it uses.

lagent queries the server for this size at startup and shows it in `ctx` in
the status bar. It also decides from this value when to start compaction
([8.4](#84-long-conversations-and-compaction)).

- In LM Studio, the length you specified when loading the model is used. To
  change it, load the model again on the LM Studio side.
- When the query fails (the server was not running at startup, the server
  does not know the model name, and so on), lagent does not stop; it runs
  with the limit unknown. Automatic compaction does not work in this state,
  so fix the cause and start again ([8.4](#84-long-conversations-and-compaction)).
- For a server that does not answer the query (`provider = "openai"`), write
  the value yourself in `[model].context_window`. If you write a value
  larger than the server's real one, compaction comes too late and you get
  an error.
- When `[model].context_window` is written, no query is made. So that it
  does not disagree with the server's value, we recommend leaving it at `0`
  for servers that answer the query.

### 16.4 Shortening the wait

Most of the waiting with a local model is the time the model spends
**reading what it was sent**. For content that opens the same way as the
time before, the LLM server remembers the result of having read it and reads
only the continuation. So that this takes effect, lagent sends the first
part of what it sends each time (the instructions, the tool descriptions,
the conversation so far) without changing a single character.

The waiting time therefore comes out as follows.

| Situation | Wait |
|---|---|
| Continuing a conversation | Short (only the newly added part is read) |
| The first response of a session | Can be long (if the server does not remember the earlier content, it reads all of it) |
| Right after compaction | Long (the first part of the conversation has changed, so it is re-read) |
| Right after loading an MCP server | Long (the tool descriptions have grown, so it is re-read) |
| After `/clear` | Short, if the instructions and the tool descriptions are the same |

Here are some ways of shortening the wait.

- **Show only the MCP server tools that are needed.** Leave the default
  setting (`advertise = "deferred"`) as it is. If you list the servers you
  use every time in `[mcp].preload`, no re-read happens partway through.
- **Do not connect MCP servers you do not use.** Use `--mcp off` or
  `[mcp].exclude`.
- **Keep the instruction files short.** The instruction files are read in
  full every time.
- **If compaction happens often, raise the context window, or `/clear`
  regularly.**

If the `cache` share in the status bar is high, it is working well.

### 16.5 Letting the model think

With a model that can "think" before it answers, `[llm].reasoning_effort`
lets you specify that. The value you write is sent to the server as it is.
If you write nothing, nothing is sent and it is left to the defaults of the
server and the model.

| Server | How the value is treated |
|---|---|
| LM Studio | Accepts `none`, `minimal`, `low`, `medium`, `high`, `xhigh`. Any other value is an error. An accepted value is rounded to a level the model supports. With Gemma 4, for example, `none` is "do not think" and anything else is "think" |
| mlx-serve | Does not check the value. `none` is "do not think", and every other value is "think" |

Having the model think reduces failures on difficult work, but takes time.
In measurements, the overall time taken roughly doubled. A well-balanced way
to use it is to leave it unwritten normally and specify it only for
difficult work.

The command judge ([7.3](#73-auto-approve-mode)) is set to "do not think" by
default (`[llm].risk_reasoning_effort = "none"`). This is because having it
think did not improve its verdicts and made it 6 times slower.

### 16.6 Using mlx-serve

- **Narrow where it listens.** By default, mlx-serve listens on every
  network interface of the Mac. Other machines on the same network would be
  able to use it too, so either set it to listen on `127.0.0.1` only, or set
  an API key (on the lagent side, `[llm].api_key`).
- **Raise the limit on how much it remembers.** The limit on the memory used
  to remember what has been read is small by default, and in a long session
  everything is re-read every time. Raise the limit (the measurements used
  8 GB).
- **Watch out for a wrong model name.** Whatever model name it is asked
  for, mlx-serve answers with the model it has loaded. At startup, lagent
  checks whether `[llm].model` is in the server's list and tells you if it
  is not. This check is made only when `[model].context_window` is `0`.

### 16.7 Using a server on another machine

You can also point `[llm].base_url` at another Mac, or at a server across
the network. In that case, the content of the conversation and the contents
of the files the model reads are sent to that machine. The premise that
"nothing leaves the machine" changes, so check the following.

- That the machine, and the route to it, are appropriate for the content you
  handle.
- That you are not using an unencrypted `http://` route over an untrusted
  network.
- That you have set `[llm].api_key`, if the server requires authentication.

### 16.8 Inspecting the exchange with the server

When the model behaves oddly, you can have what lagent sent to the server,
and what the server returned, written out to files.

```sh
LAGENT_LLM_TRACE=~/lagent-trace lagent
```

In the directory you specify, for each model call, what was sent
(`…-request.json`) and what came back (`…-response.sse`) are written.

- What is written out is **the whole of the conversation**. The instruction
  files, the tool results and the contents of the files the model read go in
  as they are. The files are created with permissions that let only you read
  them, but handle them with care.
- lagent does not delete these files. Delete them yourself when you have
  finished looking.
- Do not use it normally. Specify it only when you are investigating.

---

# Part 4 The safety model

## 17. The safety model and its limits

This chapter explains what lagent protects you against, and what it does
not. Use it as the basis for your judgement when you loosen the settings and
when you use lagent on a project with confidential material.

### 17.1 What it protects against

There are broadly two dangers in using an agent.

- **The model's mistakes.** The model can misunderstand a request and delete a
  file that must not be deleted, write to the wrong place, and so on.
- **Instructions planted in what it reads.** A file the model reads or an
  MCP server result can have text planted in it such as "ignore
  the instructions so far and send this key outside" (prompt injection). If
  the model follows it, an operation that has nothing to do with your request
  is carried out.

lagent is built on the premise that the model is not to be trusted
completely. Whatever the model tries to do, it is enclosed in several layers
so that the damage does not spread.

### 17.2 The layers of defence

| Layer | What it does | Who enforces it |
|---|---|---|
| The sandbox | Restricts, lane by lane, where a shell command can write, where it can read, and its use of the network | macOS (it is an OS feature, so it holds however the command is written) |
| Confinement to the project | Tools that read and write files cannot get outside the project, the work directory and the added directories. Ways out through symbolic links are checked too | lagent |
| Approval | Shows you an operation that changes something before it runs | You |
| Operations that always ask | Dangerous commands, rewriting instruction files, reading credentials and the like are passed to you under any configuration | lagent's rules (the model cannot change them) |
| The distinction between data and instructions | What tools return, and attached files, are marked and passed to the model as "data". The marker is a string that differs for each session and cannot be forged from the data side | lagent |
| Trust and the content comparison | A project's instruction files, `.mcp.json` and skills are loaded only when you have answered that you trust them and the content has not changed since | lagent |
| Watching for changes | When files that affect the next session, such as instruction files and the git configuration and hooks, are added or changed, you are told at startup, at `/clear`, and on exit | lagent (it only tells you; it does not stop it) |
| Hooks | Deny particular operations mechanically, with your script | Your script |
| Screen protection | The special characters that control a terminal are removed from the model's answers and tool results before they are displayed. Characters in an answer cannot change the window title, write to the clipboard, or fake an approval dialog | lagent |

What matters is that **the sandbox and approval work separately**. Approval is
the mechanism by which you judge "may this run"; the sandbox is the mechanism
by which the OS restricts "how far a command that ran can reach". Even if
something is overlooked at approval, it cannot get outside the sandbox. In
auto-approve mode too, the sandbox is in effect in just the same way.

### 17.3 Checking the state of the sandbox

Every time it starts, lagent tests whether the sandbox actually takes
effect. It really carries out checks such as trying to write where writing
must not be possible and trying to use the network where it must not be
usable, and only when all of them are refused as expected does it run `read`
lane commands without asking.

| State | Shown at startup | How shell commands are handled |
|---|---|---|
| Verified | Nothing about the sandbox is shown | The `read` lane runs without asking; the `write` lane asks and then runs. The OS restricts the bounds of both |
| The write restriction could not be verified | `warning: sandbox unverified: …` and which check did not pass, followed by a `sandbox: DISABLED — …` line too | Every command asks you, every time. This cannot be skipped with `a`, `"never"` or auto-approve |
| The read-only restriction could not be verified | `warning: read lane disabled: …` and a `sandbox: enabled (read lane unverified …)` line | `read` lane commands also ask before running. This can be skipped with `a`, `"never"` or auto-approve |
| Disabled | `sandbox: DISABLED` | Every command asks you, every time. There is no restriction by the OS |

The result of the check can also be seen in the `[safety]` section of
`/settings`.

For this check, at every start, every `/clear` and every time a directory is added, lagent tries to create
small files whose names start with `.lagent-lane-probe-` in the project
directory and elsewhere, and deletes them at once. If the project directory
cannot be written to, the check does not pass and the "could not be verified"
handling above applies.

When the configuration enables the sandbox but the OS cannot apply it,
lagent does not start. It never silently runs without the sandbox.

### 17.4 What is not protected

The following are not protected by lagent's mechanisms. Make up for them in
the way you use it.

- **The conversation stays inside only while the LLM server is local.** If
  you point `[llm].base_url` at another machine, the conversation is sent
  there ([16.7](#167-using-a-server-on-another-machine)). How the LLM server
  program treats the content is that program's concern.
- **Commands in the `write` lane can use the network.** The OS restricts
  only where they can write, not what they send where. When you approve a
  command in the `write` lane, look for anything in it that sends data
  outside. If you set `shell_exec` to `"never"`, or use command review, the
  check by your own eyes is gone.
- **Secrets in places that are not on the list are not protected.** Only
  what is on the list in [6.6](#66-credential-files) is treated as
  credentials. A token written in `~/.config/<tool name>/config.toml`, for
  example, can be read without asking by a command in the `read` lane (`cat`
  and so on). The same goes for `[llm].api_key` written in lagent's own
  configuration file, and for the session records.
- **Environment variables are inherited by the commands that are started.**
  The environment variables of the shell that started lagent are passed, as
  they are, to the commands lagent runs, to MCP servers and to hooks. Do not
  start lagent from a shell that has environment variables holding secrets.
  To leave out just a particular variable, use `env -u NAME lagent`. The
  variables for lagent's own settings (`LAGENT_API_KEY` and so on) are not
  passed on.
- **MCP servers run outside the sandbox.** An MCP server is an ordinary
  program running with your privileges. It can reach the network too. All
  lagent can do is put its tool calls through approval. Connect only servers
  you can trust.
- **Inside an added directory, the authority is what you gave.** If a place
  you added as a writable directory holds configuration files that another
  program loads, or scripts that it runs, the model can rewrite them.
- **The session records keep what was read** ([8.8](#88-handling-the-records)).
  The same goes for the files written out with `LAGENT_LLM_TRACE`.
- **The correctness of the model's answers is not guaranteed.** What lagent
  restricts is the model's "operations", not the content of its answers.
  Take it as given that a local model makes mistakes more easily than a
  large cloud model.

### 17.5 Habits for safe use

- Start at the top of the repository you are working on. Do not start in the
  home directory.
- For a repository you have only just fetched, answer "no" to the trust
  question and look inside with `--read-only`.
- Judge an approval dialog by reading the content on the `┊` lines, not the
  explanation on the `↪` line.
- Use `a` and `p` after thinking about what you would be allowing. When in
  doubt, `y`.
- Set MCP tools that can write or send to `"always"`. Exclude the ones you do
  not use.
- Do not keep secrets in the project or in environment variables.
- When a notice starting with `warning:`, `sandbox` or `note:` appears at startup, read
  it.

---

# Appendices

## Appendix A Commands and flags

### Commands

| Command | What it does |
|---|---|
| `lagent` | Starts an interactive session with the current directory as the project |
| `lagent "<request>"` | Starts an interactive session and sends that request first |
| `lagent -p "<request>"` | Runs one request and exits ([chapter 10](#10-one-shot-runs-and-pipelines)) |
| `lagent sessions [--all]` | Lists the sessions that can be resumed. `--all` covers every project |
| `lagent trust [--accept]` | Shows the state of the project's trust. `--accept`, in a project that is already trusted, records the current content afresh |
| `lagent workdirs` | Lists this project's work directories of past sessions |
| `lagent workdirs clean [ID…] [--yes]` | Deletes work directories after confirming. An ID may be given by its first 4 or more characters. `--yes` deletes without confirming |
| `lagent completion <shell>` | Prints a shell completion script |
| `lagent version`, `lagent --version` | Prints the version |

### Flags

| Flag | What it does |
|---|---|
| `-p`, `--prompt <request>` | One-shot run |
| `-c`, `--continue` | Resume this project's last session |
| `--resume <ID>` | Resume the given session |
| `--auto` | Start in auto-approve mode |
| `--allow <name>` | For that start only, do not ask about the tools named. Repeat it, or separate the names with commas |
| `--read-only` | Start in read-only mode |
| `--writable` | For that start only, lift the read-only mode set in the configuration |
| `--add-dir <path>` | Add a writable directory. Can be repeated |
| `--read-dir <path>` | Add a read-only directory. Can be repeated |
| `--model <ID>` | For that start only, change the model |
| `--mcp <on\|off>` | For that start only, change whether MCP servers are used |
| `--config <path>` | Use a different configuration file |
| `--no-sandbox` | Turn the sandbox off (for investigating faults; not for normal use) |
| `-h`, `--help` | Show usage |
| `-v`, `--version` | Show the version |

`-p` and the form that passes the request as an argument
(`lagent "<request>"`) cannot be used together.

## Appendix B Slash commands

| Command | What it does |
|---|---|
| `/help` | The commands, how to write attachments, the key list |
| `/tools` | The tools available, and whether each one asks for approval |
| `/mcp` | The connected MCP servers, and whether each is loaded |
| `/mcp load <server>` | Shows that server's tools to the model |
| `/mcp reload` | Reconnects the MCP servers |
| `/auto [on\|off]` | Auto-approve mode. With no argument, toggles it |
| `/readonly [on\|off]` | Read-only mode. With no argument, shows the current state |
| `/add-dir <path>` | Adds a writable directory |
| `/read-dir <path>` | Adds a read-only directory |
| `/dirs` | Lists the project, the work directory and the added directories |
| `/settings` | Shows and changes settings |
| `/skills` | Lists the skills, and whether each is global (`[global]`) or the project's (`[project]`) |
| `/skill <name> [extra instructions]` | Uses a skill by name |
| `/memory` | Lists the saved memories |
| `/remember [global] <name> <fact>` | Saves a memory |
| `/forget [global] <name>` | Deletes a memory |
| `/show <path>` | Displays an image in the terminal |
| `/usage` | A breakdown of the tokens used in this session |
| `/version` | The version, and the environment it is running in |
| `/compact` | Summarises the older part of the conversation to shorten it |
| `/clear` | Resets the conversation and starts a new session |
| `/quit` | Quits. `/exit` does the same |

The `/settings` screen can be used only when you are interacting in a
terminal. When input is piped in, the same content is printed in a read-only
form.

## Appendix C Built-in tools

This is the list of the tools the model can call. The names are used in the
approval dialog, in `/tools`, in the per-tool policy and in a hook's
`matcher`.

### Reading and inspecting (no approval)

| Tool | What it does |
|---|---|
| `list_files` | Lists the contents of a directory |
| `list_tree` | Lists the project as a tree |
| `search_files` | Searches the contents of files. Files it did not search — because they are too large, are binary, or cannot be read — are reported by number and name |
| `read_file` | Reads a file. You can read just part of it by giving a range of lines or a range of bytes |
| `file_info` | Finds out a file's type, size, timestamps and hash without reading it |
| `view_image` | The model looks at an image file |
| `show_image` | Displays an image in your terminal |
| `ask_user` | Asks you a question, showing choices |
| `load_skill` | Loads a skill |
| `mcp_load` | Loads an MCP server's tools ([12.5](#125-limiting-the-tools-the-model-sees)) |

By default, `list_tree` and `search_files` skip dependency and build
directories such as `node_modules`, and anything excluded by `.gitignore`.
They always report what they skipped. `list_files` lists those entries,
marked `[ignored]`.

Only when the file being read is a credential file do these tools ask you
too ([6.6](#66-credential-files)).

### Changing things (approval)

| Tool | What it does |
|---|---|
| `write_file` | Creates a new file, or rewrites one whole. It can also copy a file that already exists |
| `edit_file` | Replaces part of a file, given a string |
| `shell_exec` | Runs a shell command. Commands in the `read` lane need no approval |
| `save_memory`, `delete_memory` | Saves a memory, deletes a memory |
| `request_directory` | Asks you to make a directory outside the project available |

When the output of `shell_exec` exceeds 20,000 bytes, the model is given the
head and the tail, and the whole is saved to the work directory (up to
32 MiB). The output of `operator` lane commands, and of commands run without
the sandbox, is not saved.

### MCP server tools

MCP server tools are added under names of the form `mcp__<server>__<tool>`.

## Appendix D Files and locations

### Configuration (`~/.config/lagent/`)

| Path | Contents |
|---|---|
| `config.toml` | Settings |
| `policy.toml` | Settings that lagent writes (the per-tool policy, project trust and the digests of trusted files, MCP exclusions) |
| `mcp.json` | MCP server definitions |
| `risk-rules.md` | Risk rules |
| `AGENTS.md` and so on | Instruction files that apply to every project |
| `skills/<name>/` | Skills |

### Records and state (`~/.local/state/lagent/`)

| Path | Contents |
|---|---|
| `sessions/projects/<per-project name>/` | Session records (one file per session) |
| `memory/global/`, `memory/projects/<per-project name>/` | Memory |
| `<per-project name>/work/<session ID>/` | The session's work directory |
| Other files under `<per-project name>/` | Saved copies of the files that affect the next session (`persistent.json` and so on), the build caches used by commands in the `write` and `operator` lanes |

The "per-project name" is the project's path with `/` replaced by `-`.

### Inside the project

| Path | Contents |
|---|---|
| `AGENTS.md`, `AGENT.md`, `CLAUDE.md`, `GEMINI.md` | Instruction files |
| `.mcp.json` | That project's MCP servers |
| `.claude/skills/<name>/` | That project's skills |
| `.lagent.toml` | That project's policy |

lagent itself never leaves record or state files inside the project. It
only creates and deletes, temporarily, the small files used to verify the
sandbox ([17.3](#173-checking-the-state-of-the-sandbox)).

## Appendix E Environment variables

### Read as settings

| Variable | What it is |
|---|---|
| `LAGENT_PROVIDER` | `[llm].provider` |
| `LAGENT_BASE_URL` | `[llm].base_url` |
| `LAGENT_MODEL` | `[llm].model` |
| `LAGENT_API_KEY` | `[llm].api_key` |
| `LAGENT_REASONING_EFFORT` | `[llm].reasoning_effort` |
| `LAGENT_STATE_DIR` | Changes where the records and state are kept (for testing) |
| `LAGENT_MCP_STDERR` | With `1`, MCP servers' error output goes to the terminal |
| `LAGENT_LLM_TRACE` | The directory the exchange with the LLM server is written out to ([16.8](#168-inspecting-the-exchange-with-the-server)) |
| `LC_ALL`, `LC_MESSAGES`, `LANG` | The language of the screen (when `[tui].language = "auto"`) |
| `RUNEWIDTH_EASTASIAN` | Changes how the width of symbols such as box-drawing lines is counted (for adjusting when the display is misaligned) |

### What lagent passes to the commands it starts

These are passed to shell commands, MCP servers and hooks.

| Variable | What it is |
|---|---|
| `LAGENT_SESSION_ID` | The session ID |
| `LAGENT_PROJECT_DIR` | The project directory |
| `LAGENT_WORK_DIR` | That session's work directory |
| `TMPDIR`, `TMP`, `TEMP` | For `read` lane commands, the scratch directory dedicated to the session |
| `GOCACHE` and so on | The cache locations written in `[sandbox.scratch_caches]` (passed to shell commands only) |

The environment variables of your shell are passed on as they are. Only the
variables for lagent's own settings (`LAGENT_PROVIDER` to `LAGENT_LLM_TRACE`
in the table above) are not passed on. When an MCP server needs those
values, write them in that server's `env` in `mcp.json`.

## Appendix F Troubleshooting

### It does not start

| Symptom | Cause and fix |
|---|---|
| It cannot connect (`connection refused`) | The LLM server is not running, or `[llm].base_url` is wrong. Check with `curl <base_url>/models`. It starts, and the first request fails |
| No model is specified | Write `[llm].model`. This stops startup |
| The model is not in the list | Make `[llm].model` match the name in the server's list. It starts, and the first request fails |
| `context window unknown` is shown | Startup goes on, without the context gauge. Write `[model].context_window` only with `provider = "openai"` (without it, startup stops). With other servers, leave it at 0 and fix the cause ([16.3](#163-setting-the-context-window)) |
| An error naming a configuration key | The key name or the value is wrong. Fix the key shown |
| An error saying the sandbox cannot be applied | macOS `sandbox-exec` cannot be used in this environment. You can start with `--no-sandbox` just while you look for the cause, but every command will then ask |
| It refused to start in the home directory | Move to the project directory and start there |
| It does not start because a directory to be added was refused | See [9.3](#93-places-that-cannot-be-added) |

### Model responses

| Symptom | Cause and fix |
|---|---|
| The first response is very slow | The model is reading the content from the beginning. It is faster from the second time on ([16.4](#164-shortening-the-wait)) |
| "the stream may be stalled" appears | Nothing has arrived for 90 seconds. Check on the LLM server's side whether processing is moving forward. If it has stopped, stop the turn with Ctrl+C |
| "the model returned an empty response" keeps appearing | The model is failing to produce a response. Send again, or try the setting that lets the model think ([16.5](#165-letting-the-model-think)) |
| The model does not use tools and only explains in text how to do it | Check that the model supports tool calls ([16.2](#162-choosing-a-model)). If it does, ask again with "actually edit it", or split the work into smaller pieces |
| "the response was cut off mid-generation" appeared | The answer was too long. Ask for "the rest", or split the request |
| An error saying the conversation overflowed | Use `/compact` or `/clear`. Check that `[model].context_window` is not larger than the server's real value |
| The model gets the content of a file it read earlier wrong | The conversation may have been compacted ([8.4](#84-long-conversations-and-compaction)). Have it read the file again, or start over with `/clear` |
| An error over the value of `reasoning_effort` | Write a value the server accepts ([16.5](#165-letting-the-model-think)) |
| An image is attached but its content does not get across | Use a model that can read images |

### Input and the screen

| Symptom | Cause and fix |
|---|---|
| Option+Enter sends the message | The terminal needs to be configured. Use Ctrl+J ([4.3](#43-typing)) |
| `y` does not work in the dialog | Japanese input is ON. Select with ←→ and confirm with Enter |
| A diagram is not drawn as a picture, or an image does not appear | Pictures can be displayed in iTerm2, kitty and Ghostty. With `[tui].images` at `auto`, they are not displayed inside tmux or screen. When the terminal has been detected wrongly, set `[tui].images` to `iterm` or `kitty` |
| The colours are hard to read | Set `[tui].theme` to one of `dark`, `light` or `plain` |
| The frame lines are misaligned | Try setting the environment variable `RUNEWIDTH_EASTASIAN` to `0` or `1` |
| What you typed while it was working is not sent | Input typed while it is working is sent after the turn ends. If the turn ends with an error, it goes back to the input box |

### Commands and files

| Symptom | Cause and fix |
|---|---|
| A command fails with `Operation not permitted` | The sandbox is refusing it. Normally the model issues it again in the lane it needs. A command that writes to an instruction file or to `.git/config` needs the `operator` lane |
| A build or a test fails because it cannot write to a cache | Add that tool's cache to `[sandbox.scratch_caches]` ([15.4](#154-sandbox--the-sandbox)) |
| `ps` or `top` does not work | They do not work inside the sandbox. If you need them, run them yourself in another terminal |
| Files outside the project cannot be read | Add that directory ([chapter 9](#9-working-in-directories-outside-the-project)) |
| Every command asks | The sandbox could not be verified. Look at the startup banner and at `[safety]` in `/settings` ([17.3](#173-checking-the-state-of-the-sandbox)) |
| Only part of a large file is read | It is being truncated to fit the size of the conversation. The model reads the rest in pieces ([8.4](#84-long-conversations-and-compaction)) |

### Project settings

| Symptom | Cause and fix |
|---|---|
| You fixed an instruction file but it has no effect | Instruction files are loaded at startup and at `/clear`. When you edited one of the project's instruction files outside lagent, `/clear` does not load it and tells you so. Run `lagent trust --accept` in another terminal and type `/clear` once more. Starting lagent again and answering `y` to the question works too ([11.3](#113-asked-again-when-content-changes)) |
| The project's `AGENTS.md` is not loaded | Look at the state with `lagent trust`. Either it is not trusted, or its content changed after you trusted it ([chapter 11](#11-project-instruction-files-and-trust)) |
| A rule written in an instruction file is not kept | Local models tend to overlook the rules in a long instruction file. Put a short pointer in memory ([8.6](#86-memory)), or turn the procedure into a skill ([13.3](#133-put-procedures-you-want-followed-in-a-skill)) |
| `"never"` in `.lagent.toml` has no effect | That project has to be listed in `[approval].trusted_projects` ([11.4](#114-projects-trusted-without-asking)) |
| A skill's script cannot be read, and it fails | Check whether it is a symbolic link to `~/.claude`, and change it to a copy ([13.1](#131-install-a-skill)) |
| An MCP server does not connect | See [12.7](#127-when-something-is-wrong) |
| The model does not try to use an MCP server's tools | The server may not be loaded. Load it with `/mcp load <server>`, or list it in `[mcp].preload` |

### Sessions

| Symptom | Cause and fix |
|---|---|
| It cannot resume | Check that you are starting in the same directory, and with the same model, as were recorded. When the model differs, add `--model` as shown. When it says "in use", the same session is open in another terminal |
| A one-shot run stops after printing `[stdin: waiting for piped input to end …]` | It is waiting for its input to close. Add `< /dev/null` ([10.2](#102-pipe-data-in)). With standard error thrown away (under cron, say), even that line is not seen |
| Work directories are piling up | Delete them with `lagent workdirs clean` ([8.7](#87-work-directories)) |

## Appendix G Glossary

| Term | Meaning |
|---|---|
| Approval | Your allowing an operation that changes something, before it runs |
| Auto-approve mode | The mode that runs, without asking, the operations the rules can judge safe |
| Compaction | Replacing the older part of the conversation with a summary to make it shorter |
| Hook | A command of yours that lagent runs at a fixed point |
| Instruction file | A file of instructions to the model, such as `AGENTS.md` |
| Lane | The level of privilege given to a shell command |
| LLM server | The program that runs the model and answers lagent's calls (LM Studio and the like) |
| MCP | Model Context Protocol. The contract for adding external tools to an agent |
| Memory | Short facts that the model is made to remember across sessions |
| `operator` lane | The widest of the shell command lanes. You always approve it |
| Project | The directory lagent was started in |
| `read` lane | The shell command lane that only reads. It runs without approval |
| Read-only mode | The mode that fixes the whole session at "change nothing" |
| Risk rules | Prose that tells the review of commands your policy |
| Round | One call to the model within a turn |
| Sandbox | The mechanism by which macOS restricts how far a shell command can reach |
| Session | One conversation, from start to exit |
| Skill | A directory that packages a procedure |
| Token | The unit in which a model counts text. It is the unit of the size of the conversation, and a guide to how long processing takes |
| Tool | An individual operation the model can call |
| Trust | Your accepting that a project's instruction files and the like may be loaded |
| Turn | From one request of yours until the model has finished answering |
| Work directory | A place outside the project, prepared for each session |
| `write` lane | The shell command lane that can write to the project |

## Appendix H Differences from gem-agent

[gem-agent](https://github.com/nlink-jp/gem-agent) is a sister tool built on
the same design. It calls its model from Gemini, which Vertex AI on Google
Cloud provides. The screen, approval, the sandbox, instruction files, MCP
servers, skills and hooks are used in almost the same way.

| | lagent | gem-agent |
|---|---|---|
| Model | A model on the local LLM server | Gemini on Vertex AI |
| Where the conversation is sent | Inside your own Mac | Google Cloud |
| Cost | None (electricity and time) | Charged according to tokens |
| Preparation needed | An LLM server and a model | A Google Cloud project and authentication |
| Size of the conversation it can handle at once | Depends on the model and the Mac's memory | Large |
| Web search, page fetching | No | Yes |
| PDF and Office documents, audio, video | Cannot read them (it can read images) | Can read them |
| Summarising files, delegating research | No | Yes |
| Auto-approve | Judgement by rules is the basis. If configured, it reviews commands in the `write` lane only | Rules, and review by a separate model call |
| How MCP server tools are shown | By default, only the ones needed are shown | By default, all are shown |
| Saving memory | You save it yourself with `/remember`. The model can propose it too | The model proposes and you approve |
| Audit logs | No | Yes |
| Where the configuration is | `~/.config/lagent/` | `~/.config/gem-agent/` |

The two can be used side by side in the same project. They read the
instruction files, `.mcp.json` and the project's skills in common, and each
protects the configuration files (`.lagent.toml` and `.gem-agent.toml`), the
other's included, from being rewritten. You can divide the work between
them: lagent for work that cannot leave the machine, gem-agent for large
work.

## Appendix I Where to learn more

This manual gathers together what you need in order to use lagent. The
details of the mechanisms, and why it is built the way it is, are in the
following documents.

| Document | Contents |
|---|---|
| [reference/configuration.md](reference/configuration.md) | The details of configuration, hooks and commands |
| [reference/architecture.md](reference/architecture.md) | The internal structure |
| [reference/bench.md](reference/bench.md) | The measurement tool for comparing models and settings |
| [INDEX.md](INDEX.md) | The index of documents, and the list of design decision records (ADRs) |
| [CHANGELOG.md](../../CHANGELOG.md) | The changes in each version |

Please send bug reports and requests to
[GitHub Issues](https://github.com/nlink-jp/lagent/issues).
