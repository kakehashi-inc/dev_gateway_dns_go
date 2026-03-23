# DevGatewayDNS

## 1. システム概要

システムの概略をここに記載してください。

## 2. 開発者向けリファレンス

### 開発ルール

- 開発者の参照するドキュメントは`README.md`を除き`Documents`に配置すること。
- 対応後は必ずリンターで確認を行い適切な修正を行うこと。故意にリンターエラーを許容する際は、その旨をコメントで明記すること。 **ビルドはリリース時に行うものでデバックには不要なのでリンターまでで十分**
- モデルの実装時は、テーブル単位でファイルを配置すること。
- 部品化するものは`modules`にファイルを作成して実装すること。
- 一時的なスクリプトなど（例:調査用スクリプト）は`scripts`ディレクトリに配置すること。
- モデルを作成および変更を加えた場合は、`Documents/テーブル定義.md`を更新すること。テーブル定義はテーブルごとに表で表現し、カラム名や型およびリレーションを表内で表現すること。
- システムの動作などに変更があった場合は、`Documents/システム仕様.md`を更新すること。

### Go 操作コマンド

デバッグモジュールの追加・更新

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

モジュールの追加

```bash
go get <package-name>
```

モジュールの追加・ビルド

```bash
go install <package-name>
```

モジュールファイルの作成

```bash
go mod init <module-name>
```

モジュールのダウンロード（モジュール名を省略するとgo.modの全て）

```bash
go mod download <module-name>
```

モジュールの最適化（ソースとgo.modの双方向での一致）

```bash
go mod tidy
```

モジュールの最新化

```bash
go get -u
```

Go バージョンの更新

```bash
go mod tidy --go=1.25
```

キャッシュのクリア

```bash
go clean --cache --testcache
```

### ビルドやリリース方法

ビルド

```bash
go build
```

リリース

```bash
make
```
