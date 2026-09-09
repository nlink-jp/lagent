# 設定とコマンド

インストール、設定ファイル、優先順位、コマンド表。恒常文書: キーやコマンドが
増えるたびにその場で更新する。

## インストール

ソースからビルドし（`make build` → `dist/lagent`）、バイナリを PATH に置く。
リリースアーカイブと Homebrew formula は初回リリース（RFP Phase 3）から。

## 設定ファイル

`~/.config/lagent/config.toml`。出荷テンプレートは
[`config.example.toml`](../../../config.example.toml)。ローダがテストで
これを読むので、テンプレートと組み込み既定値は乖離できない。未知のキーは
エラー（strict decode）。

| キー | 既定 | 意味 |
|---|---|---|
| `[llm].provider` | `lmstudio` | 応答するローカルサーバ: `lmstudio`、`ollama`、`openai`。コンテキスト長の検出先を選ぶだけで、会話は全て OpenAI 互換の `chat/completions` エンドポイントを通る |
| `[llm].base_url` | `http://localhost:1234/v1` | OpenAI 互換 API のベース URL |
| `[llm].model` | `google/gemma-4-26b-a4b-qat` | サーバが一覧に出すモデル ID |
| `[llm].api_key` | （未設定） | bearer トークンを要求するサーバ向け。ローカルサーバには不要 |
| `[model].context_window` | `0` | コンテキスト窓（トークン）。`0` は起動時に provider から検出（LM Studio は `/api/v0/models`、Ollama は `/api/show`。`openai` は明示が必要） |
| `[sandbox].enabled` | `true` | `shell_exec` を sandbox-exec で包む。モデルが宣言したレーンをカーネルが強制。off だと全シェル呼び出しが操作者の承認待ち |
| `[sandbox].read_lane_deny_exec` | （未設定） | read レーンが起動してはならないプログラム。組込一覧に追加 |
| `[sandbox].read_lane_prompts` | `false` | read レーンのコマンドにも承認プロンプトを残す |
| `[agent].max_turns` | `50` | 1 ターンのラウンド予算。対話中はチェックポイント、`-p` では停止。絶対上限は 3 倍 |
| `[agent].shell_timeout_sec` | `120` | `shell_exec` のコマンドごとのタイムアウト |
| `[agent].auto_approve` | `false` | 自動承認で開始: 規則層で Safe の呼び出しは尋ねずに走り、Review と Block は尋ねる。**`-p` では無視** — そこでは `--auto` だけが有効。`/auto on|off` と shift+tab でセッション中に変更 |
| `[agent].read_only` | `false` | レーン天井を有効にして開始: セッションのスクラッチの外は何も変えない。`-p` でも効く。`--read-only` / `--writable` で実行ごとに、`/readonly on|off` でセッション中に上書き。ランタイムが下げることはない |
| `[mcp].enabled` | `true` | `false` で global とプロジェクトの全 MCP サーバを無効化。`--mcp on|off` で実行ごとに上書き |
| `[mcp].call_timeout_sec` | `60` | MCP ツール呼び出しごとのタイムアウト |
| `[mcp].exclude` | （未設定） | このセッションに無いサーバ、またはその一機能。プロジェクトの `.lagent.toml` は追加のみ可、削除は不可 |
| `[mcp].advertise` | `deferred` | 接続したサーバのうちモデルに見せる範囲: `deferred` はランタイム事実に目録を出し、モデルがサーバ名で `mcp_load` を呼んだ時点でそのサーバのツールを広告する。`all` は最初から全ツールを広告（計測のベースライン） |
| `[mcp].preload` | （未設定） | `deferred` でも最初から広告するサーバ。`--allow mcp__<server>__*` の許可はその実行でそのサーバをプリロードする |
| `[tui].theme` | `auto` | `auto`、`dark`、`light`、`plain` |
| `[tui].language` | `auto` | `auto`（`LC_ALL` / `LC_MESSAGES` / `LANG` から）、`ja`、`en` |
| `[tui].show_thoughts` | `true` | サーバが送る推論差分をライブ領域に表示。表示専用 |
| `[approval].pin_trusted_files` | `true` | 信頼は内容に与える: 信頼済みプロジェクトのエージェント向けファイルはダイジェストで固定され、変わると再度尋ねる |
| `[approval].tools` | （未設定） | ツールごとのポリシー: `"always"`（常に尋ねる。自動承認でも外せない床）または `"never"`（尋ねない。ブロック対象のシェルパターンは尋ねる）。`--allow` は 1 実行分の同等物 |
| `[approval].trusted_projects` | （未設定） | 自身の `.lagent.toml` で承認を削除できるプロジェクト。起動時の信頼プロンプトと同じ判断 |

2 つのセッションモードは独立した軸: `auto_approve` はゲートに誰が答えるかを、
`read_only` はセッションが何に到達できるかを決める。両方 on も成立する。
プロジェクトの `.lagent.toml` はどちらも持たず、`[approval.tools]` と
`[mcp].exclude` だけを持つので、クローンしたリポジトリはゲートを外せない。

## 優先順位

フラグ > `LAGENT_*` 環境変数 > 設定ファイル > 組み込み既定値。

| 環境変数 | キー |
|---|---|
| `LAGENT_PROVIDER` | `[llm].provider` |
| `LAGENT_BASE_URL` | `[llm].base_url` |
| `LAGENT_MODEL` | `[llm].model` |
| `LAGENT_API_KEY` | `[llm].api_key` |

## コマンド

| コマンド | 意味 |
|---|---|
| `lagent` | カレントディレクトリで対話セッションを開始（端末なら TUI、パイプなら plain REPL） |
| `lagent "<first message>"` | 引数を第 1 ターンとして送ってから対話へ |
| `lagent sessions` | このプロジェクトのセッション一覧（id、日時、プレビュー） |
| `lagent trust` | プロジェクトの信頼とピンを表示・変更 |
| `/mcp`、`/mcp load <server>`、`/mcp reload`（セッション内） | サーバ一覧をロード状態付きで表示。1 サーバのツールを手で広告。再接続 |
| `lagent workdirs` | 過去セッションの作業ディレクトリ一覧。`workdirs clean` で削除 |
| `lagent version` | 版数を表示 — `--version` と同じ行 |

| フラグ | 意味 |
|---|---|
| `--version` | 版数を表示して終了 |
| `-p`, `--prompt` | 単発: このプロンプトを実行して終了。変更を伴うツールは `--allow` に列挙するか `--auto` を付けない限り拒否 |
| `--auto` | 自動承認モードで開始: 規則層で Safe の呼び出しは尋ねずに走る |
| `--allow` | この実行で尋ねないツール: 名前または `mcp__server__*` 接頭辞 |
| `--read-only` / `--writable` | セッションを read レーンに上限で抑える / 上限が無いことを明示 |
| `-c`, `--continue` | このプロジェクトの直近セッションを再開 |
| `--resume` | 特定のセッション id を再開 |
| `--model` | この実行の `[llm].model` を上書き |
| `--mcp on|off` | この実行の `[mcp].enabled` を上書き |
| `--no-sandbox` | sandbox-exec ラッパを無効化（デバッグ専用、危険） |
| `--config` | 設定ファイルのパス |
