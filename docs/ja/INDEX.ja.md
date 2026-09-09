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

## リファレンス

現在の挙動。コードの変更に合わせてその場で更新する。

- [`reference/configuration.ja.md`](reference/configuration.ja.md) —
  インストール、設定ファイル、優先順位、コマンド表とフラグ表
- [`reference/architecture.ja.md`](reference/architecture.ja.md) —
  パッケージ配置、バックエンド、ターンループ、承認、ラウンド階梯、
  永続化、ここに無いもの

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
