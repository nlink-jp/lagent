# ADR-0054: セッションは部品から組み立てる — `runREPL` を、することを変えずに分ける

| 項目 | 値 |
|-------|-----|
| Status | **Accepted**（2026-11-18）— 実装済み（追記を参照） |
| Date | 2026-11-18 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | gem-agent の `runREPL` を作り直した後（gem-agent ADR-0124、v0.108.1 でリリース）の操作者の指示:「lagent も同じように作り直して」 |
| Relates to | gem-agent ADR-0124（この設計が取り込む部品と、それを実装して分かったこと）、ADR-0001 と ADR-0002（gem-agent は移植元であって追随先ではない・持ち込まないもの）、ADR-0003（事実はシステムプロンプトではなくメッセージで告げる）、ADR-0012 と ADR-0014（フック）、ADR-0023（安定性の約束）、ADR-0032（モデルの層）、CONVENTIONS ADR-029（実装の前に設計） |

## 背景

### 測ったもの（v0.32.0、2026-11-18）

| 項目 | 値 |
|---|---|
| `cmd/root.go` の `runREPL` | **1,587 行、分岐 226** |
| 次に大きいもの | `tui.Model.Update` 339 行 / 64、`Agent.execCallInner` 320 / 48、`Agent.Run` 303 / 50 |
| 直近 300 コミットで各ファイルを変えた回数 | `cmd/root.go` **70**、`internal/agent/agent.go` 38、`internal/tools/tools.go` 33 |
| 起動後にクロージャが書き換える局所変数 | 15 個（`grant.excluded`、`persistentSnap`、`curLog`、`sessionLog`、`sessionPath`、`sessionID`、`hookSession`、`workDir`、`dates`、`projectContext`、`skillsList`、`mcpClients`、`mcpSummary`、`mcpInv`、`mcpFilter`）、加えてフィールドの `settings.inv`。後から決まるものがさらに 2 つ（`prog`、`ag`） |
| `if prog != nil` による出力の行き先の判断 | 9 か所: ストリームの観測、`askOperator`、`notice`、`OnToolCall`、`OnToolDone`、`OnAttach`、`OnUsage`、`OnAutoDecision`、`resolveWindow` |
| `defer` | `runREPL` 自身に 10 個（`resolveWindow` のクロージャの中にもう 1 個） |
| 記録ファイルの nil 確認 | 8 か所（`sessionLog` に 6、`curLog` に 2） |
| `root.go` のソースを読むか、`runREPL` の流れを写すテスト | 12 ファイル。`cmd/` に `regrant_test`、`wiring_test`、`review3_test`、`grant_test`、`ladder_wiring_test`、`pins_recorded_test`、`review4_test`、`usage_test`、`risktier_test`、`sessions_test`（再開の流れを写す）。`internal/archtest/` に `inert_test`、`unwrap_test` |
| `root.go` の行番号を引用する ADR | 4 か所: ADR-0020（英日）の `root.go:1602`、ADR-0021（英日）の `root.go:918` |

形は、ADR-0124 より前の gem-agent と同じで、理由もそこに記録したとおりである。lagent の `cmd` はそこから移植した（ADR-0001）。起動の順序はコメントだけが守っている。`/clear` と再読込が変えるものは、クロージャが捕まえた局所変数に置かれている。出力の行き先は 9 か所で決めている。後片づけの順序は、1,500 行にわたる 10 個の `defer` の逆順で決まる。テストは関数を呼べないので、写すかソースを読むかしている。入口の 3 つの分岐は、それぞれ `announceGrant()` の後に `ag.Run` でターンを始め、`-p` は `ag.Run` を直接呼ぶ。

### 読んでいて見つけたもの（この決定には含めない）

1. **`session_start` フックが何かを出力すると、`/clear` で TUI が固まる（不具合、実測）。** スラッシュコマンドは Bubble Tea の `Update` の中で同期的に動く（`tui/model.go` の `submit`）。`onClear` は `sessionStartHooks("clear")` を動かし、それは添付した文脈を自分で `notice` を呼んで知らせ、`notice` は `prog.Send` を呼ぶ。`Update` の中からの `Send` は戻らない。待っているチャネルを読むのは `Update` だけだからである。フックの実行役自身の知らせ（フックの失敗・時間切れ）も、`sessionEndHooks("clear")` や `sessionStartHooks("clear")` から同じ経路を通る。v0.32.0 のバイナリを専用の tmux サーバの中で、`--no-sandbox`、`session_start` フック 1 つで測った。`command = "true"` では `/clear` も、その後の `/version` も応えた。`command = "echo context-from-hook"` では `/clear` が応えず、その後に打ったものも何も処理されなかった。gem-agent は同じ不具合に当たっていて、スラッシュコマンドの間に出たフックの知らせをそのコマンドの出力に集めている。lagent のフック（ADR-0014）は、それを持たずに移植された。
2. **起動時に記録ファイルを開けなかったセッションで、`/clear` が panic する（不具合、コードから読んだ。gem-agent と同じで、そちらは v0.108.1 で直した）。** 状態ディレクトリは決まったが、再開でないのに `openSessionLog` が失敗すると、`curLog` は nil のまま残る。その後の `/clear` は自分の記録ファイルを開き、永続ファイルの報告を `curLog.Log` で書く。nil の `*session.Logger` のメソッドで、最初にロックを取るので、報告することがあればセッションは panic する。
3. **`session_end` フックが何かを出すと、終了の要約が最後にならない。** lagent は要約を `session_end` フックより前に動かす。そのため素の REPL では、フックの知らせ（失敗・時間切れ）が再開のコマンドの下に出る。TUI では、知らせはプログラムが終わった後に送られて失われる。gem-agent の保守者が挙げた理由（gem-agent ADR-0065 の追記）と同じ理由で、lagent 自身の決定として提案する: 再開のコマンドと合計が、スクロールバックの最後に残る。
4. **測って決める: `/clear` の永続ファイルの報告が、TUI が描いている最中に stderr へ書く。** `reportPersistent` と `watch.report` は stderr へ直接書く。TUI の下の `/clear` では、それが `Update` の中で起きる。`reconnectMCP` のコード自身のコメントは、TUI の下での stderr の書き込みは画面を壊すと言っている。固まりはしない。gem-agent も同じことをしている。手順 0 の TUI を動かす仕組みで測る。画面が壊れるなら両方のランタイムの不具合で、その修正は、そのときに保守者に諮る。この作業には混ぜない。

## 決定

`runREPL` を、次の部品に作り直す。部品は gem-agent ADR-0124 が導入したもので、中身は lagent のものである。**操作者・モデル・子プロセス・記録の読み手から見えるものは何も変えない** — フラグ、出力の文言と順序、終了コード、記録ファイルのレコード、環境変数、ファイル、起動と終了の手順、システムプロンプト、事実のメッセージ、ツール宣言の順序。ADR-0023 の約束（フラグ・設定キー・`LAGENT_*`・レコード形式・フックの約束）には、作りとして触れない。上の 4 件は、独立した `fix:` コミットで扱い（§3）、`refactor:` に混ぜない。ADR-0002 が挙げる機能のコード・文言・文書は、gem-agent から何も持ってこない。

### 1. 概念設計 — 部品

| 部品 | 約束すること |
|---|---|
| **`runFlags`** | ルートコマンドの `flag*` の値を 1 回だけ読む。今はパッケージ変数の `afterDirectShell` は、`runDirectShell` の引数にする |
| **起動の段** | それぞれ、前の段から要るものを引数で受け取り、型のある結果を返す。今の順序で動くので、どの警告も今と同じところに出る |
| **状態の保持役** | `newLiveSession` が最初に作り、中身はそれを生む段が入れ、ポインタで共有する: 許可、永続ファイルの控え、記録ファイル（nil に強い `log` を持つ）、作業ディレクトリ、プロジェクトの中身（指示・スキル・メモリの保管）、MCP の一式 |
| **`liveSession`** | 起動後の保持役を持つ。起動後に状態を変える処理はそのメソッド。ターンの合間にしか動かない |
| **`surface`** | 操作者に向けた出力の行き先を 1 か所で決める。後から決まるプログラムで判断する、1 つの具体的な型 |
| **入口** | `runOneShot`、`runTUI`、`runPlain`。どのターンも `liveSession.turn` で始める |
| **後片づけの積み上げ** | `push`（最後に積んだものが先に動く）と `pushFinal`（ほかのすべての手の後に、積んだ順で動く） |

### 2. 詳細設計

#### 2.1 起動の段

| 段 | 今の順序のまま移すもの | 注記 |
|---|---|---|
| `resolveRun` | 設定の読み込み、読み取り専用のフラグ、最初のメッセージ、自動承認と天井の実効値、UI の言語、割り込みの梯子 | |
| `admitProject` | プロジェクトのディレクトリ、広すぎるルートの拒否、ポリシーファイル、プロジェクトの信頼とピン、プロジェクトの設定、MCP の除外、承認ポリシー、使われない項目の知らせ、永続ファイルの控え、直接シェルの知らせ | |
| `openTranscript` | 再開先の解決、記録ファイル、復元した履歴 | `opened` を返す |
| `prepareWorkDir` | 作業ディレクトリと、その環境変数 | `opened` を受け取る |
| `buildConfinement` | サンドボックスが使えるか、追加ディレクトリ、レーン、レジストリ、granter、**`request_directory`**（今ここで `load_skill` より前に登録している。レジストリの順はモデルが受け取る宣言の順なので、動かさない）、ファイル読み取りの子 | 作業ディレクトリを受け取る |
| `loadProject` | 指示ファイル、スキル、メモリ、MCP サーバ、advertiser と `mcpStatus`（MCP の保持役越しに読む） | |
| `buildModels` | メインのクライアントと、モデルの層が有効ならそのクライアント（ADR-0032）、層の知らせ | 知らせは今と同じく、後で `notice` で出す |
| `chooseFrontEnd` | TUI かどうか、共有の stdin リーダ、画像、承認の門、行リーダ、`surface`、ストリームの観測 | |
| `registerTools` | `ask_user`、`mcp_load`、ラウンド上限の問い | `toolsRegistered` を返し、`assembleSession` がそれを受け取る |
| `assembleSession` | `composeSystem`、書き込みの見張り、`notice`、層の知らせ、フック、エージェント、復元した履歴、日付、`announceFacts` と `announceGrant`、開始フック、終了の要約、`showImage`、設定の保管、再読込、`clear` | 150 行を超えるならさらに分ける |

#### 2.2 `liveSession` と記録ファイルの保持役

メソッドは、今のクロージャの本体を、名前以外は手を入れずに移したもの: `composeSystem`、`announceFacts`、`turn`、`clear`、`reconnectMCP`、`reloadMCP`（つなぎ直してから `announceFacts` — `/mcp reload` がすること。`clear` は今と同じく、告げずにつなぎ直し、最後に告げる）、`reconnectServer`、`loadMCP`、`sessionStartHooks`、`sessionEndHooks`、`reportPersistent`、`resolveWindow`、そして `slash`（今の 2 つの呼び出し箇所と同じく、`slashOutput` の前に `memorySlash` を動かす）。`slashOutput` は引数を変えない。記録ファイルの保持役の `log` は、記録ファイルが無ければ何も記録しない。見つけたもの 2 の修正の後は、セッションの記録はすべてそこを通る。

#### 2.3 `surface`

メソッドは背景の 9 か所の判断: `stream`、`ask`、`notice`、`toolCall`、`toolDone`、`attached`、`usage`、`autoApproved`、`contextWindow`。どれも今その分岐が出すものをそのまま出す。見つけたもの 1 の修正の後は、`hookNotice` と `collectHookNotices` も持つ。

#### 2.4 モデルの差し替え口

`newModelClient(cfg, model)` は、モデル名ごとにクライアントを作る。`modelClient` は `*llm.OpenAI` を埋め込むインターフェースで、`runREPL` とそれが呼ぶものが使うもの（`ChatStream`、`ContextWindow`、`SetObserver`、`SetReasoningEffort`）を持つ。今 `llm.NewOpenAI` を呼んでいる場所で、メインのモデルと層のモデルのために呼ぶ。層が無効なとき、層のバックエンドは nil のインターフェースのままにする（インターフェースの中に型つきの nil を入れない）。`ModelTier: riskBackend != nil` と、エージェントがメインのクライアントへ戻る判断が、それを確かめるからである。`internal/llm` は変えない。

#### 2.5 終了の手順

今の動く順: 追加ルートの永続ファイル（`watch.report`）、プロジェクトの永続ファイル（`reportPersistent`）、終了の要約（`-p` を除く）、`session_end` フック、MCP クライアントを閉じる、スキルのルートを閉じる、作業ディレクトリが空なら消す、予備の一時領域を消す、記録ファイルを閉じる、`afterDirectShell` を外す。手順 1 は、変数と一緒に無くなる最後の手を除いて、これを再現する。その後、見つけたもの 3 の修正で、要約を最後の手にする。

#### 2.6 判定役

- **特性テスト**（`cmd/characterize_test.go`）: テストバイナリを子プロセスとして起動し直し、モデルは台本にし、基準は v0.32.0 で記録する — ツールを 1 回呼ぶ `-p`、プロンプトごとに 1 行ずつ渡して読むだけのスラッシュコマンドをすべて打つ素の REPL、`session_start` と `session_end` のフックを設定して 2 回の `!env` の間で打つ `/clear`、全体を動かしたときの終了の手順、いくつかの段で起動を失敗させたときの終了の手順（どれも名指した理由で失敗すること）。機械に依存する行（ファイル読み取りの子の知らせ）は比べる対象から外す。
- **TUI を動かす仕組み**: 同じ子プロセスを専用の tmux サーバの中で TUI として動かす。tmux が無ければ飛ばす。
- **ソースを読むテスト**: 背景の 12 ファイル。どれも、そのコードを動かす手順の中で、`cmd` の全ファイル（または組み立てのファイルの一覧 — 一覧そのものも漏れを検査する）を読むように向け直す。`archtest/inert_test` は `runREPL` で通り続ける。
- **ADR の引用**: ADR-0020 と ADR-0021 の `root.go` の行番号。そのコードが動いたときに向け直す。

### 3. 開発計画

| 手順 | コミット | 完了の条件 |
|---|---|---|
| — | `fix:` 見つけたもの 2、テストつき | 守りのテストが通る。ほかは何も変わらない |
| 0 | `test:` モデルの差し替え口、特性テスト、TUI を動かす仕組み | このコミットの親で記録した基準が通り、繰り返しても一致する |
| — | `fix:` 見つけたもの 1: `/clear` の間に出たフックの知らせ（実行役のものと、`sessionStartHooks` の添付の知らせ）を、どの入口でも `/clear` の出力に集める | 出力する `session_start` フックでの TUI のテストが、前は落ち、後は通る。素の REPL では、その知らせが `/clear` の出力の前からその中へ移るので、`/clear` の基準はちょうどその行だけ変わる。置き換える前に確かめる |
| — | 見つけたもの 4 を TUI の仕組みで測る | 結果をここに記録し、画面が壊れるなら保守者に諮る |
| 1–5 | `refactor:` フラグと後片づけ、`surface`、保持役と `liveSession`、段、入口と `turn` | `make check` が通り、基準がバイト単位で一致し、動かしたコードのソースを読むテストと ADR の引用を同じコミットで向け直す |
| — | `fix:` 見つけたもの 3: 終了の要約を最後の手にする | 失敗する `session_end` フックで素の REPL を動かすと、要約が最後に出る。テストは前に落ちる |
| 6 | `docs:` AGENTS.md、構成の文書（英日）、測り直した値を載せるこの記録の追記 | `make check` が通る |

TUI は、実際の端末（tmux、台本のモデル）でも、v0.32.0 と最後の手順の間で比べる。

### 4. 検証（始める前に回数を決める）

- **この設計**: 記録済みの規則と、今の `runREPL` がしていることに照らして、独立した検証を 1 回。2026-11-18 に実施: 所見 12 件をすべて上に反映した（見つけたもの 2 の panic、§2 の中身を「gem-agent のとおり」ではなく lagent のものとして書いたこと、リリース前の検証の回数、見つけたもの 3 を lagent 自身の決定として書いたこと、背景の数、`request_directory` の段、見つけたもの 4、見つけたもの 1 の修正が素の REPL に与える影響、モデルの差し替え口のメソッドと層の nil、`memorySlash`・`reloadMCP`・`mcpStatus`・層の知らせの置き場所）。
- **実装**: §1〜§2 への適合を、特性テストで判定する。リリースの前に、差分全体へ `/code-review` を 1 回と、修正の差分だけにもう 1 回。3 回目は保守者の了承が要る。修正の差分への検証で high が出たら、この設計へ戻る。

### 5. gem-agent

ここでは gem-agent を何も変えない。ただし、見つけたもの 4 の測定の結果、保守者が決めることは除く。2 つのランタイムの `cmd` は、同じ名前の同じ部品を持つようになる。`cmd` の組み立ては、AGENTS.md の共有の仕組みの一覧の外のままである。

## 検討した代替案

| 代替案 | 採らない理由 |
|---|---|
| lagent はそのままにする | いちばんよく変わるファイルがいちばん読みにくく、テストはそれを写していて、gem-agent が直した不具合を 2 つ抱えていた（見つけたもの 1、2）。 |
| gem-agent の新しいファイルを写す | 中身が全体に違う（テレメトリが無い、プロンプトを作り直さず事実のメッセージで告げる、層は自分のクライアント、フックの配線が違う）。写すと gem-agent のコードと文言が入り、ADR-0002 が挙げる機能についてはそれを禁じている。部品は取り込み、中身は lagent のものを保つ。 |
| 見つけたもの 1 と 2 を直して止める | どちらも「出力の行き先を 9 か所で決める」「状態がクロージャの中にある」というクラスの 1 例で、作り直さずに直してもクラスは残る。いずれにせよ最初に単独で直すので、残りがどうなっても、リリースはこれらの修正を運べる。 |

## 追記: 実装（2026-11-18/19）

§3 の順に lagent の `main` に実装した。`/clear` の panic の修正（`e527ee4`）、手順 0（`dbfe2bd`）、`/clear` で固まる件の修正（`d99943a`）、手順 1〜5（`a2666bd`、`a3085c2`、`0d70b25`、`7b5defb`、`e1c992f`）、テストの修正（`d35eff2`）、終了の要約の修正（`5aecaa0`）、そしてこの記録の手順 6。どの手順でも `make check` が通り、特性テストの基準は一致し続けた。基準を変えたのは下に書く 1 回だけである。

### 測り直した値

| 項目 | 前（v0.32.0） | 後 |
|---|---|---|
| `runREPL` | 1,587 行、分岐 226 | **66 行** |
| `cmd/root.go` | 2,808 行 | 1,307 行（残りは `startup.go` 1,021、`live.go` 505、`frontends.go` 353、`surface.go` 151、`cleanups.go` 62、`modelclient.go` 32） |
| 新しいファイルで最大の関数 | — | 150 行未満（`assembleSession` は `assembleHooks`・`assembleAgent`・残りに分けた） |
| `surface` の外の `if prog != nil` | 9 | 0 |
| フィールドへ書き換えた局所変数 | — | 40 個の 611 か所。型検査器の解決から書き換え、混ざった定義 5 か所は手で直した |

### 見つけたもののその後

1. **見つけたもの 1、`/clear` で固まる件** — `d99943a` で直した。`TestTUIClearWithAPrintingSessionStartHook` が tmux の中で TUI を動かし、直す前のコミットで落ちた。特性テストの基準は変わらなかった。その場面のフックは何も出力しないからである。素の REPL の新しい形（知らせが `/clear` の出力の中に入る）は `TestPlainClearCarriesTheHookNotice` が固定する。
2. **見つけたもの 2、`/clear` の panic** — `e527ee4` で直した。手順 3 からは `transcript.log` が nil に強い道になった。
3. **見つけたもの 3、終了の要約** — `5aecaa0` で `pushFinal` を使って直した。`TestExitSummaryIsLastAfterASessionEndHook` は直す前のコミットで落ちた。
4. **見つけたもの 4、測った結果: 画面は壊れないが、知らせが消える。** TUI のセッションの途中で AGENTS.md を変えて `/clear` を打つと、記録ファイルには `persistent_changes` が残る。一方、stderr へ書いた行（"note: this session added or changed: AGENTS.md"）は、画面にも端末のスクロールバックにも出ない。TUI の再描画が上書きする。素の REPL では出る。gem-agent も同じ書き方をしている。保守者に諮り、見せるべきと決まった（2026-11-19）。直す前に、起動時に決めた書き先へ書く箇所のうち、TUI が画面を持っている間に動きうるものを全部挙げた。`/clear` でのプロジェクトの報告と追加ルートの報告の 2 つだけだった。MCP の再読み込みと付与時の監視は既に知らせを返り値で返し、残りは TUI の前か後に動く。両方の報告は、書き先を呼ぶたびに受け取るようにした。`/clear` は自分の出力を、終了時の後片づけは端末を渡す。`addedWatch` は書き先を持たないので、後から書く先が残らない（`f494514`）。`TestTUIClearNamesAChangedPersistentFile` が TUI で動かして確かめ、修正前は落ちる。gem-agent にも同じ修正を入れた。
5. **見つけたもの 5（見つけたもの 4 の入口を挙げる途中で見つけた）: TUI が終わった後に言うことが消える。** 行が消える道は、TUI の実行中に端末へ書くことのほかに、もう終わったプログラムへ送ることがある。surface は `prog.Run` が戻った後もプログラムを持ち続けていた。Bubble Tea の `Send` は、終わったプログラムに対しては取り消し済みのコンテキストを選んで何もしない。tmux で測ると、終了時に失敗した `session_end` フックの知らせは、画面にもスクロールバックにも出なかった。素の REPL では出る。`runTUI` は `Run` が戻ったらプログラムを外すようにした。surface は再びストリームへ書き、知らせは終了の要約より前に出る（`abfa01a`）。セッションが起こしたゴルーチンがプログラムの終わりぎわにまだ報告しうるので、このフィールドは atomic にした。`TestTUIExitShowsTheSessionEndHookNotice` が確かめ、修正前は落ちる。gem-agent にも同じ修正を入れた。

### §2〜§3 と違うところと、その理由

1. **`buildConfinement` はエラーだけを返す。** それが作るもので後の段が読むものは、すべてセッション越しに読むので、結果の型は作らなかった。
2. **TUI の比較は v0.32.0 ではなく `d99943a` と行った。** 動かす仕組みの設定には出力する `session_start` フックが入っていて、v0.32.0 は `/clear` で固まった（見つけたもの 1）。そこで、それを直し、作り直しは何もしていないコミットと比べた。記録ファイルは同一で、画面の違いは乱数の id、一時ディレクトリの名前、tmux の終了時刻だけだった。
3. **特性テストの終了の記録の切り替えは `CHARACTERIZE_LAGENT_EXIT` という名前にした。** `/clear` の場面はセッションの `LAGENT_` の環境を出力し、テストの道具の変数はその一部ではないからである。
4. **曜日だけの表記も正規化した**（`d35eff2`）。事実のメッセージは開始日を "(Wednesday, JST)" と書き、基準は日付をまたぐと落ちた。基準はちょうどその置き換えの分だけ変わった。gem-agent の基準にも同じ漏れがあり、同じ修正を入れた。
5. **`request_directory` のテストは、起動が行う登録を読む。** `buildConfinement` は起動と同じくそのツールを登録するので、テストが 2 回目の登録をすることはやめた。

### 何で判定したか

v0.32.0 に panic の修正を足したもので記録した特性テスト。どの場面でも、また 5 か所で起動を失敗させたときの終了の手順。tmux の中で動かした TUI。写しの代わりに `openTranscript` と `buildConfinement` を呼ぶ再開と許可のテスト。`cmd` の全ファイル、または組み立てのファイルを読むソース照合のテスト。その一覧自身も検査していて、漏れていた `startup.go` と `frontends.go` を名指しした。stdin リーダのテストが、リーダが root.go から出た後も root.go だけを読んでいたことも、ここで見つかった。§4 のリリース前の検証（差分全体と、修正の差分）は、これを出すリリースの前にかける。
