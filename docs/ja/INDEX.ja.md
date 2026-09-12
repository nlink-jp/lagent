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
  `list_files`、`list_tree` はそのファイルを飛ばし、飛ばしたと言う。一覧は
  `internal/sandbox` に 1 つ
