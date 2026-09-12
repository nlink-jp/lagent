# タスクベンチ

固定タスク集でランタイムを計測する方法（ADR-0006）。恒常文書: タスクや列が
変わるたびにその場で更新する。

## 実行

```bash
make build bench-build                       # dist/lagent, dist/bench-mcp
go run ./bench run --bin dist/lagent --configs baseline --reps 3
go run ./bench report bench/_results/<timestamp>
```

設定が指す `base_url` でローカルモデルサーバが動いている必要がある。6 タスク・
3 反復・1 構成の実行は one-shot セッション 18 本で、秒ではなく分の単位で
見積もる（cold の接頭辞は基準機で約 2 分）。

| フラグ | 意味 |
|---|---|
| `--bin` | 計測するランタイムのバイナリ（必須） |
| `--configs` | カンマ区切りの構成: `baseline` は `bench/configs/<runtime>/baseline.toml` に解決。`name=/path/config.toml` で自分のファイル |
| `--runtime` | `lagent`（既定）または `gem-agent`: バイナリが読む状態ルート変数と設定ディレクトリ |
| `--tasks` | 走らせるタスク名（既定: `bench/tasks` 下の全ディレクトリ） |
| `--reps` | タスク×構成ごとの反復数（既定 3） |
| `--timeout` | 実行ごとの期限（既定 10m）。超えた実行は timed out として記録し、ベンチは次へ進む |
| `--out` | 結果ディレクトリ（既定 `bench/_results/<timestamp>`、git は無視） |
| `--mcp-bin` | `mcp-lookup` タスク用のフィクスチャ MCP サーバ（既定 `dist/bench-mcp`） |

順序: ケースを外側、構成を内側 — タスクと反復ごとに全構成を連続で走らせる
ので、サーバの状態（キャッシュ、負荷）の揺れは全構成に等しく当たる。実行が
終わるたびに 1 行印字する。

## 隔離

実行ごとに専用の `HOME`（構成を `config.toml` として、タスクが要すれば
`bench-mcp` を指す global `mcp.json`）、専用の状態ルート
（`LAGENT_STATE_DIR` / `GEMAGENT_STATE_DIR`）、タスクの `testdata/` の新しい
複製をプロジェクトとして与える。プロジェクトはその `HOME` の下に置くので、
結果ディレクトリより上の指示ファイルは読まれない。あなたの設定、セッション、
プロジェクトには決して触れない。実行は `-p` と `--auto` で行い、ベースライン
構成はフィクスチャサーバに `"never"` ポリシーを持つので MCP タスクに
`--allow` は要らない（許可はサーバをプリロードし、モデルが自分でロードするか
を隠してしまう）。

## タスク

| タスク | 種別 | 完了条件 |
|---|---|---|
| `search-answer` | 検索して答える | 回答が `store.go:SaveConfig` を含む。ツール呼び出し 1 回以上 |
| `read-edit` | 読んでから直す | `pager.go` に `i < n` があり `i <= n` が無い。ツール呼び出し 2 回以上 |
| `multi-file-rename` | ファイルをまたぐ変更 | 3 ファイルが `MaxEntries` を持ち、どれも `MaxItems` を持たない。ツール呼び出し 3 回以上 |
| `shell-count` | 数を出すシェルコマンド | 回答が `57` を含む。ツール呼び出し 1 回以上 |
| `mcp-lookup` | MCP lookup | 回答が `Iceland` を含む。ツール呼び出し 2 回以上（`mcp_load`、次に lookup） |
| `view-image` | 見るべき画像 | 回答が `red` を含む。ツール呼び出し 1 回以上 |
| `pointer-small`、`pointer-mid`、`pointer-large` | プロジェクトの `AGENTS.md`（0.6 KB、13 KB、32 KB — 最後はファイル上限の直下）にある `PROCEDURE.md` を指す常置の指示 | `read-edit` と同じく `pager.go` を直し、`CHANGES.log` が `pager.go` を含む（手順の唯一の動作）。ツール呼び出し 2 回以上 |
| `pointer-facts` | 同じ指示を runtime-facts メッセージのスキル一覧 1 行にしたもの。指示を含まない 32 KB の `AGENTS.md` を併置 | 同上 |
| `memory-follow` | `pointer-*` の指示を実行の状態ルートに global memory としてインストール — facts メッセージ内のツール名を含まない素のポインタ行。指示を含まない 32 KB の `AGENTS.md` を併置 | `pointer-*` と同じ |
| `skill-follow` | 読み込んで従うべきスキル | 回答がスキルの固定形式行そのもの（`BRIEF: <n> rows, <n> columns, first id <n>, last id <n>`）。ツール呼び出し 2 回以上（`load_skill`、次に数える） |

タスクは `bench/tasks/<name>/task.toml`（プロンプト、期待、フィクスチャ
サーバが要るなら `mcp = true`）と `testdata/`、スキルのインストールが
要るタスクなら `skills/` ディレクトリ — ランナーが実行の隔離 global スキル
ディレクトリへ複製する。`memory/` ディレクトリ（`global/<name>.md`）は
ランナーが実行の状態ルートの memory としてインストールする。`trust = true` は実行のプロジェクトを信頼済みに
する（構成の `[approval].trusted_projects` がそれを名指し、実行前にピンを
記録する）ので、フィクスチャ自身の `AGENTS.md` が読み込まれる。既定は off
で、未信頼のプロジェクトがベースラインである。期待は `min_tool_calls`、
`[[expect.file]]`（`path` と `contains` / `not_contains`）、
`[[expect.answer]]`（`regex`）。ファイル無しで答えられるタスクはここに
置かない。

## 計測するもの

実行ごとに、ランタイム自身のスキャナでセッション transcript から読む:
ラウンド数（ツール呼び出しを運ぶ assistant メッセージ）、ツール名ごとの
呼び出し数、メインループの usage レコードを合計した prompt / output / cached
トークン、壁時計時間、終了コード、最終回答、失敗した期待。`runs.jsonl` に
実行ごと 1 行。各実行のディレクトリに `stdout.txt`、`stderr.txt`、transcript、
home、project が残る。

`bench report` はタスク×構成ごとに Markdown 1 行を出す: 実行数、完了数、
no-tool answers（ツールを呼ばずに答えた実行）、ラウンド数・ツール呼び出し数・
prompt トークン・壁時計秒の中央値。

## 計測結果

ベースライン、2026-09-12: lagent v0.1.0（`acd6c4c`）、LM Studio、
`google/gemma-4-26b-a4b-qat`、3 反復、構成 1 つ。

| task | config | runs | completed | no-tool answers | rounds (med) | calls (med) | prompt tok (med) | wall s (med) |
|---|---|---|---|---|---|---|---|---|
| mcp-lookup | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 12578 | 12 |
| multi-file-rename | baseline | 3 | 3/3 | 0/3 | 11 | 11 | 57408 | 30 |
| read-edit | baseline | 3 | 1/3 | 0/3 | 3 | 3 | 16271 | 10 |
| search-answer | baseline | 3 | 3/3 | 0/3 | 3 | 3 | 16239 | 10 |
| shell-count | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 11992 | 10 |
| view-image | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 12039 | 10 |

読み取れること:

- ツールを呼ばずに答えた実行は無い。ベースラインのプロンプトで全種別が
  多段作業に入った。MCP lookup（モデルが自発的に `mcp_load` を呼んだ 3/3）
  と画像（3/3 で `view_image`）を含む。Phase 1 で観察した単発応答はこれらの
  タスクでは再現しない。それを示した操作者のセッションはスクリーンショット
  と read レーンの事例で、どちらも対処済み。境界があるとすれば、これより
  大きなタスクにある。
- 唯一の失敗種別はモデルの計画ではなく出力の挙動: `read-edit` の 2/3 で、
  `pager.go` を読んだ直後にモデルが空の completion（finish reason `stop`、
  出力 3 トークン、本文もツール呼び出しも無し）を返し、one-shot 実行は
  そこでエラー終了した。3 回目は 9 ラウンドでファイルを直した。
- 複数ファイルの改名は 11 ラウンドで prompt 約 57k トークン。毎ラウンド
  履歴を再送するので、30 秒に収まっているのは接頭辞キャッシュのおかげ。

`read-edit`、生ストリームをトレース（`LAGENT_LLM_TRACE`）した 8 反復:
5/8 完了。2 回は空 completion で終わり、トレースは両方とも同じバイト列 —
`reasoning_content` が `<tool_call|>` の delta 1 つ、空の delta、
`finish_reason: stop`、completion 4 トークン: 壊れたツール呼び出し開始
トークンをサーバが reasoning チャネルへ流したもの（ADR-0007）。1 回は修正を
散文とコードブロックで説明して `edit_file` を一度も呼ばなかった — 行動せず
語るモデルで、プロンプト改訂が計測対象にする形。

ADR-0007（`4df8c4f`）適用後の `read-edit`、8 反復: 8/8 完了、中央値 6 ラウンド、
19 秒。この 8 回では空 completion が起きなかったので、再送そのものは実機で
発火していない。挙動は単体テストで固定してあり、障害の実機発生率は前 2/8、
後 0/8 という小さな標本の数字である。

参照ランタイム、2026-09-12: gem-agent v0.76.0（Vertex AI Gemini）、1 反復、
操作者の設定から hooks と telemetry を外し、`read_only_auto` を off、
フィクスチャサーバに `"never"` ポリシー。

| task | config | runs | completed | no-tool answers | rounds (med) | calls (med) | prompt tok (med) | wall s (med) |
|---|---|---|---|---|---|---|---|---|
| mcp-lookup | bench | 1 | 1/1 | 0/1 | 2 | 2 | 17570 | 23 |
| multi-file-rename | bench | 1 | 1/1 | 0/1 | 23 | 23 | 193341 | 141 |
| read-edit | bench | 1 | 1/1 | 0/1 | 15 | 15 | 119359 | 128 |
| search-answer | bench | 1 | 1/1 | 0/1 | 2 | 2 | 17427 | 23 |
| shell-count | bench | 1 | 1/1 | 0/1 | 4 | 4 | 29993 | 55 |
| view-image | bench | 1 | 1/1 | 0/1 | 1 | 1 | 12369 | 11 |

ADR-0008（規則ではなく経路）適用後、ADR-0007 を再送 2 回に訂正した状態、
2026-09-12（`a69160a`）、3 反復:

| task | config | runs | completed | no-tool answers | rounds (med) | calls (med) | prompt tok (med) | wall s (med) |
|---|---|---|---|---|---|---|---|---|
| mcp-lookup | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 12623 | 12 |
| multi-file-rename | baseline | 3 | 3/3 | 0/3 | 15 | 15 | 88664 | 42 |
| read-edit | baseline | 3 | 3/3 | 0/3 | 6 | 6 | 35627 | 23 |
| search-answer | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 12131 | 9 |
| shell-count | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 11980 | 10 |
| view-image | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 11985 | 10 |

ベースライン比: 18/18 完了（前は 16/18）、全実行 exit 0、拒否後に散文で
終わった実行は無い（拒否そのものが起きなかった: 検証の `go run` は全
read-edit 実行で read レーンで成功）、`list_tree` 後の `list_files` ラウンドは
18 実行中 15 で消えた（multi-file-rename の 3 回はなお列挙した）。空 completion
は 4 回起き、すべて再送で回復した。編集系 2 タスクのラウンド数は増えた
（read-edit 3 → 6、改名 11 → 15）。プロンプトが求める検証が実際に走るように
なったためで、以前の中央値は失敗で早く終わった実行を数えていた。

`read-edit`、同じバイナリで 8 反復: 8/8 完了、中央値 6 ラウンド、22 秒。
8 実行で空 completion が 9 回再送を発火し、8 回は回復、1 実行は最終回答の
地点で 3 連続（ファイルは既に直っていたのでタスクは完了したが、実行は
回答文無しの exit 1）。障害は検証成功後の最終回答の地点に集中し、そこでは
コイン投げより頻繁である。何も変えない再送はその地点では答えの全部ではない
かもしれず、それが次の計測項目。

thinking off 対 on（ADR-0009）、2026-09-12（`bea19b7`）、3 反復、交互実行。
`reasoning-on` はベースラインに `reasoning_effort = "low"` を足したもので、
LM Studio が Gemma 4 の thinking on に丸める。ベースラインは何も送らず、
それが off。

| task | config | runs | completed | no-tool answers | rounds (med) | calls (med) | prompt tok (med) | wall s (med) |
|---|---|---|---|---|---|---|---|---|
| mcp-lookup | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 12619 | 12 |
| mcp-lookup | reasoning-on | 3 | 3/3 | 0/3 | 2 | 2 | 12630 | 14 |
| multi-file-rename | baseline | 3 | 2/3 | 0/3 | 10 | 10 | 51828 | 30 |
| multi-file-rename | reasoning-on | 3 | 3/3 | 0/3 | 11 | 11 | 58866 | 70 |
| read-edit | baseline | 3 | 3/3 | 0/3 | 3 | 3 | 20813 | 14 |
| read-edit | reasoning-on | 3 | 3/3 | 0/3 | 6 | 6 | 30396 | 40 |
| search-answer | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 12061 | 10 |
| search-answer | reasoning-on | 3 | 3/3 | 0/3 | 2 | 2 | 12151 | 13 |
| shell-count | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 12037 | 10 |
| shell-count | reasoning-on | 3 | 3/3 | 0/3 | 3 | 3 | 16320 | 17 |
| view-image | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 12018 | 10 |
| view-image | reasoning-on | 3 | 3/3 | 0/3 | 2 | 2 | 12055 | 12 |

各 18 実行の合計: ベースラインは 17/18 完了、壁時計 252 秒、prompt 357k
トークン、reasoning 53 トークン、空 completion 3 回（すべて行付き再送で
回復）。thinking on は 18/18、526 秒、prompt 463k トークン、reasoning
12,398 トークン、空 completion ゼロ。ベースラインの 1 失敗は空の不具合では
なくループ: 回復した空の後、モデルは `limits.go` を 2 回編集してから 3 回
読み、ループガードがターンを止めた。thinking on は空の不具合を消し 18 中
1 タスクを得たが、代価は壁時計 2 倍と prompt トークン 3 割増。編集系が
最も払い（read-edit 14 秒 → 40 秒、改名 30 秒 → 70 秒）、lookup 系はほぼ
払わない。したがって既定は未設定のまま: 再送の行が不具合をごく小さな代価で
覆い、難しいタスクでは操作者が `reasoning_effort` を設定する。

再送に一時的な行を付けた `read-edit`（ADR-0007 第 2 改訂）、8 反復: 8/8
完了、全実行 exit 0、中央値 6 ラウンド、22 秒。空 completion は 2 回起き、
どちらも促し付きの最初の再送で回復した。行を付ける前の同じ系統のバイナリ
では、8/8 完了だが 1 実行が 3 連続の空で exit 1 だった。

全タスクが両ランタイムで完了した。参照側はタスクごとの検証が多く（改名に
23 ラウンド、編集に 15 ラウンド、前後でプログラムを実行）、その分を prompt
トークンと壁時計時間で払う。ローカルモデルは 3 分の 1 のラウンドで同じ完了に
達する。実行が要するラウンド数は各モデルの癖の性質であってランタイムの
性質ではなく、品質の点数でもない。

`skill-follow`、2026-09-12（`7806818`、skills 導入、ADR-0011）、3 反復:
モデルは facts メッセージの一覧 1 行から促されずに 3/3 で最初に
`load_skill` を呼び、3/3 でスキルの固定形式で答えた — 2 ラウンド、約 14
秒、prompt 13〜18k トークン。最初に書いたタスクは数値の一致も要求し、
それを満たしたのは 1/3: 1 回は最後から 2 行目を最終行と取り（`tail -n 2 |
head -n 1`）、1 回はヘッダを数えた（`wc -l` を引かず）。これはシェルの
算術で `shell-count` が計測するものなので、期待は数値を自由にした形式行に
改めた。その基準でさらに 3 反復:

| task | config | runs | completed | no-tool answers | rounds (med) | calls (med) | prompt tok (med) | wall s (med) |
|---|---|---|---|---|---|---|---|---|
| skill-follow | baseline | 3 | 3/3 | 0/3 | 2 | 2 | 18144 | 13 |

再び 3/3 で `load_skill` が最初、形式は 3/3、数値の一致はやはり 1/3。
このタスクが示すのは機構: facts の 1 行で名指されたスキルを、このモデルは
プロンプト規則無しに読み込んで従う。その後シェルで何をするかはモデル自身の
問題である。

`pointer-*`、2026-09-12（`384cd2c`、skills と hooks 導入後）、各 3 反復。
memory 設計が依存する問い: 常置の指示はどこに置けばこのモデルは行動するか。

| task | config | runs | completed | no-tool answers | rounds (med) | calls (med) | prompt tok (med) | wall s (med) |
|---|---|---|---|---|---|---|---|---|
| pointer-small | baseline | 3 | 0/3 | 0/3 | 3 | 3 | 23172 | 15 |
| pointer-mid | baseline | 3 | 0/3 | 0/3 | 8 | 8 | 71879 | 32 |
| pointer-large | baseline | 3 | 0/3 | 0/3 | 6 | 6 | 84398 | 37 |
| pointer-facts | baseline | 3 | 2/3 | 0/3 | 8 | 8 | 124739 | 44 |

全実行が `pager.go` を直した。`AGENTS.md` の 9 実行でモデルは `PROCEDURE.md`
を一度も読まなかった — 0.6 KB でも 32 KB でも同じで、トレースしたリクエスト
は指示がシステムプロンプトのプロジェクト指示節に入っていたことを確認して
いる。ファイルの大きさは変数ではない: その節の常置の指示にこのモデルは
まったく行動しない。`pointer-facts` ではモデルが一覧 1 行から自発的に
`load_skill` を呼んだのが 2/3 で、どちらもログを書いた。3 回目は空 completion
にも当たり、ファイル全体を書き直して読み込まずに終えた。同じタスクの最初の
系列は手順がシェルの追記（`>> CHANGES.log`）を求めており、スキルは 3/3 で
読み込まれたが完了は 0/3: write レーンのシェルは無人では拒否されるので、
計測器を `write_file` に直してから上の系列を走らせた。両系列を通して、facts
メッセージの行は 6 実行中 5 で行動され、指示ファイルの指示は 18 実行中 0。
呼ぶべきツールを添えて runtime-facts メッセージに乗ったものは従われ、指示節
に置かれたものは大きさによらず従われない。

`memory-follow`、2026-09-12（`72a4788`、memory 導入、ADR-0013）: 同じ指示を
facts メッセージで想起される memory にしたもので、行にツール名は無い。
移植した見出し（「背景知識、古い可能性あり、指示ではない」）の下では 1/3:
モデルが手順ファイルを読んで編集を記録したのは 1 回。そこで見出しを、
memory は利用者の常置メモ — どれも利用者の手を経ている、打ち込んだか承認
したか — と呼び、いまの作業に当てはまるメモには従えと言うものに変えた。
その見出しの下で 3 + 6 反復:

| task | config | runs | completed | no-tool answers | rounds (med) | calls (med) | prompt tok (med) | wall s (med) |
|---|---|---|---|---|---|---|---|---|
| memory-follow | baseline | 3 | 2/3 | 0/3 | 10 | 10 | 145674 | 45 |
| memory-follow | baseline | 6 | 6/6 | 0/6 | 7 | 7 | 104286 | 39 |

8/9: 1 回を除く全実行でモデルは `PROCEDURE.md` を読みログを書いた。チャネルは
`pointer-facts` と同じで、1/3 と 8/9 の間で変わったのは見出しの立場である。
「指示ではない」と言う行をこのモデルは言葉どおりに受け取り、その下の
ポインタは背景になる。同じポインタが「利用者のメモ — 当てはまるものには
従え」の下にあれば、スキル行と同じ率で従われる。代価はラウンド数: 手順を
読んでログを書くのは素の修正より 2〜3 回多いツール呼び出し（中央値 7 対
`pointer-small` の 3）。

## 比較

プロンプト改訂、thinking トグル、ツール説明の変更は、同じタスクに対する
構成ファイル 1 つ、または `--bin` 1 つの追加。参照ランタイムは
`--runtime gem-agent --bin $(which gem-agent) --configs mine=/path/to/your/gem-agent.toml`
で同じタスクを走らせる。その資格情報はあなたのファイルに留まり、この
リポジトリには決して入らない。`~/.config/gcloud` 下の Application Default
Credentials は隔離 `HOME` で見えなくなるので、プロファイルはファイルが
あれば `GOOGLE_APPLICATION_CREDENTIALS` としてそれを通す。
