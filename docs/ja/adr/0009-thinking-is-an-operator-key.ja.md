# ADR-0009: thinking は操作者のキーであり、そのまま送り、既定にする前に計測する

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-12 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | ADR-0007 の再生実験: 同一リクエストが 10/10 で空になった地点で、thinking on は 0/10 だった — そしてランタイムには thinking を on にする手段が無かった |

## Context

RFP の Phase 2 は「Gemma 4 の thinking トグル」を計測で採否を決める項目に
挙げる。これまでランタイムは reasoning 設定を一切送っていなかったので、
すべての計測 — ベースラインベンチも空 completion のトレースも — は thinking
off のものだった: このモデルに対する LM Studio の既定である（設定無しの
リクエストで reasoning トークン 0 を実測）。

ADR-0007 第 2 改訂の背後にある再生実験がトグルの最初の効果を測った。検証
実行からコンパイルエラーを受け取った直後のリクエストで、同一リクエストは
10 回中 10 回空だった。`reasoning_effort` を設定すると 10 回中 10 回ツール
呼び出しで、呼び出しごとの中央値は 0.8 秒に対して 3.0 秒。一時的な促しの
1 行は 1.1 秒で同じ 0/10 に達し、それが ADR-0007 がトグルではなく行を対処に
した理由である。トグルがタスク全体 — ラウンド、トークン、壁時計時間、完了 —
に何をするかは未計測。

語彙はサーバのもの。LM Studio のエンドポイントは OpenAI の値（`none`、
`minimal`、`low`、`medium`、`high`、`xhigh`）を検証し、その後モデルが対応する
値へ丸める。Gemma 4 では on か off で、`none` が off、それ以外は on、サーバ
ログに丸めの 1 行が出る。`on` や `off` を直接送ると 400。別のサーバは別の
値を取る。ランタイムがその表を持つ理由は無い。

## Decision

1. **`[llm].reasoning_effort`（と `LAGENT_REASONING_EFFORT`）は設定時に
   リクエストの `reasoning_effort` として**そのまま送る**。未設定なら何も
   送らず、それがサーバの既定。ランタイムは値を検証も翻訳もしない: 語彙は
   サーバに属し、モデルへの丸めはサーバのもので、そのログが何をしたかを
   言う。`/settings` にキーを表示する。
2. **既定はベンチが言うまで未設定のまま。** ベンチ（ADR-0006）で 6 タスクを
   ベースライン構成と `reasoning_effort = "low"`（LM Studio: on）を設定した
   構成で、各 3 反復、ケースを外側・構成を内側で走らせる。数字は本記録の
   日付で `reference/bench.md` に入る。既定 on は出荷設定の変更であり、その
   数字で決めて本記録に改訂として記録する — 既に計測した 1 地点から仮定
   しない。
3. **reasoning の内容は決して保存しない。** バックエンドは既に
   `reasoning_content` を表示（`[tui].show_thoughts`）にだけ流し、履歴からは
   落とす。したがって thinking のターンは接頭辞キャッシュを元のまま残し、
   transcript は usage レコードに reasoning トークン数を持つだけである。

## Consequences

- thinking on は計測した地点でモデル呼び出しごとの遅延が約 3 倍になり、
  `output` に reasoning トークンが加わる。ラウンドを節約するかはベンチ実行が
  決める。
- 空 completion の不具合は計測した地点では thinking on で起きない。ADR-0007
  の行が同じ地点をごく小さな代価で覆うので、このキーはその不具合の対処では
  ない。
- ベンチ計測、2026-09-12: `reference/bench.md` を参照。

## Alternatives considered

- **ランタイムが provider ごとに翻訳する真偽値 `thinking = true|false`。**
  却下: ランタイムが表（LM Studio の丸め、OpenAI 流サーバの段階、次に来る
  何か）を持つことになり、それはサーバが既に持ちログに出す。そのままの
  文字列は、何を送ったかについて決して嘘をつかない唯一の形。
- **再生の結果を根拠に今すぐ既定 on。** 却下: 遅延 3 倍の 1 地点はタスク
  水準の計測ではなく、既定はベンチで決めるために ADR-0006 がある。
- **空 completion 後の再送だけ thinking を on にする。** 却下: 1 行の促しが
  3 分の 1 の遅延で同じ 0/10 に達し、サーバの語彙も要らない。
