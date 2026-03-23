# DevGatewayDNS

[English / 英語](README.md)

## 1. システム概要

DevGatewayDNSは、社内LANにおいてWiFi接続のスマートフォン等を含む全クライアントから、ホスト名ベースで仮想Webページにアクセス可能にする統合開発ツールです。

主要機能:

- **リバースプロキシ**: HTTP/HTTPS受信をホスト名ベースでバックエンドサービスへ転送。SNI振り分け、ヘッダ/Cookie自動処理に対応。
- **DNSサーバー**: プロキシルールに連動するAレコードの自動生成、手動レコード管理、NIC別上流DNS転送に対応。
- **フォワードプロキシ**: DNS設定変更が困難なクライアント (iOS等) 向けにHTTPプロキシを提供。
- **SSL証明書管理**: 自己署名CA証明書の自動生成、ホスト別証明書の自動発行、QRコードによるモバイル端末への配布。
- **Web UI**: プロキシ設定、DNS管理、証明書管理、ステータスモニタ、システム設定をブラウザから操作。日本語/英語対応。
- **REST API / WebSocket**: 管理UI向けの全機能API、リアルタイムログ配信。
- **OSサービス登録**: Windows/macOS/Linuxサービスとして登録・管理可能。
- **単一バイナリ配布**: フロントエンド、マイグレーションSQL等を全てバイナリに同梱。6プラットフォーム対応。

技術スタック: Go, SQLite (WAL), codeberg.org/miekg/dns, kardianos/service, nhooyr.io/websocket, pressly/goose v3

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
