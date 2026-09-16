# ドキュメント索引

lagent の保守者向けドキュメントの入口。利用者向けは
[`README.ja.md`](../../README.ja.md) を参照。

英語版: [`INDEX.md`](../en/INDEX.md)。`scripts/docs-mirror-check.sh` が
`make check` で構造面を強制する — `docs/en` の各ファイルに `docs/ja` の対が
あり、その逆も成り立ち、ADR 一覧は両言語で完全かつ昇順、各対の識別子が
一致する。文意の一致は書き手の責任。

## 仕様

- [`lagent-rfp.ja.md`](lagent-rfp.ja.md) — 正典となる仕様: 問題定義、
  機能面、スコープ境界、フェーズ計画。ここに無い機能は ADR を要する。
  ADR や実装が節を修正した箇所にはその場に注記があり、現行はリファレンス
  文書が示す。

## リファレンス

現在の挙動。コードの変更に合わせてその場で更新する。

- [`reference/configuration.ja.md`](reference/configuration.ja.md) —
  インストール、設定ファイル、優先順位、コマンド表とフラグ表
- [`reference/architecture.ja.md`](reference/architecture.ja.md) —
  パッケージ配置、バックエンド、ターンループ、承認、ラウンド階梯、
  永続化、ここに無いもの
- [`reference/bench.ja.md`](reference/bench.ja.md) — タスクベンチ:
  実行方法、隔離、タスク、計測するもの、構成と参照ランタイムの比較

機能リファレンス（interface、tools、approval、sessions、integration）は
Phase 1 の計測で表面が落ち着くのに合わせて書く。

## ADR

ある時点の設計判断。採択後は不変で、判断が変わったときは新しい ADR が
古いものを置き換える（誤字とリンクの修正は除く）。

- [`ADR-0001`](adr/0001-porting-sources-pinned.ja.md) — gem-agent と
  llm-cli はコミットで固定した移植元: フォークではなく新規リポジトリで、
  パッケージは移植元を記録して 1 つずつ持ち込み、その後は追随しない
- [`ADR-0002`](adr/0002-features-not-reproduced.ja.md) — lagent が
  再現しない gem-agent の機能、それぞれが Vertex AI / Google Cloud に
  縛られている理由、代わりに lagent が行うこと
- [`ADR-0003`](adr/0003-session-facts-ride-the-conversation.ja.md) —
  system プロンプトはセッションをまたいでバイト同一。隔離タグ、作業
  ディレクトリ、開始日はランタイムの冒頭メッセージに乗せ、サーバの接頭辞
  キャッシュが新セッションと `/clear` を生き延びる（実測: MCP 243 ツールで
  118 秒対 2 秒）
- [`ADR-0004`](adr/0004-mcp-tools-load-on-demand.ja.md) —
  MCP ツールは必要なときに広告する — 事実メッセージ内の目録、`mcp_load`
  ツール 1 つ、ロード後はネイティブ呼び出し。`[mcp].preload`、ベースライン
  としての `[mcp].advertise = "all"`
- [`ADR-0005`](adr/0005-images-reach-the-model.ja.md) — 画像はモデルに
  届く: ドロップされた画像パスは `@` 無しで添付（エスケープ済み空白を
  読む）、`view_image` 復帰、添付されなかった参照はモデルに伝える
- [`ADR-0006`](adr/0006-measurement-bench.ja.md) — Phase 2 がランタイムを
  変える前にタスクベンチで計測する: フィクスチャのタスク、構成ファイル、
  隔離実行、計測値としての transcript、同じタスクでの参照ランタイム
- [`ADR-0007`](adr/0007-empty-completion-asked-again.ja.md) — 空の
  completion は多くて 2 回再送する: 生ストリームは誤サンプルされたツール
  呼び出し開始トークンが reasoning チャネルへ流れたことを示し、起きる地点
  ではコイン投げで、同じリクエストの再送は正常に答える
- [`ADR-0008`](adr/0008-routes-not-rules.ja.md) — ランタイムは規則では
  なく経路を供給する: ツールチェインのキャッシュをセッション scratch に
  乗せてビルドを read レーンで走らせ、無人の拒否は経路を名指し、
  `list_tree` は見たものを言い、プロンプトはそれらが置き換えた規則を落とす
  （ローカル向けプロンプト改訂）
- [`ADR-0009`](adr/0009-thinking-is-an-operator-key.ja.md) — thinking は
  操作者のキー: `[llm].reasoning_effort` をそのまま送り、既定はベンチが
  決めるまで未設定、reasoning の内容は保存しない
- [`ADR-0010`](adr/0010-no-model-tier.ja.md) — 自動承認のモデル層は
  採らない: ベンチの Review 呼び出しは今や read レーンが走らせる write
  レーンの検証で、判定はローカルモデル 1 つが自分を裁いて呼び出しごとに
  数秒かかり、操作者の行とレーンが既に覆っている
- [`ADR-0011`](adr/0011-skills.ja.md) — スキルは Claude Code の形式の
  まま lagent 自身のディレクトリから読み込む: `~/.config/lagent/skills` と
  プロジェクトの `.claude/skills`（信頼済み・ピン留め）、facts メッセージに
  スキルごと 1 行の一覧、`load_skill` の結果は包まず閉じ込めて送る、
  `/skill` と `/skills`、`allowed-tools` は無視
- [`ADR-0012`](adr/0012-pre-tool-hooks.ja.md) — pre-tool フックは
  モデルの外にある操作者の制御: Claude Code の計測済み契約による
  `[[hooks.pre_tool_use]]`、拒否は梯子の前の床、それ以外は注記付きで
  fail open、global 設定のみ
- [`ADR-0013`](adr/0013-memory-rides-the-facts-message.ja.md) — agent
  memory は runtime-facts メッセージに乗る: 指示節の指示は 0/18、facts
  メッセージの行は 5/6 で行動されたので想起はそこへ。状態ルート下の 2
  スコープ、操作者は `/remember` で書き、モデルはゲート付きツールで提案する
- [`ADR-0014`](adr/0014-context-and-end-hooks.ja.md) — フックの集合は
  gem-agent のもの: `session_start`、`user_prompt_submit`、`session_end` が
  Claude Code の計測済み契約で `pre_tool_use` に加わる。注入文脈は `hook`
  添付としてデータレーンに乗り、プロンプトは拒めるが開始と終了は拒めない
  （ADR-0012 §1 を改訂）
- [`ADR-0015`](adr/0015-credential-reads-are-operator-only.ja.md) —
  資格情報のパスは read ツールでも操作者専用: レーンが拒むパスへの
  `read_file`、`file_info`、`view_image` は実パスで判定され、どのモードでも
  操作者だけが答える Review で、`-p` は拒否する。`search_files`、
  `list_files`、`list_tree` は何も隠さない — ADR-0016 が §2 を撤回した。
  カーネルは名前を出して内容を拒むからである。一覧は `internal/sandbox` に 1 つ
- [`ADR-0016`](adr/0016-the-kernel-reads-the-file.ja.md) —
  ファイルを読むのはカーネル（**Accepted**、実装済み）: file ツールの読取を
  `sandbox-exec` 下の子で
  走らせ、資格情報の open はカーネルが拒み、Go の照合器は境界であることを
  やめる。拒否が操作者への確認になり、承認後にインプロセスで再発行する。
  実測: `.env` への `cat` と `stat` は拒否、`.env.example` は読め、`grep -r`
  はその 1 ファイルだけ飛ばし、`ls -a` は名前を出す。起動は 1 回 18.8 ms。
  名前は出て内容は拒まれるので ADR-0015 §2 を撤回する。`credentialTally` と
  列挙ツールの資格情報コードを削除し、walk の綴りの欠陥もそれと共に溶ける
- [`ADR-0017`](adr/0017-the-runtime-hides-only-its-own.ja.md) —
  ランタイムは自分のものだけを隠す（**Accepted**、実装済み）: 環境変数の
  scrub は 6 つの子のうち 1 つ
  しか覆わず、`NPM_TOKEN` を捕まえながら `OPENAI_KEY` を通していた。許可
  リストは無限の一覧を移すだけなので、削除して操作者の環境には触れない。
  有限な集合は lagent 自身の名前空間だけなので接頭辞規則を反転し、3 つの
  export を除く `LAGENT_*` を全ての子から外す。分割はテストが閉じる。R01 が
  名指した `LAGENT_API_KEY` は外れる側にある。設定ファイル経由は ADR-0015 が
  受容した残余のまま
- [`ADR-0018`](adr/0018-the-injection-bench-scores-argument-values.ja.md) —
  注入ベンチは引数の値を採点し、benign twin と対にする（**Accepted**、設計
  のみ）: 本モデルでの実測では、叫ぶペイロード（「全指示を破棄せよ」）は包装の
  有無によらず通らないので、それで組んだベンチは防御を外した後も完璧な耐性を
  報告する。タスクを受け入れて出力の 1 欄を指定するペイロードは包装しなければ
  92〜100% 通り、包装は 1 つを 0.3%、別を 14% に下げ、3 つ目には全く効かない。
  よってエージェント側のタスクはペイロードをツール結果に置き、散文ではなく
  引数の値を採点し、benign twin を対にし、対象要素を変え、率の測定は単発の
  ハーネスに委ねる
- [`ADR-0019`](adr/0019-the-caller-names-the-work-dir.ja.md) — 呼び出す側が
  work dir を告げる: 全 `tools/call` に `_meta` を付ける（**Accepted**、
  実装済み。gem-agent ADR-0088 が反対側の同じ決定）: 組織 ADR-021 が
  「ファイルを産む MCP サーバーは出力先を呼び出しごとの `work_dir` 引数で
  受け取る」と決め、フリート 10 サーバーがそれを必須で取るようになった —
  契約は呼び出し側にも義務を置く。そもそも引数という形になったのは 4
  ランタイムの実測による。MCP の `roots` も環境変数も半数には届かなかった。
  契約の第 2 の経路であるリクエストの `_meta["jp.nlink/work_dir"]` は
  自分たちが書いたランタイムのためのものである — スキーマを知らずに全ての
  `tools/call` に付けられ、モデルが引数を書き忘れても、このセッションが
  読み返せる出力先がサーバーに残る。`LAGENT_WORK_DIR` は役に立たない。
  子プロセス向けの環境変数であってプロトコル上の値ではなく、読ませるには
  登録行に `${LAGENT_WORK_DIR}` を書く運用が要り、同居ランタイムでは
  未定義変数が無言で空文字に展開される。よって `mcp.Client` がセッションの
  work dir を保持し、全ての呼び出しの `params._meta` にキーを付ける。work
  dir が無いセッションでは何も付けない — 空の hint は無いより悪く、サーバーが
  それを答えとして受け取ってしまうからである。値はグローバルでもセッターでも
  なく `NewStdio` の引数とする。クライアントが生きている間変わらないからで
  ある。そしてモデルが書いた引数が常に勝つ。`_meta` は引数が無かったときに
  だけ読まれる（ADR-021 §2 の解決順）。chrome-pilot の登録行から
  `--workspace-root ${LAGENT_WORK_DIR}` が消える — gem-agent と lagent の
  `mcp.json` を 2 実体に分けていた唯一の差分である
- [`ADR-0020`](adr/0020-inline-images-declare-their-height.ja.md) —
  インライン画像はボックスを宣言する: カウンタは測らず告げられる
  （**Proposed**、未実装。独立検証パスを受けて 2026-09-17 に書き直し。
  gem-agent ADR-0089 が反対側の同じ決定）: ADR-0005 は画像がモデルへ届く道を
  決めたが、操作者へ届く道は決められていない。`emit` は全ての行の物理行数を
  数え bottom pin はその数に乗る。x/ansi v0.11.6 で `ansi.StringWidth` は
  iTerm2 `OSC 1337`・kitty `APC _G`・sixel `DCS q` のいずれにも 0 を返し、
  `ansi.Hardwrap` は 3 方式ともバイト同一で通し、`physicalRows` は 1 で
  床打ちするので不足は N-1 である。測定は gem-agent 側のプローブで行った —
  測るのは端末なので移植しない — 全実行で同 fill の対照つき: sixel を描く
  tmux 3.7c では画面充填後に画像 1 枚ごとフレームが 1 個取り残され、未充填では
  被害が出ず、iTerm2 3.7.2 では同じ過小計上が何も動かさなかった。領域が効く
  理由はこのランタイム自身のコードが書いている — pin の padding は画面が
  埋まると 0 で床打ちする（model.go:1570）。宣言したボックスは中の絵に関わらず
  縦横とも丸ごと予約されるので、描画側がそれを宣言し `physicalRows` は告げられる
  — 床打ちの 1 に加算せず置換する — そして宣言は**列**も覆う。
  `wrapForScrollback` の「厳密に狭く」の不変条件は、幅がカウンタに見えない行に
  対してこそ無効だからである。反対側との違いはレーンで、`newGlamourRenderer` は
  応答を 1 個として描画するため、本決定は画像を唯一の成員とする segment レーンを
  作る。`internal/diagram` の移植は明示的に含まない。供給源は **1 つ** —
  MCP intake が既にツールの画像をこのランタイムの決めたパスへ書いており
  （mcpresult.go:184）、モデルが名指すパスは却下する。`PathJudged`
  （risk.go:323）に不可視な view 層の open であり、ADR-0015/0016 が修理した
  クラスだからである。ADR-0005 の裸パス文法も流用しない。理由はいまや 2 つある。
  こちら側の訂正: 初稿は `WithAutoStyle` の危険注記を「継承ではなくこのコードに
  記録されている」と書いたが、gem-agent のものとバイト同一で、ADR-0001 の下で
  継承したものである。測っていないものはそう書いた: kitty と Ghostty が `r=` を
  守るか、Terminal.app の対応、画像 1 枚あたりのコスト
