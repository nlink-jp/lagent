# ADR-0031: mlx-serve を対応バックエンドとする

| 項目 | 値 |
|-------|-----|
| Status | **Proposed**（2026-10-07）— provider は実装済み。採否を決める計測は進行中 |
| Date | 2026-10-07 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | 運用者が日常の利用で lagent を mlx-serve に向けて（`provider = "openai"`、窓は手書き）問題なく使っており、本人の感覚では LM Studio より速い。バックエンドとして正式に採用するよう求めた |
| Relates to | [ADR-0006](0006-measurement-bench.ja.md)（タスクベンチ）、[ADR-0007](0007-empty-completion-asked-again.ja.md)（空の completion）、[ADR-0009](0009-thinking-is-an-operator-key.ja.md)（`reasoning_effort`） |

## Context

[mlx-serve](https://github.com/ddalcu/mlx-serve) は Apple silicon 向けの MLX
推論サーバで、Zig で mlx-c に対して書かれ、Developer ID 署名・公証済みのアプリとして
配布されている（MIT）。既定では `http://localhost:11234/v1` で OpenAI 互換の
`chat/completions` API を話す。作者はこの組織の外の個人である。

**サプライチェーン。** 組織はコミュニティのコードを採用しない（組織 ADR-024）。
lagent はそのコードを何も取り込まない: LM Studio と同じ HTTP の契約で、自前の
stdlib クライアントからサーバに話しかけるだけで、mlx-serve のものは何も同梱しない。
サーバは、LM Studio と同じく、運用者が動かすと選ぶものである。それを動かすことで
運用者が引き受けるもの — アプリ、その更新経路、配信する重み — は、lagent が
片付けるものではなく、下に残余のリスクとして記録する。

**通信路に載るもの（実測、mlx-serve 26.10.1、Qwen 3.6 35B-A3B）:**

- ツール呼び出しは 1 呼び出しにつき引数全体を持つ 1 つの差分で流れ、並列の呼び出しは
  別の index になり、最後に `finish_reason: tool_calls` が来る — LM Studio と同じ形で、
  組み立て側はすでに受け取れる。
- usage はストリームに載り（`stream_options.include_usage`）、
  `prompt_tokens_details.cached_tokens` を持つ。その横に `timings` オブジェクトが
  付く。`completion_tokens_details.reasoning_tokens` は**送られない**ので、記録の
  出力トークンにはモデルの思考が含まれる。
- 思考は `reasoning_content` で流れる。`reasoning_effort` は検証されない: `none` で
  思考が止まり、**それ以外の値は `off` や未知の語も含めてすべて思考を残す**。値を
  送らなければ Qwen 3.6 は思考する。
- `max_tokens` に達すると、思考の途中でも `finish_reason: length` でターンが終わる。
  `max_tokens` を省いても回答は切られない（1,491 トークンを実測）— サーバの
  `--max-tokens` のヘルプ文が示唆する挙動とは違う。
- **サーバが持たないモデル名には、読み込み済みのモデルが**ステータス 200 で**答える**。
  打ち間違えた `[llm].model` は黙って動いてしまう。
- `/v1/models` は配信できるすべてのモデルを `context_length` 付きで並べる。
- `image_url` の data URL（PNG、JPEG）の画像は、このモデルで読める。上流の報告
  （ddalcu/mlx-serve#753）によると、26.10.1 で Gemma 4 と Qwen 3.5-VL の画像入力が
  壊れた。

## Decision（案）

1. **`[llm].provider = "mlxserve"`。** コンテキスト窓は `/v1/models` のそのモデルの
   項目から読む。一覧に無いモデルは原因を名指すエラーにする。打ち間違えた ID が表に
   出る場所はこの問い合わせしかないからである（実装済み、`fbb786a`）。
2. **既定は `lmstudio` のまま。** 変えると既定に頼るすべての設定が壊れ、cli-series の
   安定性の約束（ADR-0023）は破壊的変更のプロセスの外でそれを許さない。
3. **採否は、日常の利用の感覚ではなく計測で決める:** タスクベンチ（ADR-0006）と
   サーバの計測（`bench serve`: 1k〜69k の初回のプロンプト読み込み、再送と次のターンでの
   再利用、生成速度、会話の切り替え、同時 2 本）を、**両サーバで同じ重み**で行い、
   ランタイムの差をモデルの差と混ぜない。さらに、日常の利用どおり mlx-serve 独自の
   ビルド（MTP ヘッド付き）でも測る。

## Verification

数字とともにここに記録する。

## Residual risk

ここに記録する。

## References

- `internal/llm/openai.go` — `mlxServeWindow`
- `bench/serve.go` — サーバの計測
