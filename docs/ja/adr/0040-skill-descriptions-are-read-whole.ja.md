# ADR-0040: スキルの説明は Claude Code と同じに読む — 1,024 文字まで、複数行の YAML も含めて

| 項目 | 値 |
|-------|-----|
| Status | **Proposed**（2026-11-04） |
| Date | 2026-11-04 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | gem-agent Issue #3（2026-11-04）: スキルの説明が 400 ルーンで切られ、1,024 文字の上限まで書いたスキルのトリガーが失われる。lagent の `internal/skills` は同じコードである |
| Ports | gem-agent ADR-0106（2026-11-04 Accepted、gem-agent v0.98.0 でリリース）、gem-agent `ac5d03e` 時点 |
| Amends | ADR-0011（長さを定めていなかった説明の行） |
| Relates to | ADR-0003（一覧はランタイムの事実のメッセージに載る）、ADR-0034（1 ラウンドを窓で上限づける） |

## 背景

ADR-0011 は、スキルを Claude Code の書式のまま読む。スキルごとに説明 1 行がランタイムの事実のメッセージ（ADR-0003）に載り、その行だけでスキルを読み込むかどうかが決まる。`internal/skills` は gem-agent から移植したもので、`f8bdb7e` で確かめると、コードが ADR-0106 以前の gem-agent と違うのは一覧の描き方だけである。したがって gem-agent ADR-0106 が見つけた欠陥を二つとも持つ:

1. **説明を 400 ルーンで切り、そのことを何も言わない**（`DefaultLimits().MaxDescription`）。Claude Code と org のスキル検証は 1,024 文字まで受け付ける。skills-series の 9 個のうち 6 個が 400 を超える（mcp-tactics 1,020、news-digest 644、compliance-review 594、weather-forecast 580、incident-review 578、incident-research 509）。切れ目より後には、たとえば mcp-tactics の BigQuery・ブラウザ・音声と、日本語のトリガーがすべて落ちる。
2. **複数行の説明を 1 行目として読む。** `description: >` の後に字下げした行が続くと、説明は `>` と読まれる。

## 決定

lagent は gem-agent ADR-0106 を、`ac5d03e` で実装されたとおりに採る:

1. **`MaxDescription` を 1,024 にする。** それより長い説明はこれまでどおり切るが、切ったことを発見時の注記にする: `skill "<name>": description is <n> characters; only the first 1024 are listed`。
2. **`parseFrontmatter` は複数行の値を読む**: ブロックスカラー（`>` は行を空白でつないで折りたたみ、空行は改行になる。`|` は改行を保つ。チョンピングの指示と字下げの数字も受け付ける）と、深く字下げした行に続くプレーンスカラー（空白でつないで折りたたむ）。共通の字下げは取り除き、ブロックは字下げも空行でもない最初の行で終わる。行をまたぐ引用符つきの値、アンカー、フローの集まりは読まない。完全な YAML のパーサは、一つの欄のために org が採らない依存である。
3. **一覧の行は 1 行のままにする。** 改行を保つ説明（`|`、または `>` の中の空行）は、載せる前に空白でつなぐ。事実のメッセージのどの行も、スキルの作者の言葉にはならない（独立検証、ADR-0039 §6）。

**ADR-0034 の下で。** 一覧は事実のメッセージの一部で、ラウンドの予算はそれを固定の費用として数える。skills-series の 9 個では約 1,700 文字増え、行数は `MaxSkills`（100）がこれまでどおり抑える。小さな窓では本物の費用だが、トリガーが隠れたスキルのほうが高くつく。読み込むべきときに読み込まれない。

## 結果

- プラットフォームの上限まで書いたスキルが、lagent でも宣言したすべてのことで起動する。
- 長い説明の分だけ事実のメッセージが大きくなる。増え方には上の上限がある。
- 1,024 文字を超える説明は、黙ってではなく注記つきで切られる。

## 検討した代替案

gem-agent ADR-0106 と同じ: 400 のまま注記を足す（org が配る 9 個のうち 6 個で注記が出て、トリガーは見えないまま）。上限なし（すべてのセッションに上限の無い行が入る）。YAML のライブラリ（一つの欄のための外部の依存）。

## 参照

- gem-agent ADR-0106、Issue #3、`ac5d03e`（移植元）
- `internal/skills/skills.go`（`DefaultLimits`・`readSkill`・`parseFrontmatter`）
