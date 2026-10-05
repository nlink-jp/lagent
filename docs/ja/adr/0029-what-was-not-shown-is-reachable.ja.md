# ADR-0029: ランタイムが見せなかった部分は、たどり着けて数えられるようにする

| 項目 | 値 |
|-------|-----|
| Status | **Accepted**（2026-10-05）— gem-agent ADR-0096 Part A から移植。実装済み |
| Date | 2026-10-05 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | 運用者が採択した gem-agent ADR-0096: 運用者の分析品質のレポートは、部分的な結果が全件として扱われた原因を、ランタイムが保持したものの置き場所にたどった。退避の intake、`read_file`、`shell_exec` の出力の書き込み器、`search_files` の走査は両ランタイムが共有する仕組みなので、欠陥はこのランタイムのものでもある |
| Relates to | gem-agent ADR-0096（レポート、所見の検証、独立した設計レビュー、Part B を決めたプローブ）、gem-agent ADR-0058（退避）、gem-agent ADR-0052 §2（スキップは報告する） |

## Context

gem-agent はレポートのランタイムに関する所見を自分のコードと照合し、
このランタイムがそのまま移植した仕組みに 4 つの欠陥を見つけた:

- 退避した MCP のテキストブロックは**先頭だけ**（800 rune）を見せていた。
  サーバが後ろに付けるメタ情報 — `"truncated": true`、総行数 — がモデルの
  前に出ることはなかった。
- 退避の通知は `read_file <path>` と言うが、`read_file` は行で窓を切り、
  200 KB で止まる。200 KB を超える 1 行の退避結果の末尾には、通知が名指し
  するツールでは届かなかった。
- `shell_exec` は出力の**先頭** 20 KB を残し、何も保存しなかった。
  スクリプトが最後に出す集計は失われた。
- `search_files` は 2 MB 超・バイナリ・画像のファイル、読めないファイルと
  ディレクトリを、**数えずに**スキップしていた。

ここのコードは同じ（`cmd/mcpresult.go`、`internal/tools`、`internal/bounded`）
なので、欠陥も同じである。

## Decision

gem-agent ADR-0096 の Part A を、そのとおりに:

1. **退避のプレビューは先頭と末尾** — 先頭 600 rune と末尾 200 rune、
   どちらも rune の境界で切る — で、通知は見せたバイト範囲と、残りへの
   `read_file` offset/length の経路を示す。
2. **`read_file` はバイトでも読める**: `offset`（負は末尾から）と `length`
   （既定 `OutputCap`、上限 `readCap`）。行の窓と排他で、rune の境界に寄せ、
   返したバイトを注記で示す。`readCap` で切れた通常の読み取りは、続きを
   読む `offset=N` を示す。
3. **`shell_exec` は先頭と末尾を残し、全体を保存する**: `OutputCap` を
   超えると、モデルは最初の 4 分の 3 と最後の 4 分の 1 を受け取り、出力は
   パイプを持つランタイムがセッションの作業ディレクトリに保存する
   （32 MiB まで）— どのレーンの届く範囲も変わらない。operator レーンは
   保存しない: 認証情報を読めるレーンであり、作業ディレクトリの写しは承認
   なしで読めてしまう。サンドボックス無しで走るシェルも、どのレーンも認証情報を
   読めるので保存しない。保存ファイルは本人だけが読める（`0600`）。書き込み器は
   `bounded.HeadTail`。レーンの案内は、ランタイムの注記ではなくコマンド自身の
   出力を見て出す。
4. **`search_files` は検索しなかったファイルをすべて数える**: 理由ごとに —
   2 MB 超（5 件まで名前を出す）、バイナリ、画像、読めない、一覧できない
   ディレクトリ — そして名前を出す 5 件を超えた拒否も。

`internal/tools/bytewindow.go`、`shellout.go`、`bounded.HeadTail` と、
`mcpresult.go`・`tools.go`・`nav.go` への変更は、それを作った gem-agent の
コミットから、同じテストとともに移植する。

移植しないもの: Part B（部分的な表示を nonce タグの外で申告する）。
gem-agent は決める前に測った — 実バイナリをスタブの MCP サーバに対して、
3 つの腕で — そして先頭だけの腕も含め、すべての腕が正しく答えた。あちらで
採らなかったので、移植するものが無い。gem-agent の `tools/coverageprobe` も
移植しない: このランタイムにはプローブが無い。

## Consequences

- 退避した結果の末尾がモデルの前にあり、そのどのバイトにも、通知が名指し
  するツールで届く。
- `shell_exec` の出力は 20 KB を超えても失われなくなる。ただし operator
  レーンは除き、そう書く。長い出力は作業ディレクトリにファイルを残す。
- `search_files` の「該当なし」は、検索しなかったものを言う。
- 注記はツールのテキストの中の、今の位置にとどまる。

## References

- gem-agent ADR-0096 — 決定、検証、測定
- `cmd/mcpresult.go`、`internal/tools/bytewindow.go`、`internal/tools/shellout.go`、
  `internal/tools/nav.go`、`internal/bounded/bounded.go`
