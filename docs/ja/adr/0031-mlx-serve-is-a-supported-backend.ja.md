# ADR-0031: mlx-serve を対応バックエンドとする

| 項目 | 値 |
|-------|-----|
| Status | **Accepted**（2026-10-07）— 実装済み。下の計測に基づいて採用 |
| Date | 2026-10-07 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | 運用者が日常の利用で lagent を mlx-serve に向けて（`provider = "openai"`、窓は手書き）問題なく使っており、本人の感覚では LM Studio より速い。バックエンドとして正式に採用するよう求めた |
| Relates to | [ADR-0006](0006-measurement-bench.ja.md)（タスクベンチ）、[ADR-0007](0007-empty-completion-asked-again.ja.md)（空の completion。ここで改める）、[ADR-0009](0009-thinking-is-an-operator-key.ja.md)（`reasoning_effort`）、[ADR-0023](0023-promotion-to-cli-series.ja.md)（安定性の約束） |

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

## Decision

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
4. **空白だけの completion は空とする**（ADR-0007 を改める）。比較の LM Studio 側で
   見つかった: そこでの Qwen 3.6 の回答は思考の後の空行で始まり、1 回の実行では最終回答の
   全体が `\n\n` だった — 空の判定はそれを回答として通していた。

## Verification

2026-10-07 に M2 Max（64 GB）、mlx-serve 26.10.1 と LM Studio、Qwen 3.6 35B-A3B 4-bit
で計測した。「同じ重み」は文字どおりで、LM Studio の `qwen/qwen3.6-35b-a3b` と
mlx-serve の `lmstudio-community/Qwen3.6-35B-A3B-MLX-4bit` はディスク上の同じ
ディレクトリである。「MTP」は mlx-serve 独自のビルド
（`ddalcu/Qwen3.6-35B-A3B-MLX-Serve-4bit`）で、複数トークン予測ヘッドを持ち、
日常の利用が動かしている構成である。mlx-serve は運用者の設定（prefill の一部を
Neural Engine に分担、前置きキャッシュはメモリ 2 GB とディスク）で、LM Studio は
262k の文脈・並列 4 で動かした。サーバは一度に 1 つだけ読み込んだ。

**タスクベンチ**（7 タスク — ADR-0006 の 6 つと `skill-follow` — を各 3 回、思考は
`reasoning_effort = "medium"` でオン、
`bench/configs/lagent/{mlxserve-qwen36-mtp,mlxserve-qwen36,lmstudio-qwen36}.toml`）:

| 構成 | 完了 | 壁時計、21 回 | 失敗 |
|---|---|---|---|
| mlx-serve、MTP | 19/21 | 294 s | `read-edit` 1（修正を説明しただけで行わない）、`view-image` 1（隔離タグの名前を答えた） |
| mlx-serve、同じ重み | 19/21 | 337 s | `read-edit` 1、`multi-file-rename` 1（どちらも変更を説明しただけで行わない） |
| LM Studio、同じ重み | 14/21 | 324 s | `skill-follow` 3（空行 2 つの後に正しい行 — 形式の検査が行頭に固定されている）、`read-edit` 2、`multi-file-rename` 2（1 回は回答が `\n\n` だけ） |

MTP の構成で繰り返すと、`read-edit` は 5/6、`view-image` は 6/6 で完了した: タグ名の
回答は 9 回中 1 回で、再発していない。「説明しただけで行わない」は ADR-0008 が
Gemma 4 で測った型で、モデルの振る舞いであり、両方のサーバの下で見えた。mlx-serve の
どの実行も、空の completion、壊れたツール呼び出し、引数のエラーを返さなかった。

**サーバ**（`bench serve`、2 回の中央値。プロンプトのトークン数は報告どおり —
サイズ区分 1k/12k/35k/69k は 966、11,185、31,694、60,333 になった）:

| | mlx-serve、MTP | mlx-serve、同じ重み | LM Studio、同じ重み |
|---|---|---|---|
| 初回の読み込み、11k | 11.5 s | 11.4 s | 15.5 s |
| 初回の読み込み、32k | 46.0 s | 45.4 s | 49.0 s |
| 初回の読み込み、60k | 124.4 s | 122.9 s | 110.6 s |
| 再送、60k | 0.9 s | 0.8 s | 1.5 s |
| 次のターン、60k | 1.3 s | 1.2 s | 1.5 s |
| 生成、1k | 158 tok/s | 106 tok/s | 85 tok/s |
| 生成、11k | 110 tok/s | 93 tok/s | 81 tok/s |
| 生成、60k | 65 tok/s | 61 tok/s | 57 tok/s |
| 同時 2 本、1k（合計） | 168 tok/s | 115 tok/s | 125 tok/s |
| 5 つの 32k の会話のうち最初のものへ戻る | 27.7 s（16k を再利用） | 27.6 s（16k を再利用） | 1.1 s |

- **速さはランタイムそのものから来ているのではない。** 同じ重みでは、〜32k までの
  プロンプトは mlx-serve が速く読み、60k は LM Studio が速く読む。生成は mlx-serve の
  下で 7〜25 % 速い。大きな差は mlx-serve 独自ビルドの MTP ヘッドによる: 1k で
  LM Studio の生成の 1.9 倍、11k で 1.4 倍、60k では縮まる（1.1 倍）。
- **長い文脈は持ちこたえる。** 初回の 122,019 トークンのプロンプトが mlx-serve で
  完了した（414 s、生成 39 tok/s）: Magnitude を外した GPU 監視による停止
  （2026-10-04）は起きなかった。
- **メモリキャッシュは設定であり、その既定は lagent には小さすぎる。** 既定の 2 GB
  （アプリの「Auto」— 長文脈のハイブリッドモデル向けに大きくするという説明は 1 つの
  アーキテクチャにしか効かない）では、32k の会話 5 つは収まらず、122k の会話は 1 つでも
  収まらない: 再送は 122,019 トークン中 81,920 を再利用して 207 s、次のターンは
  98,304 で 131 s、5 つの会話の最初のものへ戻るのに 27.7 s かかった。Qwen 3.6 は
  1 トークンあたり約 26 KB を保持する（40 層中 10 層の attention の KV と、線形
  attention の状態）ので、122k は約 3.1 GB になる。同じサーバに
  `--prefix-cache-mem 8GB`（アプリの「Prefix cache memory cap」）だけを足すと —
  1 回の計測、他は運用者の設定のまま — 122k のプロンプトの再送は 1.2 s、次のターンは
  2.1 s、5 つの会話のそれぞれへ戻るのは 0.4〜0.5 s で、LM Studio の 1.1 s より速かった。
  キャッシュの最大使用量は 5.0 GB だった。
- mlx-serve は `cached_tokens` を報告し、LM Studio は報告しない。そのため LM Studio の
  記録は、前置きを再利用していてもキャッシュ 0 と出る。

## Residual risk

lagent はこれらを片付けられない。サーバを動かす運用者が引き受ける。

1. **サーバはコミュニティのソフトウェアである。** 作者は 1 人、リリースは毎週で、
   現行版には画像入力の退行の報告が未解決のまま出ている（#753）。アプリは自身の更新と、
   ダウンロードしたモデルパックの更新を確かめに行き、独自ビルドのモデル（`ddalcu/…`）は
   同じ作者が変換したものである。ここで LM Studio が配信する重みも第三者が変換したもの
   （`lmstudio-community`）なので、これは種類ではなく程度の差である。
2. **26.10.1 は既定で全インタフェースを鍵なしで待ち受ける**（ヘルプには、後の版で既定を
   `127.0.0.1` にするとある）。API には絶対パスでの `/v1/load-model` と
   `/v1/unload-model` が含まれる。運用者は `127.0.0.1` に絞るか API キーを設定する。
   基準機では、この検証が指摘するまで LAN に開いていた。
3. **サーバはツール呼び出しの引数を書き換える** — スキーマが宣言する型へ
   （`toolAutocorrect`、既定でオン）。Qwen のタグ形式の呼び出しを、LM Studio の
   パーサと同じく JSON に読み直す。lagent が受け取るのは、どちらのサーバでも、サーバに
   よるモデルの読み取りである。
4. **記録の出力トークンには思考が含まれる**（`reasoning_tokens` が無い）。同じ作業でも
   mlx-serve の下では LM Studio より多くの出力が記録される。
5. **前置きキャッシュの既定の予算では、長いセッションを読み直す。** キャッシュが 2 GB
   未満 — Qwen 3.6 でおよそ 8 万トークン — のうちは何も変わらないが、それを超えると毎ターン
   数万トークンを読み直す。メモリの上限（計測したのは 8 GB）は、RAM の許す範囲で運用者が
   設定する。lagent にはできない。
6. **打ち間違えたモデル ID は読み込み済みのモデルで動く。** provider は起動時に通知として
   報告するが、セッションは始まる — どの provider でも窓の問い合わせが失敗したときと同じ。

## References

- `internal/llm/openai.go` — `mlxServeWindow`
- `bench/serve.go` — サーバの計測。`bench/_results/serve-*` と `tasks-*` に生ログが
  ある（git には入れない）
- `internal/agent/agent.go` — 空の判定（決定 4）
