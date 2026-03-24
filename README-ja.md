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
- **単一バイナリ配布**: フロントエンド、マイグレーションSQL等を全てバイナリに同梱。Windows/macOS/Linux対応。

## 2. 使い方

ポート53/80/443のバインドに管理者権限が必要です。サービスとして登録すると管理者権限で実行されます。

### Step 1. 動作確認

フォアグラウンドで起動し、正常に動作することを確認します（Ctrl+Cで停止）。

```bash
# Windows: 管理者として実行したコマンドプロンプトで実行
# macOS/Linux:
sudo ./devgatewaydns serve
```

管理UIが `http://<サーバーIP>:9090` で開けることを確認してください。

### Step 2. サービス登録

動作確認後、OSサービスとして登録します。サービスは管理者権限で実行されるため、以降 `sudo` は不要です。起動オプションはサービスに保存されます。

```bash
# Windows: 管理者として実行したコマンドプロンプトで実行
# macOS/Linux:
sudo ./devgatewaydns install
./devgatewaydns start
```

### Step 3. サービス管理

```bash
./devgatewaydns stop       # 停止
./devgatewaydns start      # 開始
./devgatewaydns status     # 状態確認
./devgatewaydns uninstall  # 登録解除
```

### 起動オプション（serve / install 共通）

| オプション | デフォルト | 説明 |
|---|---|---|
| `--http-port` | 80 | HTTP受付ポート |
| `--https-port` | 443 | HTTPS受付ポート |
| `--dns-port` | 53 | DNS受付ポート |
| `--proxy-port` | 8888 | フォワードプロキシポート |
| `--admin-port` | 9090 | 管理UIポート |
| `--listen` | 0.0.0.0 | LISTENアドレス（複数指定可） |
| `--db` | (バイナリ同ディレクトリ)/devgatewaydns.db | DBファイルパス |

例:

```bash
./devgatewaydns serve --listen 192.168.1.10
./devgatewaydns install --listen 192.168.1.10
```

## 3. 開発者向けリファレンス

### 開発ルール

- 開発者の参照するドキュメントは`README.md`を除き`Documents`に配置すること。
- 対応後は必ずリンターで確認を行い適切な修正を行うこと。故意にリンターエラーを許容する際は、その旨をコメントで明記すること。 **ビルドはリリース時に行うものでデバックには不要なのでリンターまでで十分**
- モデルの実装時は、テーブル単位でファイルを配置すること。
- 部品化するものは`modules`にファイルを作成して実装すること。
- 一時的なスクリプトなど（例:調査用スクリプト）は`scripts`ディレクトリに配置すること。
- モデルを作成および変更を加えた場合は、`Documents/テーブル定義.md`を更新すること。テーブル定義はテーブルごとに表で表現し、カラム名や型およびリレーションを表内で表現すること。
- システムの動作などに変更があった場合は、`Documents/システム仕様.md`を更新すること。

### ファイアウォールの設定

OS付属またはセキュリティソフト付属のファイアウォールが受信接続をブロックする場合があります。`--listen` で指定したIPアドレスのヘルスチェックが失敗する場合は、ファイアウォール設定でデバッグバイナリの受信接続を許可してください。

ワイルドカード指定が可能な場合:
`<プロジェクトルート>/__debug_bin*`

ファイル指定の場合:
`<プロジェクトルート>/__debug_bin<数字>`

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
