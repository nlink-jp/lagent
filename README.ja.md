# lagent

**ローカル LLM** で動くサンドボックス付き CLI エージェントランタイム。
OpenAI 互換 API（LM Studio、Ollama）を通してモデルに接続し、ファイルの
読み書き、サンドボックス内のシェルコマンド、MCP サーバを扱う。変更を伴う
呼び出しは操作者が承認する。

lagent は [gem-agent](https://github.com/nlink-jp/gem-agent) とは別の
プロダクトラインで、設計は同じもの: 監査可能な最小ループ（read / edit /
shell / MCP / 承認）、対象プロジェクトの AGENTS.md / CLAUDE.md / .mcp.json を
そのまま読む drop-in 互換、同じセッション記録。ローカルモデルがエージェント
ランタイムをどこまで担えるかを、コスト（トークン数・壁時計時間・ターン数）と
実効性の両面で、gem-agent と同じ物差しで測るために存在する。

> **状態: 実験中（lab-series）。** [RFP](docs/ja/lagent-rfp.ja.md) の
> Phase 1 コア — ループ、ツール、sandbox レーン、承認、MCP、セッション、
> TUI — が入った。リリースは署名・notarize 済みの darwin/arm64 アーカイブを
> 伴う。gem-agent との並列計測は比較にならなかった（RFP §4）: Gemini が
> ツールループで作業するところをローカルモデルは 1 ラウンドで答える。
> その所見が Phase 2 の出発点。

English: [README.md](README.md)

## 動作要件

- Apple silicon の macOS（隔離は `sandbox-exec` に基づく）
- OpenAI 互換 API を持つローカル LLM サーバ:
  [LM Studio](https://lmstudio.ai/)（検証済みバックエンド、モデル
  `google/gemma-4-26b-a4b-qat`）または Ollama
- 資格情報は不要

## 設定

`~/.config/lagent/config.toml` — [config.example.toml](config.example.toml)
を参照。優先順位はフラグ > `LAGENT_*` 環境変数 > ファイル > 既定。未知の
キーはエラー。

```toml
[llm]
provider = "lmstudio"        # lmstudio | ollama | openai
base_url = "http://localhost:1234/v1"
model    = "google/gemma-4-26b-a4b-qat"

[model]
context_window = 0           # 0 = provider から自動検出
```

## クイックスタート

LM Studio を起動してモデルをロードし、プロジェクトディレクトリで lagent を
実行する。

```bash
cd ~/work/my-project
lagent
```

初回起動でプロジェクト自身の AGENTS.md / CLAUDE.md / .mcp.json を信頼する
かを尋ねる。変更を伴うツールは実行前に尋ねる。`--auto` で規則層が Safe と
判定した呼び出しは尋ねずに走る。`-p "…"` はプロンプト 1 つを実行して終了。
`/help` でスラッシュコマンド一覧。

## できること

- **ツール:** `list_files`、`list_tree`、`search_files`、`read_file`、
  `file_info`、`view_image`、`write_file`、`edit_file`、`shell_exec`、
  `ask_user`、および
  `.mcp.json` の MCP サーバが提供する全ツール — モデルには目録として見せ、
  サーバごとに `mcp_load` を呼んだ時点で広告する（ローカルモデルは毎ターン
  243 スキーマを読む余裕がない。`[mcp].preload` と `[mcp].advertise = "all"`
  が操作者の調整点）。
- **封じ込め:** ファイルツールはプロジェクト（とセッション作業ディレクトリ）
  の内側に留まる。`shell_exec` は宣言したレーンで `sandbox-exec` 下で走る —
  read は尋ねずに（inspection と、キャッシュをセッション scratch に持つ Go の
  ビルド・テスト）、write と operator は尋ねてから。
- **セッション:** セッションごとの JSONL transcript。`--continue` と
  `--resume`。usage レコードは
  [gem-usage-lens](https://github.com/nlink-jp/gem-usage-lens) が両ランタイム
  に対して読む形。
- **無いもの:** web 検索と取得、メディアアップロード、監査ログ出力、
  履歴圧縮、skills、agent memory、hooks — RFP と
  [ADR-0002](docs/ja/adr/0002-features-not-reproduced.ja.md) を参照。

## 添付

`@<path>` はプロジェクト内のファイルやディレクトリを、`@<image>` はどこに
あっても画像を添付する（絶対パス・`~` パス可）。端末にドロップした画像の
パスは `@` 無しでも添付され、エスケープ済みの空白も読む。読めない参照は
操作者に警告され、モデルにも伝えられる。

## インストール

Apple Silicon Mac に nlink-jp の Homebrew tap から（署名・notarize 済みの
リリースアーカイブをそのまま入れる）:

```bash
brew tap nlink-jp/tap
brew install nlink-jp/tap/lagent
```

## ビルド

```bash
make build      # → dist/lagent
make test
make check      # vet + lint + test + docs ミラー + リリースゲート + build
```

## ドキュメント

- [docs/ja/INDEX.ja.md](docs/ja/INDEX.ja.md) — 仕様（RFP）、リファレンス、ADR

## ライセンス

MIT
