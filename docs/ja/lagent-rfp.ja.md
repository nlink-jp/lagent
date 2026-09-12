# RFP: lagent

> Generated: 2026-09-10
> Status: Draft

## 1. Problem Statement

クラウド API に依存せず、ローカル LLM だけで動くコーディングエージェントランタイムがない。
gem-agent は Vertex AI Gemini 専用で、オフライン環境、機密プロジェクト、API 費用を
掛けたくない用途では使えない。lagent は gem-agent と同じ監査可能な最小ループ
（read / edit / shell / MCP / 承認）を、OpenAI 互換 I/F を持つローカル LLM サーバ
（想定: LM Studio、モデル Gemma 4 26B A4B QAT）で提供する。

**目的:** ローカル LLM でエージェントランタイムがどこまで実用になるかを、コスト
（トークン数・壁時計時間・ターン数）と実効性（タスク完遂）の両面で、gem-agent と
同じ物差しで検証する。

**想定利用者:** 当面は開発者本人。実験用途であり、配布は前提にしない。
macOS 上で LM Studio または Ollama を自力で運用できることが前提。

**位置づけ:** gem-agent とは別プロダクトライン。gem-agent の技術（設計・pure
パッケージ）を移植元とするが、フォークではない。Vertex AI 固有機能を再現しないことは
仕様として許容する。

## 2. Functional Specification

### Commands / API Surface

gem-agent の表面を基準に、実験目的に必要なものへ絞る。

| コマンド / フラグ | 内容 |
|---|---|
| `lagent` | 対話 TUI を起動 |
| `lagent "<初手>"` | 位置引数を第 1 ターンとして送信してから対話へ |
| `lagent -p "<prompt>"` | 単発モード。stdout にモデルテキストのみ |
| `--continue` | 直前セッションの transcript から再開 |
| `--auto` | 規則層で Safe と判定した呼び出しを自動承認 |
| `--version` | 版数（`git describe` 由来） |

実装された Phase 1 にはこのほか `--resume`、`--allow`、`--read-only` /
`--writable`、`--model`、`--mcp`、`--config`、`--no-sandbox` と、
`sessions` / `trust` / `workdirs` / `version` サブコマンドがある。現行の表は
`reference/configuration.md`。

Phase 1 の組込ツール: read_file / write_file / edit_file / list_files / list_tree /
search_files / file_info / shell_exec / ask_user。MCP サーバのツールは
`.mcp.json` から接続して追加する。ADR-0005 により修正: `view_image` が一覧に
加わり、ADR-0004 が `mcp_load` を加える。

### Input / Output

- stdout はモデルテキストのみ。バナー・ツールイベント・承認プロンプトは stderr。
- `-p` は非端末 stdin を EOF まで読む（gem-agent と同じ契約）。
- セッションは JSONL transcript。レコード種別は gem-agent と同じ命名を保ち、
  `--continue` はこれを読む。
- usage レコードは gem-usage-lens が読める形式で書く（`prompt` / `output` /
  `tool_prompt` / `total` の 4 項。ローカルでは `tool_prompt` は常に 0）。
  実装ではレコードは lens の 6 項 — この 4 項に `thoughts` と `cached` を
  加えたもの — を運ぶ（`reference/architecture.md` に列挙）。
  費用はゼロなので、比較軸はトークン数・壁時計時間・ターン数・
  キャッシュヒット（初回応答時間から推定）。

### Configuration

`~/.config/lagent/config.toml`。strict decode（未知キーはエラー）。
優先順位: フラグ > `LAGENT_*` 環境変数 > ファイル > 既定。

```toml
[llm]
provider = "lmstudio"        # lmstudio | ollama | openai
base_url = "http://localhost:1234/v1"
model    = "google/gemma-4-26b-a4b-qat"
api_key  = ""                # 任意。ローカルサーバでは不要

[model]
context_window = 0           # 0 = provider から自動検出
```

`provider` はコンテキスト長の自動検出先を選ぶだけ（LM Studio は `/api/v0/models`、
Ollama は `/api/show`）。会話は全て OpenAI 互換の `chat/completions`。
`context_window` を明示すれば provider に依存しない。実装ではファイルは
`[sandbox]`、`[agent]`、`[mcp]`、`[tui]`、`[approval]` も持つ。全キーは
`reference/configuration.md` に列挙。

### External Dependencies

- ローカル LLM サーバ（LM Studio または Ollama）。資格情報なし。
- macOS の `sandbox-exec`（gem-agent と同じ封じ込め）。
- Go 依存: stdlib + cobra + BurntSushi/toml + nlk。OpenAI SDK は使わない。
  実装では ADR-0001 で移植したインライン TUI が Bubble Tea、bubbles、
  lipgloss、glamour を伴った。

## 3. Design Decisions

### 言語・依存

Go。移植元の gem-agent と llm-cli が Go で、sandbox / risk / bounded / approve /
session / mcp / tools / tui を関数単位で持ち込める。OpenAI 互換クライアントは
llm-cli と同じ REST 直叩き（stdlib `net/http`、SSE 手書き）。組織のサプライチェーン
方針により、コミュニティ製 SDK・ラッパーは採用しない。

### 既存ツールとの関係

- **gem-agent:** 対照実験機。同じタスクを両者で走らせ、usage レコードを並べて比較する。
- **llm-cli:** OpenAI 互換クライアント実装の移植元。
- **gem-usage-lens:** 両者の usage レコードを同じ物差しで読む。

### 移植の形

フォークではなく新規リポジトリ。gem-agent の pure パッケージを移植元として個別に
持ち込み、持ち込んだ元コミットを記録する。gem-agent 側の変更に追随する義務は負わない。
旧フォーク（`_wip/local-agent`、2026-09-09 に廃棄）の失敗は「提供元機能を注入点 nil で
塞いだ縫い目」と「参照文書が gem-agent の挙動を現行として記述」の 2 クラスだった。
lagent では持ち込まない機能のコードと文書を最初から持たない。

### プロンプト

ADR-0003、ADR-0004、ADR-0005 により修正: プロンプトは現在、それらの記録が
挙げる点（セッション単位の事実をランタイムの冒頭メッセージへ移動、MCP 目録の
一文、`view_image`）で gem-agent のものと異なる。以下の計測はそれ以前のもの。

gem-agent のシステムプロンプトをそのまま入れて第 1 計測点とする。実測では
gem-agent 自身のプロンプト（約 1.2k トークン）+ 56KB の AGENTS.md + 組込ツール 15 個で
18,397 トークン、262k 窓の 7%。同じプロンプトで比較しなければ、差がモデル由来か
指示文由来か分からない。ローカル向けの書き直しは計測結果を見てから ADR で判断する。

### 自動承認

Phase 1 は規則層 + 人間承認のみ。gem-agent のモデル層（別モデル呼び出しによる risk 審査）は
ローカルでは同じモデルへの追加呼び出しになり 1 回あたり数秒から数十秒かかるため、
Phase 2 で計測してから採否を決める。

### 対象外（明示）

- web_search / web_fetch（Vertex の grounding と URL context）
- GCS メディアアップロード、Cloud Logging テレメトリ
- thought signature、safety 設定
- Linux / Windows、GUI
- 複数バックエンドの同時接続
- 配布・チーム利用（実験段階）

## 4. Development Plan

### Phase 1: Core

- OpenAI 互換バックエンド: stream、`tool_calls` delta の組み立て、
  `finish_reason=length` は部分結果として扱う（全破棄しない）、途中で切れた
  transcript の再開
- エージェントループ、組込ツール、sandbox、承認ゲート、MCP クライアント
- JSONL transcript と `--continue`、`-p`、TUI
- usage レコード（gem-usage-lens 互換）
- provider 別のコンテキスト長自動検出（LM Studio / Ollama）
- 全項目にテスト。独立レビュー可能。

**完了判定:** gem-agent で実施済みの E2E 手順（ツールコール往復、承認フロー、sandbox
強制、MCP 実往復、AGENTS.md 注入、`-p` 両経路、pty 経由 TUI）を lagent で全て通し、
同じタスクを両者で走らせた usage レコードを gem-usage-lens で並べる。

計測結果（2026-09-10）: E2E 側は全て通った。並べる側は比較にならなかった:
同じ指示に対して Gemini はツールループで作業するのに、Gemma 4 は 1 ラウンドで
答えるため、2 つの transcript は違う仕事を記録しており、トークン数を突き合わ
せられない。この判定の互換性側は `gem-usage-lens verify --sessions-root
<lagent sessions>` がチェックサム失敗を報告しないこと（transcript 11、
レコード 37、NG 0）に縮める。この所見そのもの — ローカルモデルが多段のツール
作業に入らない — が Phase 2 の判断材料となる実効性の結果で、コスト比較は
その後になる。`verify` は lens の store を開かずに transcript を読む。
`ingest` は開くので、lagent の transcript を操作者の実 store に向けることは
しない。

### Phase 2: Features

各項目は Phase 1 の計測結果を根拠に ADR で採否を決める。

- 履歴圧縮（KV キャッシュ破棄コストを計測してから設計）
- 自動承認のモデル層 — 計測のうえ採らず（ADR-0010）
- skills — 採用、Claude Code の形式のまま lagent 自身のディレクトリから（ADR-0011）
- pre-tool hooks — 採用、Claude Code の計測済み契約で 1 イベント（ADR-0012）
- agent memory
- ローカル向けプロンプト改訂
- Gemma 4 の thinking トグル

### Phase 3: Release

- README.md / README.ja.md、CHANGELOG、AGENTS.md
- 健全性チェック手順（gem-agent の drill 相当）
- 署名・notarize、`_wip` から lab-series へ統合

## 5. Required API Scopes / Permissions

None。外部サービスなし。LM Studio / Ollama はローカルの無認証 HTTP。`api_key` は
OpenAI 互換の他サーバ向けの任意項目。macOS の sandbox-exec は追加権限不要。

## 6. Series Placement

Series: lab-series
Reason: 実験用途、利用者は本人、目的はコストと実効性の検証。gem-agent も lab-series で
始まり実戦投入実績で cli-series に昇格した前例に従う。実用性が示せた時点で lite-series
（ローカルファースト LLM ツール）への昇格を再検討する。

## 7. External Platform Constraints

2026-09-09〜10 の実測（LM Studio、Gemma 4 26B A4B QAT、MLX 4bit、Apple M2 Max 64GB）。

- プロンプト処理は約 580 tok/s。キャッシュミス時の初回応答は 1k トークンあたり約 1.7 秒
  （21k トークンで 36 秒）。生成は約 50 tok/s。
- LM Studio の KV キャッシュは同一接頭辞の再送で効き（1 秒）、履歴圧縮や並列スロットの
  切り替えで失われる。実効ウィンドウは 262k ではなく数万トークンで設計する。
- ストリーミングで tool call 引数は 1 チャンクで届き、生成中は無音（Vertex の
  「1 part」と同じ特性）。
- 並列 tool call、role=tool の 2 ラウンド目、`response_format: json_schema`、
  base64 画像入力、24 ツール定義 + 21k トークン prompt でのツール選択、90 行の
  write_file 引数、edit_file の完全一致文字列は全て動作確認済み。
- 未検証: Gemma 4 の thinking トグル、`finish_reason=length` 時の挙動、
  Ollama の tool calling とコンテキスト長検出、nonce 注入防御の指示追従。

---

## Discussion Log

- **2026-09-09:** gem-agent 技術をベースにローカル LLM ランタイムを新造する構想。
  実機（LM Studio + Gemma 4 26B A4B QAT）で API 契約を実測し、実現可能と判断。
  gem-agent の Vertex 直依存は 4 ファイルに閉じ、`llm.Backend` は 1 メソッド。
- **2026-09-09:** 旧フォーク `_wip/local-agent` は品質不足で廃棄。フォークではなく
  別プロダクトラインとして新造する方針に転換。
- **2026-09-10 項目 1:** 利用者は本人、実験用途。動機はコストと実効性の検証。
- **項目 2:** Phase 1 の機能をループ + 組込ツール + sandbox + 承認 + MCP + transcript +
  `-p` + TUI に限定。自動承認は規則層のみ。usage レコードは gem-usage-lens 互換に
  すると利用者判断。
- **項目 3:** Ollama も config で選べるようにする（利用者判断）。プロンプトが入るかを
  先に測る（利用者指摘）→ 18,397 トークンで 262k 窓の 7%、gem-agent のプロンプトを
  そのまま第 1 計測点にする。ツール名は `lagent`（利用者決定）。
- **項目 4:** Phase 1 完了判定を gem-agent の E2E 手順の全通過 + usage レコード比較とする。
- **項目 5〜7:** 外部サービスなし。lab-series（利用者決定）。制約は実測値を記載。
- **不採用案:** gem-agent への複数バックエンド追加（cli-series 安定契約下の本体を触る）、
  共通ライブラリ抽出（同上）、lite-series 配置（実験段階では早い）。
