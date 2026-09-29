# ADR-0025: 端末が絵を描ける場所では、mermaid のフェンスを絵として描く

| 項目 | 値 |
|------|----|
| Status | **Proposed**（2026-09-29） |
| Date | 2026-09-29 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | 運用者:「画像はトランスクリプトの描画として使う。モデルのツールではなくランタイムの描画機能に」— gem-agent と lagent の両方に（mermaid-render の RFP）。gem-agent ADR-0092 が gem-agent v0.85.0 で出荷した |
| Relates to | gem-agent ADR-0092（決定・実測・レビューをここへ移植）、[ADR-0020](0020-inline-images-declare-their-height.ja.md)（宣言するボックスのレーン。**§3 の「返答は分割されない」と A1 をここで改める**）、[ADR-0022](0022-showing-is-an-act-of-output.ja.md)、[ADR-0024](0024-outside-text-is-made-inert-for-the-terminal.ja.md)（無害化の入口と描画関数の出力の抑え）、[ADR-0001](0001-porting-sources-pinned.ja.md)（移植元の固定） |

## Context

分析・代替案・運用者の判断・iTerm2 と kitty での実測は gem-agent ADR-0092 にあり、ここでは繰り返さない。
このランタイム固有なのは、出発点の違いである。

### こちら側で違うこと

- **罫線のレーンが無い。** gem-agent は絵より前に mermaid のフェンスを mermaid-ascii の罫線で描き
  （その ADR-0042/0063）、絵を描けない場所ではそのレーンを残している。このランタイムには元から無い:
  ADR-0020 A1 は `internal/diagram` を前提として移植することを退け —「図が来るなら、ここで作るレーンに
  加わる」— ADR-0024 は図を描かないと記録している。返答中のフェンスは、モデルが書いたとおり Markdown の
  コードとして見える。
- **モデルの文字列は TUI の入口で無害化され、Markdown の描画関数の出力は自身のスタイルだけに抑えられる**
  （ADR-0024 §2〜§3）。絵の payload はこのランタイムが自分で書くエスケープ列であり、ツールの画像と同じく、
  その抑えを通らずに端末へ届かなければならない。
- **返答は分割されない**（ADR-0020 §3）: `newGlamourRenderer` は返答を 1 つのまま描き、区切りのレーンが
  運ぶのはツールの画像だけである。

エンジンは同じ組織のライブラリ `github.com/nlink-jp/mermaid-render` v0.1.0、端末も同じ 2 つで、同じ運用者が
測った。

## Decision

gem-agent v0.85.1（`8d7c780`）から移植する（ADR-0001）。

1. **TUI が画像を描く場所（iTerm2 か kitty、ADR-0020 §7）では、返答中の mermaid フェンスは絵になる。**
   mermaid-render が書かれたままのソースから描く。それ以外（プロトコル無し・`images = "auto"` での
   マルチプレクサ・素の REPL・`-p`）では、フェンスはいまと同じくソースのまま見える。**罫線のレーンは
   足さない**: mermaid-ascii はこのランタイムが持たないコミュニティ製のモジュールで、文字で間違って描く
   描画器こそ gem-agent の絵が置き換えたものである（gem-agent ADR-0092 B1/B4）。
2. **返答を分割する**（`diagram.Split`。ここでは絵だけ）: 返答の描画は行数を宣言した区切りを返し、
   `takeLive` と 4 つの呼び出し元はそれを 1 回の書き込みとして渡し、絵の区切りは行数をカウンタに伝える
   （ADR-0020 §1）。これは ADR-0020 §3 の「返答は分割されない」を改める: 返答はいまや分割され、画像は
   返答の中からも届きうる。
3. **ボックス・帯・上限は gem-agent ADR-0092 §4 の実測どおり**: 図の文字 1 em を端末の 1 行に（1 em =
   28 px、mermaid-render の Scale 2）、幅は `TIOCGWINSZ`（ioctl。問い合わせではない。ピクセルを返さない
   端末では 2.25）で読んだセルの縦横比から。端末の幅 − 1 桁より広い絵は縮め、高い絵は縮めずに流す —
   kitty は画面より高い絵を切るので画面の半分以下の帯に分け、帯に継ぎ目が出た iTerm2 には 1 枚のまま渡す。
   エンコード後の payload の合計は `termimg.MaxBytes` を超えてはならない。
4. **失敗はすべてソースと注記 1 行を見せ、何も無くはならない**（gem-agent ADR-0092 §5）: 構文の誤り・
   未対応の構文・どのフォントにも無い文字・上限・エンジン自身の確かめを破る配置・作れない payload・
   エンジンの panic。未対応の図の種類（gantt・class など）は黙ってソースのまま。
5. **フォントは cmd 層が一度だけ、画像を描く場所でだけ読む**（`[tui.diagram]`: `font`・`font_name`・
   `bold_font`・`bold_font_name`。既定はヒラギノ角ゴシック W3 / W6）。読めない設定はバナーの警告 1 行と
   既定のフォントで、起動は止めない（gem-agent ADR-0092 §6 の運用者の判断）。既定のフォントも読めなければ
   フェンスはソースのまま。ユーザ設定だけ。
6. **無害化は ADR-0024 の決定どおり**: エンジンが読むフェンスの文字列は入口を通過済み。注記は Markdown
   として描画関数とその抑えを通る。絵の payload は `internal/tui` がエンジンのピクセルから作り、ツールの
   画像と同じく描画関数の横から `emitSegments` へ渡り、それを通らない。`termimg.Payload` を呼べるのは
   引き続き `internal/tui` だけ。
7. **ランタイムは図について引き続き何も言わない**: ツールもプロンプトの段落も無い。

## Consequences

- iTerm2 と kitty では返答の図が読めるようになる。日本語のラベルも。
- **既知の制約（gem-agent で実測、ADR-0092 §4）**: ウィンドウを狭めると、そのとき画面にある絵が失われ、
  スクロールバックに黒い空白が残る。TUI が縮小時に画面を消去するためで、このランタイムのリサイズ処理も
  同じである。gem-agent の見直しとあわせて直す。
- lagent は `github.com/nlink-jp/mermaid-render`（組織のモジュール）に、それを通じて `golang.org/x/image`
  に依存する。
- ADR-0024 の「このランタイムは図を描かない」は「絵としてだけ描く」になる。その結論 — 罫線の抑えは要らない
  — は変わらない。罫線はいまも無いからである。

## Alternatives considered

**A1. gem-agent と同じく罫線のレーンも移植する。** 退ける: 罫線は gem-agent で使われないと実測され
（その ADR-0092 の Context）、mermaid-ascii はコミュニティ製のモジュールで、画像を描けない端末ではフェンスが
ソースとして見えるのは、素っ気なくても正しい。

**A2. gem-agent の縮小時の消去の見直しを待ってから移植する。** 退ける: 制約は両側で同じで、直すときは
両方で直る。絵はいま役に立つ。

## References

- gem-agent ADR-0092（§1〜§8、§4 の実測、既知の制約）。
- mermaid-render の RFP（lib-series）。
