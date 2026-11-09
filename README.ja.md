# lagent

**ローカル LLM** で動くサンドボックス付き CLI エージェントランタイム。
OpenAI 互換 API（LM Studio、mlx-serve、Ollama）を通してモデルに接続し、ファイルの
読み書き、サンドボックス内のシェルコマンド、MCP サーバを扱う。変更を伴う
呼び出しは操作者が承認する。

lagent は、クラウド API に出すべきでない作業のためのもの: オフライン環境、
機密プロジェクト、トークン単位の費用を掛けたくない作業。
[gem-agent](https://github.com/nlink-jp/gem-agent) とは別のプロダクトライン
で、設計は同じもの: 監査可能な最小ループ（read / edit / shell / MCP / 承認）、
対象プロジェクトの AGENTS.md / CLAUDE.md / .mcp.json と Claude Code 形式の
skills をそのまま読む drop-in 互換、同じセッション記録。

> **リリース済み。** `brew install nlink-jp/tap/lagent` または
> [リリースページ](https://github.com/nlink-jp/lagent/releases) から入手できる
> （Developer ID 署名・Apple notarize 済み、macOS arm64）— 現在の版数は
> リリースページが正。cli-series のツールであり、インタフェースの安定は約束で、
> 破壊的変更は組織の破壊的変更プロセスを通す
> （[ADR-0023](docs/ja/adr/0023-promotion-to-cli-series.ja.md)）。仕様は
> [RFP](docs/ja/lagent-rfp.ja.md) を参照。

English: [README.md](README.md)

## 動作要件

- Apple silicon の macOS（隔離は `sandbox-exec` に基づく）
- OpenAI 互換 API を持つローカル LLM サーバ:
  [LM Studio](https://lmstudio.ai/)（検証済みバックエンド、モデル
  `google/gemma-4-26b-a4b-qat`）、
  [mlx-serve](https://github.com/ddalcu/mlx-serve)（`provider = "mlxserve"`。
  Qwen 3.6 35B-A3B で計測済み — 既定で全インタフェースを待ち受けるので、
  `127.0.0.1` に絞るか API キーを設定する。長いセッションを毎ターン読み直さない
  よう、前置きキャッシュのメモリ上限も上げる（計測したのは 8 GB）:
  [ADR-0031](docs/ja/adr/0031-mlx-serve-is-a-supported-backend.ja.md)）、
  または Ollama
- 資格情報は不要

## 設定

`~/.config/lagent/config.toml` — [config.example.toml](config.example.toml)
を参照。優先順位はフラグ > `LAGENT_*` 環境変数 > ファイル > 既定。未知の
キーはエラー。

```toml
[llm]
provider = "lmstudio"        # lmstudio | mlxserve | ollama | openai
base_url = "http://localhost:1234/v1"
model    = "google/gemma-4-26b-a4b-qat"

[model]
context_window = 0           # 0 = provider から自動検出
```

## クイックスタート

LM Studio を起動してモデルをロードし、プロジェクトディレクトリで lagent を
実行する。

```bash
cd ~/work/my-project
lagent
```

初回起動でプロジェクト自身の AGENTS.md / CLAUDE.md / .mcp.json を信頼する
かを尋ねる。変更を伴うツールは実行前に尋ねる。`--auto` で規則層が Safe と
判定した呼び出しは尋ねずに走る。`[approval].model_tier = "shell"` なら、write
レーンのシェルコマンドも判定役のモデルが決める（実際の 1 か月分の確認ではおよそ半分が
尋ねずに走り、持ち出しや注入のケースは 1 つも通らなかった —
[ADR-0032](docs/ja/adr/0032-the-model-tier-judges-write-lane-shell.ja.md)）。
`-p "…"` はプロンプト 1 つを実行して終了し、そこへのパイプはデータとして添付します。
`-p` なしのパイプは打鍵と同じあなた自身の入力で、中の `!` コマンドも実行されるので、第三者のテキストは `-p` に渡してください。
`/help` でスラッシュコマンド一覧。

## できること

- **ツール:** `list_files`、`list_tree`、`search_files`、`read_file`、
  `file_info`、`view_image`、`show_image`、`write_file`、`edit_file`、`shell_exec`、
  `ask_user`、および
  `.mcp.json` の MCP サーバが提供する全ツール — モデルには目録として見せ、
  サーバごとに `mcp_load` を呼んだ時点で広告する（ローカルモデルは毎ターン
  243 スキーマを読む余裕がない。`[mcp].preload` と `[mcp].advertise = "all"`
  が操作者の調整点）。
- **部分的な結果にたどり着ける:** `read_file` は行の窓でも、バイト単位の
  `offset`/`length`（負の offset は末尾から）でも読めるので、長い 1 行の
  ファイル — 作業ディレクトリに保存されたツール結果 — の末尾にも届く。
  途中で切れた読み取りは、続きの読み方を示す（ADR-0029）。インラインに
  収まらない MCP の結果はセッション作業ディレクトリに丸ごと保存し、先頭 600 文字と
  末尾 200 文字でプレビューする — `"truncated": true` のようにサーバが後ろに付ける
  メタ情報は末尾に来る — 見せたバイト範囲と、残りへの読み方を添えて。
  20,000 バイトを超える `shell_exec` の出力は先頭 15,000 バイトと末尾 5,000
  バイトを残し — スクリプトの集計は末尾に来る — 作業ディレクトリに丸ごと保存
  する（32 MiB まで、本人だけが読める。認証情報を読める operator レーンと、
  サンドボックス無しのシェルは保存しない）。
  `search_files` は検索しなかったファイル — 2 MB 超（名前つき）、バイナリ、画像、
  読めないもの — をすべて数えるので、「該当なし」は見なかったものを言う。
- **エビデンスは打ち直さずに写す:** MCP の結果は、インラインで渡す小さいものも含めて
  すべて、作業ディレクトリに sha256 の名前で丸ごと保存し、トランスクリプトのツール結果に
  そのファイル（パス、サイズ、ハッシュ）を、サーバが書けない形で記録する。モデルはそれを
  告げる lagent 自身の行を読み、`write_file` に `content` の代わりに `from` を与えると、
  保存した結果をプロジェクトへバイト単位で写せる。結果には sha256 が付き、承認は内容を
  書くときとまったく同じ。エビデンスのファイルがサーバの返したものかどうかは、ハッシュの
  比較で決まる（ADR-0039）。
- **封じ込め:** ファイルツールはプロジェクト（とセッション作業ディレクトリ）
  の内側に留まり（`~/Dropbox` が `~/Library/CloudStorage` へのリンクなら、
  `/Users/<名前>/Dropbox/proj/…` のようにリンクを通した完全なパスも実パスと同じに
  届く。`~` そのものは展開しない）、資格情報ファイル — `.env`、秘密鍵、トークン保存先 — は
  あなたが毎回承認した時だけ読まれ、無人では決して読まれない。これらの読取は
  資格情報を開けないサンドボックス下の子で走るので、問いを上げるのは一覧、
  拒むのはカーネルである。`shell_exec`
  は宣言したレーンで `sandbox-exec` 下で走る —
  read は尋ねずに（inspection と、キャッシュをセッション scratch に持つ Go の
  ビルド・テスト）、write と operator は尋ねてから。
  read レーンは `~/Library`・`/Volumes`・`/Network` に立ち入らないが、そこにある
  プロジェクト — Google Drive・Dropbox・OneDrive・iCloud Drive、
  外部ディスク、ネットワーク共有 — は読める（ADR-0036）。
- **端末の安全:** モデルが書く文字列をはじめ、TUI が表示するランタイムの外から来た
  文字列は制御文字を取り除いてから端末に届くので、プロンプトインジェクションを受けた
  返答がウィンドウのタイトルを変えたり、クリップボードに書き込んだり、カーソルを
  動かしたり、承認ダイアログのコマンドを偽装したりすることはできません。`-p` の出力は
  パイプやファイルに対してはバイト単位でそのままです（ADR-0024）。
- **フック:** `[[hooks.pre_tool_use]]` がモデルのツール呼び出しの前に
  あなたのガードスクリプトを Claude Code の契約で走らせる。Claude Code を
  守る同じスクリプトがこのランタイムも守り、拒否は承認モードによらず最終で、
  理由はモデルへ返る（ADR-0012）。`session_start`、`user_prompt_submit`、
  `session_end` フックは同じ機構を共有し、その出力は引用データとしてモデル
  に届き、prompt フックはプロンプトを拒める（ADR-0014）。
- **メモリ:** `/remember <name> <fact>` で短い事実をこのプロジェクトの
  以後のセッションに残す（`global` で全プロジェクト）。モデルは `save_memory`
  で提案でき、あなたに尋ねる。メモリはこのモデルが行動する場所である
  runtime facts で想起されるので、ファイルやコマンドを名指すメモリは
  ポインタとして働く（ADR-0013）。
- **セッション:** セッションごとの JSONL transcript。`--continue` と
  `--resume`。usage レコードは
  [gem-usage-lens](https://github.com/nlink-jp/gem-usage-lens) が両ランタイム
  に対して読む形。
- **圧縮:** 要求がコンテキスト窓の `[agent].compact_at_pct`（80 %）を超えそうになると、
  会話の古い部分を要約に畳み、直近のメッセージはそのまま残す。`/compact` で手動でも
  できる。大きさは送る直前の要求で判定するので、大きなツール結果が並んだ 1 ラウンドや
  MCP サーバの読み込みも間に合う。ローカルサーバでは要約とその次の要求が入力を
  読み直すので待ち時間が出る — そのことは表示される（ADR-0033）。1 ラウンドのツール結果は
  窓に合わせた予算を分け合う: 小さい窓では大きな読み取りが短く返り、それぞれ続きの読み方を
  示すので、1 ラウンドで窓が溢れることはない。大きすぎるラウンドで詰まったセッションは、
  もう一度書き込めば立ち直る（ADR-0034）。
- **スキル:** Claude Code の `SKILL.md` 形式をそのまま、
  `~/.config/lagent/skills/<name>/` と信頼済みプロジェクトの
  `.claude/skills/<name>/` から読む。スキルごと 1 行が何のためのものかを
  モデルに伝え、モデルは `load_skill` で読み込み、操作者は `/skill <name>`
  で手で呼ぶ（ADR-0011）。説明は Claude Code の上限と同じ 1,024 文字まで載せ — それより
  長いものはスキル名を添えた注記つきで切る — 複数行の YAML の値（`>`・`|`）も丸ごと
  読む（ADR-0040）。スキルは指示として届き、ツールが返すものは —
  ナレッジ vault のサーバのノートも含め — モデルが決して従わないよう言われて
  いるデータとして届く。従わせたい手順書は、MCP で読むノートではなくスキルに
  置く（gem-agent の integration リファレンス「運用者の手順書の置き場所」）。
- **インライン画像:** モデルは `show_image` であなたに絵を見せ、あなたは
  `/show <path>` で見たい画像を出す（PNG か JPEG、2 MiB まで。パスは空白を含んでも、
  引用符付きでも、ウィンドウへドラッグしたファイルでもよい）。端末が描けるときに、入力欄より上の画面に
  収まる最大の大きさで、ただし画像そのものより大きくはせずに描く（iTerm2 と kitty。
  `[tui].images`、既定は `auto`、tmux・screen の中では off）。`view_image` は
  逆向きで、**モデルが**画像を見るためのものである（ADR-0020/0021/0022/0037）。
- **待っている対話は端末を鳴らす:** 承認や質問が 6 秒キー入力無しのままだと、TUI は端末のベルを
  1 回鳴らす（`[tui].notify`: 既定は `bell`、iTerm2 などでデスクトップ通知を出す `osc9`、または
  `off`）。キーを押すと次の 6 秒ごとの確認まで先送りされる。キーを押さずに読んでいるだけでは
  先送りされないので、長い対話を読んでいる途中で鳴ることがある。何も止めない。素の REPL と `-p` では
  知らせない（ADR-0038、gem-agent ADR-0104 から）。
- **図を絵として:** 返答中の mermaid フェンス（flowchart / graph・sequenceDiagram・erDiagram・pie・stateDiagram・gantt・mindmap、
  日本語のラベルも）は、端末が画像を描けるなら絵として描く。図の文字 1 em が端末の 1 行になる大きさ。
  正しく描けないものは注記 1 行つきでソースのまま。それ以外の端末では、flowchart・sequence・ER を同じ
  エンジンが罫線で描き、他の種類はソースのまま（ADR-0027）。素の REPL と `-p` は常にソース。フォントは
  `[tui.diagram]`。ウィンドウを狭めても、画面上の絵は残る（ADR-0026）。
- **無いもの:** web 検索と取得、メディアアップロード、監査ログ出力
  — RFP と
  [ADR-0002](docs/ja/adr/0002-features-not-reproduced.ja.md) を参照。

## プロジェクトの外で作業する

ファイルツールはプロジェクトの中にとどまり、サンドボックス内のシェルコマンドは
その外に書き込めません。ノート、Vault、隣のチェックアウトなど別のディレクトリで
作業するには、次の方法で追加します。

| 方法 | プロジェクトと同じく書く | 読むだけで書かない |
|---|---|---|
| 起動時 | `--add-dir <path>` | `--read-dir <path>` |
| 設定ファイル | `[directories].write` | `[directories].read` |
| セッション中 | `/add-dir <path>` | `/read-dir <path>` |

`/dirs` で有効なものを確認できます。モデルがディレクトリを要請することもあり、
答えるのはいつもあなたです。追加したディレクトリには同じサンドボックスのレーンが
適用され、中のもの（`AGENTS.md`、`.mcp.json`、skill）は何も読み込みません。
セッション中に追加したものは lagent の終了まで有効です。Google Drive、Dropbox、
OneDrive、iCloud Drive のフォルダも書き込み用に追加できます。

エージェントで開くプロジェクトの親にあたるディレクトリは、書き込み用に追加しないで
ください。コマンドが `AGENTS.md` や git フックを含むディレクトリをそこへ移動できます。
lagent はそれを検知して伝えますが、止めはしません。

## 添付

`@<path>` はプロジェクト内のファイルやディレクトリを、`@<image>` はどこに
あっても画像を添付する（絶対パス・`~` パス可）。端末にドロップした画像の
パスは `@` 無しでも添付され、エスケープ済みの空白も読む。読めない参照は
操作者に警告され、モデルにも伝えられる。

## インストール

Apple Silicon Mac に nlink-jp の Homebrew tap から（署名・notarize 済みの
リリースアーカイブをそのまま入れる）:

```bash
brew tap nlink-jp/tap
brew install nlink-jp/tap/lagent
```

## ビルド

```bash
make build      # → dist/lagent
make test
make check      # vet + lint + test + docs ミラー + リリースゲート + build
```

## ドキュメント

- [docs/ja/INDEX.ja.md](docs/ja/INDEX.ja.md) — 仕様（RFP）、リファレンス、ADR

## ライセンス

MIT
