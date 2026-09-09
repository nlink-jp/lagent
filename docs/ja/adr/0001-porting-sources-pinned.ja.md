# ADR-0001: gem-agent と llm-cli はコミットで固定した移植元

| Field | Value |
|-------|-------|
| Status | **Accepted** |
| Date | 2026-09-10 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | gem-agent との関係（フォーク・共通ライブラリ・移植元を記録する新規リポジトリ）を決めるまで、最初のパッケージを持ち込めない |

## Context

lagent は gem-agent の設計をローカル LLM に載せたもの（RFP §1）。2 つの
コードベースの関係には 3 つの形があり得た。

1. **フォーク**: gem-agent のスナップショットを取り Vertex バックエンドを
   置き換える。これは試された（`_wip/local-agent`、2026-09-09 廃棄）。
   分離レビューは、引き算のフォークが構造的に生む 2 つの失敗クラスを
   見つけた。提供元機能を注入点 nil で塞いだ縫い目（メディアアップロード、
   web ツール、`[gcp].bucket` を指すエラー文、dead 型）と、gem-agent の
   挙動を現行として記述する参照文書である。
2. **共通ライブラリ**: gem-agent からパッケージを抽出する。gem-agent は
   cli-series で安定契約下にあり、実験のためにリリース済みツールを
   分割するのは破壊的再構成になる。
3. **新規リポジトリ**: gem-agent の pure パッケージを 1 つずつ、移植元
   コミットを記録して持ち込む。

llm-cli の OpenAI 互換クライアント（stdlib `net/http`、手書き SSE、SDK なし）
がバックエンドの第 2 の移植元。

## Decision

lagent は新規リポジトリとする。gem-agent と llm-cli は**移植元であって
upstream ではない**。

- 移植元はこの記録を書いた時点でレビューしたコミットに固定する。

  | Source | Tag | Commit |
  |---|---|---|
  | gem-agent | v0.74.0 | `be7609980022e38314268c58ca94a6517e6f5d28` |
  | llm-cli | v0.2.0 + 7 | `6237a64ce4595a6e3cc5690f9fdddf6a6b407b84` |
  | nlk | v0.5.2 | （Go モジュール、版数で固定） |

- 持ち込んだパッケージは package doc コメントに移植元を明記する: 元
  リポジトリ、パッケージパス、上記コミット。新規に書いたパッケージは
  何も書かない。
- 移植は lagent が必要とするものの複製であり、移植元が持つものの複製
  ではない。ADR-0002 の一覧にある機能への参照は、ここでコンパイルする前に
  すべて落とす — コード、設定キー、エラー文、doc コメント。
- 以後、移植元に追随しない。gem-agent 側の後続修正は、誰かが移植を決めた
  ときにだけ、元コミットを明記した独自コミットとして入る。
- Phase 1 が必要とする順の移植候補: sandbox、bounded、tools、risk（規則層
  のみ）、approve、session、mcp、repl、tui、uitext、config（ローダの形）、
  instructions、mention、ignore、archtest。バックエンド（`internal/llm`）は
  llm-cli のクライアントを軸に新規に書く。

## Consequences

- 2 つのランタイムは自由に乖離できる。それが目的である: lagent の計測は
  ずれていくコードベースではなく、モデルに帰属できなければならない。
- 移植したパッケージは gem-agent のテストを伴うので、到着時に同じ挙動で
  検証される。
- フォークの 2 つの失敗クラスは、それを生んだ機構ごと再発できない: 引き算
  する upstream ツリーも、継承する upstream 文書も存在しない。
- 代償は重複。gem-agent で直った欠陥が、誰かが移植するまで lagent に残り得る。

## Alternatives considered

- **フォーク (1)** — 廃棄した試みの証拠により却下。
- **共通ライブラリ (2)** — 却下: 実験のために cli-series 安定契約下の
  リリース済みツールを触る。lagent が昇格し、重複のコストが計測されたときに
  のみ再検討。
- **gem-agent への OpenAI バックエンド追加** — 同じ理由で却下。加えて
  gem-agent の憲章はバックエンド 1 つ。

## References

- RFP §3「移植の形」— `docs/ja/lagent-rfp.ja.md`
- ADR-0002 — 移植が持ち込んではならない機能
