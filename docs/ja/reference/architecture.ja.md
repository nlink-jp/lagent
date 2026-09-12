# アーキテクチャ

lagent の現在の挙動を、前提知識なしで読めるように書く。ある判断がなぜ
そうなったかは [ADR](../INDEX.ja.md#adr) にあり、この文書はコードが今
何をするかを述べる。gem-agent のアーキテクチャが移植元（ADR-0001）で、
このランタイムが異なる箇所は ADR-0002 が除外したものと RFP が Phase 2 に
送ったものである。

## 形

1 バイナリ、1 プロセス、1 会話。`main.go` が `cmd.Execute` に渡し、
そこで 5 つを組み立てて配線する。

```
cmd/            flags, config load, project resolution, wiring, REPL/TUI
  |-- internal/config      strict-decode TOML + env/flag precedence
  |-- internal/llm         Backend interface + the OpenAI-compatible client (stream observer)
  |-- internal/tools       the nine file/shell/image built-ins + Register
  |-- internal/agent       the turn loop, approval dispatch, the round ladder
  `-- internal/tui         Bubble Tea inline UI (or internal/repl, non-TTY)
```

tools パッケージはプロジェクトディレクトリだけを要する 9 つの組込ツール
（`list_files`、`list_tree`、`search_files`、`read_file`、`file_info`、
`view_image`、`write_file`、`edit_file`、`shell_exec`）を持つ。`cmd/` は
同じ `Register` で `ask_user` と `mcp_load` と全 MCP ツールを登録する。

補助パッケージ: `internal/sandbox`（レーンごとの Seatbelt プロファイル
生成、永続ファイルと資格情報の一覧）、`internal/approve`（plain REPL の
ゲート）、`internal/risk`（自動承認の規則層）、`internal/policy`（ツール
ごとの承認ポリシー）、`internal/mcp`（stdio JSON-RPC クライアント）、
`internal/mcpfilter`（`[mcp] exclude` の背後にある唯一の述語）、
`internal/banner`（操作者が何か打つ前に出る行と、どの行を出すかの規則）、
`internal/mention`（`@` 参照: ファイル、ディレクトリ、画像 — および `@`
無しでドロップされた画像パス、ADR-0005）、
`internal/instructions`（`AGENTS.md` の発見）、`internal/ignore`（ignore
対応の列挙: 組込ディレクトリ一覧 + gitignore マッチャ）、
`internal/session`（transcript: ロガー + 再開ローダ、usage レコード）、
`internal/statedir`（プロジェクトごとの状態配置）、`internal/workdir`
（状態ルート下のセッション作業ディレクトリ）、`internal/trustpin`
（エージェント向けファイルのコンテンツピンと永続ファイルのスナップショット）、
`internal/uitext`（ja/en の UI 文字列カタログ）、`internal/bounded`
（他の全パッケージが使う上限付き read/list/capture プリミティブ）、
`internal/archtest`（構造規則を固定する AST テスト: パス系パッケージは
`os.Root` を通して開く、読み取りは上限付き、規則層は 1 関数で参照される、
プロジェクト内容のローダは全て grant を取る）。

## バックエンド

`internal/llm` は全ての会話を 1 つのエンドポイント、`[llm].base_url` の
OpenAI 互換 `chat/completions` に、stdlib の `net/http` と手書きの SSE
リーダで送る。履歴は呼び出しごとに 1 回ワイヤ形式へ変換される: system
プロンプトは system メッセージ、ユーザ文（`@` 参照およびそのままドロップ
された画像は image part、ADR-0005。`view_image` の結果も同じ形で画像を運ぶ）、
tool call 付きの assistant ターン、id で呼び出しに対応付けた tool 結果 —
id はサーバが送ったもの、transcript に無ければ合成したもの。ストリーミング
はテキスト差分を到着順に呼び出し元へ流し、tool call はインデックス付き
差分から組み立て、最後の usage チャンクが 4 つの会計バケツを埋める。
一過性の失敗（429、5xx、切断）は、まだ何も消費していない間だけバックオフ
付きで再試行する。`finish_reason` が `length` なら、届いたテキストを持つ
部分結果として返し、捨てない。

`[llm].provider` が選ぶのは 1 つ、`ContextWindow` がどこに尋ねるかだけ。
LM Studio はネイティブの `/api/v0/models/<id>`、Ollama は `/api/show` で
答え、素の OpenAI 互換サーバにはそのエンドポイントが無いので
`[model].context_window` が必須になる。

## MCP サーバ

`.mcp.json` は Claude Code の形式で読み、全サーバを接続し、全ツールを
`mcp__<server>__<tool>` として登録する（`[mcp].exclude` で濾過）。モデルに
何を見せるかは別に決める（ADR-0004、`cmd/mcpload.go`）: `[mcp].advertise =
"deferred"` ではランタイム事実メッセージが目録 — サーバごとに 1 行、ツール名と
initialize で公開した `instructions` の最初の一文 — を運び、組込の `mcp_load`
がそのサーバのツールをセッションの残りの間広告する。モデルに見せていない
登録済みツールへの呼び出しは、どのゲートにも達する前に経路を添えて拒否
される。`[mcp].preload` と `--allow mcp__<server>__*` の許可は最初から
広告する。`"all"` は全てを広告するベースライン。再開したセッションは
`mcp_load` の呼び出しを再生し、`/clear` は未ロードで新しい目録から始まり、
`/mcp` は各サーバのロード状態を示す。

## 1 ターン

`Agent.Run` は操作者の文を受け取り、`@` 参照を添付として展開し、ループ
する: 履歴を送る（tool 結果は送信時に nonce で包む）、応答をストリームする、
tool call ごとに判定・ゲート・実行・結果追記。ループはテキスト応答、
ラウンド上限、ループガードの停止で終わる。テキストもツール呼び出しも運ばない
completion は、報告する前に同じ履歴に一時的な 1 行を足して多くて 2 回再送する
（ADR-0007）。
全リクエストがセッション単位の
隔離タグの後ろに履歴全体を再送するので、リクエスト接頭辞はラウンドをまたいで
バイト同一に保たれ、サーバの接頭辞キャッシュが効く — ローカルモデルでは
そのキャッシュが 2 秒のターンと 2 分のターンの違いになる。

system プロンプトもセッションをまたいでバイト同一である（ADR-0003）:
隔離タグの名前、セッション作業ディレクトリ、開始日は、ランタイム自身の
user ロールのメッセージとして会話を開く（`Agent.AnnounceSession`、
`session.FactsPrefix`）。サーバは全ツールスキーマを system 文の後に描画し、
そこの 1 バイトが変わると全部を再処理するからだ。一覧はそのメッセージを
プレビューにも会話ありにも数えない。

## エージェント本体は UI を知らない

`agent.Options` がループと実行側の契約の全てで、どの面（TUI、plain REPL、
単発）も同じコールバックを配線し、agent は UI パッケージを決して import
しない。

- `OnToolCall` / `OnToolDone` — 呼び出しがゲートと実行に向かう直前、
  呼び出しが結果を出した直後（TUI の活動行と失速検知は後者で再武装し、
  ストリームチャンクでは決してしない）。
- `OnUsage` — 1 ラウンドのトークン消費。フッタのゲージ用。
- `OnAutoDecision` — 自動モードの各判定。UI が「尋ねずに走ったもの、
  その理由」を見せられる。
- `BeforeOperatorWrite` / `OnOperatorWrite` — 後続セッションが信頼する
  ファイルへの操作者承認済み書き込みの直前と直後（ピンが前後のファイルを
  比較する）。
- `OnAttach` — `@` 参照が取り込んだもの、取り込めなかったもの。
- `OnNotice` — ターン中の通知（切り詰められた応答、遅れて戻った放棄呼び出し）
  を操作者に見せる。
- `OnRoundLimit` — チェックポイントのダイアログ。nil は無人を意味し、
  チェックポイントは停止する。
- `ClipboardImage` — `@clipboard` の取り込み。nil は利用不可を報告する。
- `Advertise` — 登録済みツールのうちモデルに宣言するもの（ADR-0004）。隠した
  ツールへの呼び出しはどのゲートにも達する前に拒否される。

## 承認

規則層（`internal/risk`）が全呼び出しを分類する: Safe は `--auto` 下で
尋ねずに走り、Block は常に尋ね、Review は操作者に尋ねる。モデル層はここに
無い: 提案された呼び出しを裁く gem-agent の 2 つ目のモデル呼び出しは
計測のうえ採らなかった（ADR-0010）。セッション天井（`--read-only`）は呼び出しが
届けるレーンを上限で抑え、解除は操作者の行為。操作者専用ファイル —
指示ファイル、`.mcp.json`、`.lagent.toml`、同居ランタイムの
`.gem-agent.toml` — は常置の承認では決して答えられない。

## ラウンド階梯

同一呼び出し 3 連続は即座にエスカレートし、ラウンド上限はチェックポイント。
対話中はどちらも、そのターンの直近の呼び出しを証拠として操作者に尋ねる。
無人（`-p`）ではどちらもターンを止める、fail-closed — 操作者の代わりに
進捗を保証するモデルレビューが無いからだ。`[agent].max_turns` の 3 倍の
絶対上限は、何をもっても解除できない支出の境界。

## 永続化

transcript は状態ルート下のセッションごとの JSONL ファイルで、
`--continue` と `--resume` がそれを再生する。usage レコードはモデル呼び出し
ごとに、gem-usage-lens が両ランタイムに対して読む形（`prompt` / `output` /
`thoughts` / `cached` / `tool_prompt` / `total`、`tool_prompt` はここでは
常に 0）で書かれる。セッション作業ディレクトリは同じ状態ルート下の
プロジェクト別ディレクトリにあり（ルートは `LAGENT_STATE_DIR` で上書き）、
子プロセスへ `LAGENT_WORK_DIR` として、`LAGENT_SESSION_ID` と
`LAGENT_PROJECT_DIR` と並んで export される。

## 設定と drop-in の挙動

`~/.config/lagent/config.toml` は strict decode で読まれる。優先順位は
フラグ > `LAGENT_*` > ファイル > 既定。プロジェクトの `AGENTS.md` /
`CLAUDE.md` / `AGENT.md` / `GEMINI.md` は祖先ディレクトリまで遡り、
`~/.config/lagent` からもそのまま読まれる（プロジェクト自身のものは
信頼されピンが一致した後）。`.mcp.json` は
Claude Code の形式で読む。`.lagent.toml` はプロジェクトの承認ポリシーと
MCP の除外だけを持ち、他は何も持たない。

## ここに無いもの

`web_search`、`web_fetch`、メディアアップロード、Cloud Logging、thought
signature、safety 設定、要約モデル、委任ファイル探索は Vertex AI に
縛られた gem-agent の機能（ADR-0002）。履歴圧縮、自動承認のモデル層、
skills、agent memory、操作者 hooks は Phase 2（RFP §4）: trust プローブは
変更検知のために `.claude/skills` を今もピン留めするが、このランタイムは
スキルを読み込まない。
