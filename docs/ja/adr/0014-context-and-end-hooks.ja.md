# ADR-0014: フックの集合は gem-agent のもの — session start、prompt submit、session end が pre-tool に加わる

| 項目 | 内容 |
|------|------|
| ステータス | **Accepted** |
| 日付 | 2026-09-12 |
| 拘束対象 | lagent |
| 決定者 | nlink-jp maintainers |
| きっかけ | 操作者: hooks は 1 イベントではなくフルセットで実装すべき |
| 改訂対象 | ADR-0012 §1（1 イベント → 同じ機構で gem-agent の 4 イベント） |

## 背景

ADR-0012 §1 は `PreToolUse` を実装し、消費者が現れるまで他のイベントを
見送った。契機の無い能力は死重だという根拠による。操作者の指示はフルセット
である。その論拠は drop-in 互換で、ADR-0012 §4 は 1 イベントについて既に
それを重んじている: Claude Code と gem-agent に 1 つの hooks ブロックを
持つ操作者はこのランタイムにも同じブロックを持ち、消費者が来たときに移植
するものが無い。しかも gem-agent が 2 イベントを足した消費者（ランタイム
横断の共有知識空間、gem-agent ADR-0069）は、このランタイムがそれらの
イベントを持って初めて参加できるものだ。ここでの「フルセット」は gem-agent
の 4 つ: Claude Code の他のイベント（PostToolUse、Stop）はどちらの
ランタイムにも無く、それぞれ固有の配送規則を要する。

gem-agent は 3 つの契約を文書ではなく Claude Code 自身に対して計測した
（gem-agent ADR-0069 §2–3、ADR-0071 §4）: `SessionStart` のペイロードは
`source`（`startup` / `resume` / `clear`）を運び、`UserPromptSubmit` は
打ち込んだ本文を `prompt` で、`SessionEnd` は `reason` を運ぶ。exit 0 の
平文 stdout は注入される文脈で、JSON オブジェクトは判定であり、その
`hookSpecificOutput.additionalContext` だけが文脈。プロンプトは exit 2 か
いずれかの block 形式で拒め、ターンは始まらない。session start と end は
拒めない。

配送には lagent 自身の記録 2 つが効く。ADR-0003 はシステムプロンプトを
バイト同一に保つので、注入文脈はそこへ行けない。ADR-0013 はこのモデルが
何に行動するかを計測した: runtime-facts メッセージ、または打ち込んだ入力の
隣にある、行動すべきものとして枠付けられた行。フックの出力は、そのコードが
読んだもの何であれ — 発端の設計では他セッションが書く保存庫 — の上で走った
コードの産物なので、データであり、そう名乗る。

## 決定

1. **さらに 3 イベント、lagent 自身の設定から、global のみ。**
   `[[hooks.session_start]]`（任意の `matcher` で source を選ぶ: `startup`、
   `resume`、`clear`、`a|b`、`*`）、`[[hooks.user_prompt_submit]]`、
   `[[hooks.session_end]]`（matcher 無し。全プロンプト・全終了で走り、
   matcher があれば設定エラー）。ADR-0012 §1 のプロジェクト禁止は維持:
   文脈フックは毎ターン走る。
2. **ペイロードは Claude Code のもので、lagent が持つフィールド。**
   `hook_event_name`、`session_id`、`transcript_path`（ログ無効時は空）、
   `cwd`、それに `source`、`prompt`、`reason` のいずれか。`session_start`
   は起動時に `startup`、`--continue`/`--resume` 下では `resume`、`/clear`
   で `clear` として発火する。`user_prompt_submit` はモデルに届く全ターン —
   打ち込んだメッセージ、argv の最初のメッセージ、`/skill` 展開ターン、`-p`
   のプロンプト — で発火し、スラッシュコマンドと `!` エスケープでは
   発火しない。`session_end` は終了時に `exit`、`/clear` では新セッション
   開始前に旧セッションについて `clear` で発火する。
3. **出力は文脈か判定。プロンプトは拒める。** exit 0 の平文 stdout は文脈。
   JSON オブジェクトは判定で、その `additionalContext` が文脈。プロンプト
   フックは stderr を理由とする exit 2、または ADR-0012 §3 が受け入れる
   いずれかの block 形式で拒み、プロンプトは消える — 履歴にも transcript
   にも入らない — 操作者は理由を見る（`-p` はそれを添えて非ゼロ終了）。
   最初の block が勝ち、他のフックの文脈は捨てる。block する session_start
   フックは報告される失敗で何も注入しない。session end は出力を無視する。
   それ以外は pre-tool と同じく注記付きで fail open。
4. **注入文脈はデータレーンに乗る。** フックの出力は次の user メッセージの
   kind `hook` の添付 — transcript では打ち込んだ本文の隣に保存され、
   ワイヤ上ではその後ろにターンの nonce タグ内で平坦化され、引用データと
   告げられる。パイプ stdin が使うレーンである。システムプロンプトは
   不触、打ち込んだ入力も不触、モデルにはそれが何かを伝える。フックごと
   8000 ルーンで可視の切断、注入ごとに注記 1 行で、チャネルが静かで
   ないようにする。
5. **`/clear` は end の後に start。** 旧セッションの `session_end`（`clear`）
   は新 transcript が引き継ぐ前に走り、新セッションの `session_start`
   （`clear`）の出力は最初の新ターンに乗る。

## 結果

- 1 つの hooks ブロックが 3 ランタイムに効き、gem-agent でこれらの
  イベントを必要とした知識空間の設計は同じスクリプトをここに登録できる。
- 毎ターン、設定した `user_prompt_submit` フックごとにプロセス起動 1 回。
  開始と終了はフックごと 1 回。未設定なら何も走らない。
- `agent.Options` に `PromptHook` が加わり、`Run` は何も記録する前に
  `ErrPromptBlocked` を返しうる。`internal/hooks` は gem-agent の
  パッケージの完全な移植になり、ADR-0012 の「1 イベントに削った」注記は
  上書きされる。
- このモデルがフック注入データに行動するかは未計測。memory を計測した
  ベンチ（ADR-0013）が、計測すべき消費者の出力ができたときに計測する。

## 検討した代替案

- **消費者を待ち続ける**（ADR-0012 §1）。操作者の指示で保留解除。互換の
  論拠は消費者無しでも立つ。
- **session-start の出力を facts メッセージへ**（ADR-0013 が memory に
  選んだチャネル）。却下: facts メッセージはランタイム自身の言葉で、フック
  の出力は検分されない入力の上で走ったスクリプトのもの。出所を述べて
  データレーンに置く。
- **PostToolUse と Stop。** 不採用: どちらのランタイムにも無く、契約は
  未計測で、それぞれ固有の配送の問いを持つ。
