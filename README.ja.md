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

> **状態: 実験中（lab-series）。** 未リリース。scaffold は `--version` に
> 答えるだけで、エージェントループは [RFP](docs/ja/lagent-rfp.ja.md) の
> 開発 Phase 1。

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
