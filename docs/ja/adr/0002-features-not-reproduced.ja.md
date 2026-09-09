# ADR-0002: lagent が再現しない gem-agent の機能

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-10 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | ADR-0001 は移植がこれらの機能のコード・設定キー・エラー文・文書を持ち込むことを禁じる。最初の移植の前に一覧が必要 |

## Context

gem-agent の Vertex AI / Google Cloud への直接依存は 4 ファイル
（`internal/llm/vertex.go`、`internal/llm/web.go`、`internal/mediastore`、
`internal/telemetry/gcp.go`）に閉じているが、それらが支える機能はツール、
設定、エラー文、`/usage` パネル、リファレンス文書にまで及ぶ。廃棄した
フォークはその縫い目を残した。この記録は各機能を名指しし、移植が認識して
落とせるようにする。

「再現しない」は仕様として受け入れる（RFP §1）。いずれも別名で Phase 2 の
候補になることはない。

## Decision

lagent は以下を再現しない。各行に、プロバイダに縛られている理由と、代わりに
lagent が行うことを記す。

| gem-agent の機能 | 縛り | lagent |
|---|---|---|
| `web_search`（Google Search grounding） | Vertex 組込ツール。検索はプロバイダ側で走る | web 検索ツールなし。必要なプロジェクトは MCP サーバを接続する |
| `web_fetch`（URL context） | Vertex 組込ツール。ページはプロバイダ側で取得され、それが SSRF 安全性の源でもあった | fetch ツールなし。ローカル取得は操作者の機械から localhost と LAN に届く。再発明するより無い方が安全 |
| `[gcp].bucket` メディア経路（音声・動画添付の GCS アップロード、向こうの ADR-0027） | `gs://` URI を直接読めるのは Vertex のみ | 画像は base64 でインライン添付（動作実測済み）。音声と動画は添付対象外 |
| `[telemetry].backend = "gcp"`（Cloud Logging） | ADC と Cloud Logging クライアント | Phase 1 にテレメトリなし。監査イベントを戻すなら OTLP のみ、ADR 経由 |
| thought signature の捕捉と再送 | Gemini 3 のワイヤ形式 | 再送するものがない。OpenAI 互換の tool call は `id` を持ち、ループは `tool_call_id` を返す |
| TUI の思考要約（`StreamEvent` の kind `thought`） | Gemini の thinking 出力 | サーバが送る `reasoning_content` フィールドは表示専用。thinking を要求するか自体が Phase 2 の計測項目 |
| `[model].safety`（コンテンツフィルタ閾値） | Vertex の safety 設定 | キーなし。ローカルサーバには設定すべきプロバイダフィルタがない |
| `[model].summary`（`summarize_file` 用の軽量モデル）と `agentic_file_search`（子エージェント） | クラウド価格表上で安い委任呼び出し | Phase 1 に含めない。単一ローカルモデルでは委任 1 回がプロンプト全走査 1 回分。採算は両方を戻す前に計測する |
| `tool_prompt` usage バケツ（`toolUsePromptTokenCount`） | 上記 2 つの組込ツール | フィールドは常に `0` で書く。gem-usage-lens が両ランタイムを 1 つのスキーマで読めるように |
| `GOOGLE_CLOUD_*` の優先層、`[gcp]` セクション | Vertex の認証と location | `[llm]` セクションと `LAGENT_*` 環境変数。ADC 探索はどこにもない |
| `us` / `eu` / `global` の location 規則 | Vertex のリージョン提供 | なし |

移植が上の行への参照を見つけたら、スタブにするのではなく取り除く — コード、
設定キー、それを指すエラーメッセージ、それを説明する文書の一文まで。

## Consequences

- lagent の設定に `[gcp]` セクションはなく、起動は最初のモデル呼び出しまで
  ネットワークに触れない。自明に: メタデータサーバも ADC も探るバケットもない。
- モデルに見せるツール一覧は Phase 1 で 2 つ少なく（`web_search`、
  `web_fetch`）、委任ツールを数えれば 4 つ少ない。gem-agent との比較は
  それらを要しないタスクで行う。
- 将来の「web アクセスが要る」は MCP サーバであって lagent の機能ではない。
- usage スキーマは常に 0 のバケツを持ったまま lens 互換を保つ。lens は 0 を
  「導出」と読んではならない（キー不在と 0 を区別する — 向こうの ADR-0066）。

## Alternatives considered

- **web ツールをローカルで再現**（操作者の機械から取得）— 却下: gem-agent が
  無償で得ていた SSRF 特性を反転させ、モデルにイントラネットと認証済み
  ページへの経路を渡す。
- **設定キーを「受理するが無視」で残す**（gem-agent 設定との drop-in 互換）—
  却下: 何もしないキーは縫い目であり、gem-agent の設定は drop-in の対象では
  ない（対象はプロジェクトの AGENTS.md / CLAUDE.md / .mcp.json）。
- **OTLP だけのスタブ `telemetry` パッケージ** — 保留: Phase 1 に監査
  イベントを出すものがない。生産者のないパッケージは dead code。

## References

- ADR-0001 — 移植元と縫い目禁止の規則
- RFP §3「対象外」、§7 実測制約
- gem-agent ADR-0027（メディア経路）、ADR-0035（テレメトリ）、ADR-0066
  （usage バケツ）— 各行が置き換える設計。固定コミット時点のもの
