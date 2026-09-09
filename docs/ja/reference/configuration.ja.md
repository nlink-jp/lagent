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
