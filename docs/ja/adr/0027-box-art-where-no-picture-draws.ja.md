# ADR-0027: 絵を描かない場所では罫線で描く

| 項目 | 値 |
|-------|-----|
| Status | **Accepted**（2026-09-30）— 実装済み |
| Date | 2026-09-30 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | mermaid-render RFP 第 2 段階 2e（2026-09-30 の運用者の判断）: 自前の罫線描画器、「lagent にも入れる」 |
| Relates to | gem-agent ADR-0095（向こうで同じエンジンが mermaid-ascii を置き換える）、[ADR-0025](0025-mermaid-fences-render-as-pictures.ja.md)（**§1 の「罫線のレーンは足さない」と A1 を本記録で置き換える**。絵の経路はそのまま）、[ADR-0024](0024-outside-text-is-made-inert-for-the-terminal.ja.md)（**「罫線の抑えは無い」を本記録で改める: `inertArt` を持ち込む**）、[ADR-0020](0020-inline-images-declare-their-height.ja.md)（A1 の「図が来るなら、ここで作るレーンに加わる」がここで起きる） |

## Context

ADR-0025 は、端末が画像を描く場所では mermaid のフェンスを絵にし、それ以外ではソースのままにした。罫線のレーンを
断った理由は 2 つ: mermaid-ascii はこのランタイムが持たないコミュニティ製のモジュールであること、そして文字で
間違って描く描画器こそ絵が置き換えたものであること。

どちらの理由も無くなった。mermaid-render は今、flowchart / graph・sequenceDiagram・erDiagram を罫線で描く
（`raster.RenderText`）。絵と同じ構文解析から描き、描くたびに格子の上で確かめ、欠陥は拒む。組織のコードで、
ここでも既に依存している。実データの 43 ブロックがすべて描け、運用者はそのうち 15 を 4 回の目視で通した
（gem-agent ADR-0095）。

## Decision

1. **TUI が画像を描かない場所では、この 3 種の mermaid のフェンスは罫線になる。** プロトコルが無い、
   `images = "auto"` で多重化ソフトの中: フェンスを書かれたまま `raster.RenderText` に渡し、
   `TextOptions.Width` は TUI 自身のセルの測り方（rune ごとの `ansi.StringWidth`）。画像を描く場所では
   ADR-0025 の絵の経路はそのまま。絵から罫線への代替は無い。素の REPL と `-p` はソースのまま。
2. **罫線は独立した区切り。** Picture の無い `diagram.Split` も返答を分割するようになる: 描けたフェンスは
   `Segment{Text: art, Art: true}`。TUI はそれをそのまま書き、glamour を通さない。glamour は行を空白で
   折り返して箱を崩すからである（gem-agent ADR-0063）。行数は宣言しない。文字の区切りも宣言しないのと同じ。
3. **罫線はエスケープを一切通さない**（`inertArt`、`inert.String`）: エンジンはエスケープを書かないので、
   罫線の中のエスケープは描いた文字から来たものである。ADR-0024 が今まで要らなかった、罫線の抑えである。
4. **結果は絵と同じ**（ADR-0025 §4）:
   - 未対応の種類は黙ってソースのまま。
   - それ以外のエラーやエンジンの panic は、注記 1 行つきでソースを見せる。

## Consequences

- 画像の無い端末（Terminal.app、多重化ソフト）で、この 3 種がソースではなく図として見える。
- 返答は、絵を描く場所だけでなく、あらゆる TUI で分割される。
- `internal/diagram` に罫線の経路ができ、`internal/inert` は罫線の抑えという呼び手を得る。ADR-0024 の
  テストがこの新しい呼び出しを固定する。
- East Asian の曖昧幅の文字を 2 桁で描く設定の端末では、罫線がずれる。問い合わせなしには分からず、
  gem-agent と同じく受け入れる。

## Alternatives considered

- **絵を描かない場所はソースのまま。** 却下: それは、描画器が間違って描くコミュニティ製しか無かった間の
  つなぎだった。運用者はレーンを求めた。
- **`-p` と素の REPL でも罫線にする。** 却下（gem-agent の ADR-0042 §4 と同じ）: それらはパイプと
  コピーのための出力で、モデルの文章のままにする。
