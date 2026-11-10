# ADR-0048: Linux 版は出さない — write レーンの名前による拒否を持ち込めない

| Field | Value |
|-------|-------|
| Status | **Accepted** (2026-11-11) |
| Date | 2026-11-11 |
| Binds | lagent |
| Decision makers | nlink-jp maintainers |
| Triggered by | gem-agent に対して、Windows は WSL2 で動かす前提の Linux 版の要望があった。gem-agent では実測のうえ見送った（gem-agent ADR-0115）。測った機構はこのランタイムと共有しているものなので、操作者がここにも決定を記録するよう求めた |
| Ports | gem-agent ADR-0115 |
| Relates to | ADR-0001（移植元: レーンと `internal/sandbox`）、ADR-0032（write レーンのシェルコマンドはモデル層が判定する） |

## Context

lagent は gem-agent と同じく、隔離が `sandbox-exec` の上に作られているので macOS 専用である。別のカーネルが同じ保証を担えるかは、gem-agent ADR-0115 まで測られていなかった。

問題になる性質はここでも同じである。永続ファイル（`AGENTS.md`、`CLAUDE.md`、`.mcp.json`、`.lagent.toml`、`.claude/` ほか `sandbox.PersistentFiles` の一覧）への `write_file` / `edit_file` による書き込みは、必ず操作者に尋ねる。write レーンの `shell_exec` は、この点について文面では判定されない。カーネルがそれらのファイルを**名前で、どの深さでも、そのファイルがまだ無くても**拒否する。`write_file` で断られたモデルは次に `shell_exec` を試し、macOS ではその試みは、コマンドの綴りがどうであれカーネルで失敗する。

自動承認のもとでは、write レーンのシェルコマンドをモデル層が承認できる（ADR-0032）。カーネルの拒否が無ければ、操作者が何も読まないまま新しい指示ファイルが現れうる。

## Measurement

この記録のために測り直してはいない。`internal/sandbox` とレーンは gem-agent と共有しており、実測はランタイムに依存しない。Linux で非特権のプロセスに何を拒否できるか、についての実測だからである。機体、方法（拒否にはすべて、機構なしで成功しなければならない対照がある）、表の全体は gem-agent ADR-0115 にある。Linux の 2 台で 2026-11-11 に分かったこと:

| macOS の write レーンが保証すること | Landlock 単独 | 名前空間 |
|---|---|---|
| 既にある `AGENTS.md` を書けず、消せず、改名できない | **許された** | 拒否された |
| 新しい `AGENTS.md` を作れない | **許された** | **許された** |
| `AGENTS.md` を含むディレクトリを改名できない | 未測定 | **許された** |

既にあるものは守れ、まだ無いものは守れない。read レーンの保証（書けない、ネットワークに出られない、親へシグナルを送れない、資格情報のディレクトリを読めない）は持ち込めた。

## Decision

1. **lagent の Linux 版は出さない。** macOS 専用のままとする。
2. **Windows の経路として WSL2 を案内しない。** それは Linux 版である。
3. **サンドボックスの制御を**、第 2 の機構のために**抽象化しない。**
4. **再検討のきっかけは需要ではなく機構である。** 条件は gem-agent ADR-0115 §4 のとおり: 書き込める木の中で名前のパターンによって作成と改名を断れる、非特権で使えるカーネルの機能が、対照つきのプローブで示されること。gem-agent が再検討するなら、この記録も一緒に再検討する。機構は共有で、直すときは両方で直す（AGENTS.md の "The sibling runtime"）。

## Consequences

- Makefile のターゲットは 1 つのままで、`AGENTS.md` と `CLAUDE.md` の「linux / windows のビルドターゲットを足さない」も残り、この記録を指す。
- Linux の機体でローカルモデルを動かすのは自然な組み合わせだが、それには提供できない。これが費用である。

## Alternatives considered

gem-agent ADR-0115 で比べた 4 つがそのまま当てはまり、同じ理由で採らない。Linux では閉じ込めなしを既定にする。read レーンは再現して write レーンのコマンドは毎回尋ねる。防ぐ代わりに書かれた後で検知する。重ねがけ（overlay）に書かせて名前の判定を通して反映する。2 つ目を採ると、Linux でだけ ADR-0032 も取り下げることになる。

## References

- gem-agent ADR-0115 — <https://github.com/nlink-jp/gem-agent/blob/main/docs/ja/adr/0115-no-linux-version-the-write-lanes-denial-by-name-does-not-carry-over.ja.md>
- ADR-0001、ADR-0032
