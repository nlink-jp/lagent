# タスクベンチ

固定タスク集でランタイムを計測する方法（ADR-0006）。恒常文書: タスクや列が
変わるたびにその場で更新する。

## 実行

```bash
make build bench-build                       # dist/lagent, dist/bench-mcp
go run ./bench run --bin dist/lagent --configs baseline --reps 3
go run ./bench report bench/results/<timestamp>
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
| `--out` | 結果ディレクトリ（既定 `bench/results/<timestamp>`、git は無視） |
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

タスクは `bench/tasks/<name>/task.toml`（プロンプト、期待、フィクスチャ
サーバが要るなら `mcp = true`）と `testdata/`。期待は `min_tool_calls`、
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

## 比較

プロンプト改訂、thinking トグル、ツール説明の変更は、同じタスクに対する
構成ファイル 1 つ、または `--bin` 1 つの追加。参照ランタイムは
`--runtime gem-agent --bin $(which gem-agent) --configs mine=/path/to/your/gem-agent.toml`
で同じタスクを走らせる。その資格情報はあなたのファイルに留まり、この
リポジトリには決して入らない。
