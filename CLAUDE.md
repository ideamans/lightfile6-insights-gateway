# Lightfile6 Insights Gateway 開発計画書

## 1. プロジェクト概要

### 1.1 目的

ソフトウェアユーザーの状況把握やトラブルシューティングのための情報を収集し、S3 へ転送する Data Ingestion Service（データ取り込みサービス）を開発する。

### 1.2 プロジェクト名

- **名称**: Lightfile6 Insights Gateway
- **パッケージ名**: `github.com/ideamans/lightfile6-insights-gateway`
- **バイナリ名**: `lightfile6-insights-gateway`

### 1.3 位置づけ

AWS Kinesis Data Firehose のような、以下の一般的な名称で呼ばれるシステムの一種：

- Data Ingestion Service（データ取り込みサービス）
- Data Collector（データ収集器）
- Log Aggregator（ログ集約器）
- Data Pipeline Receiver（データパイプライン受信器）

## 2. 機能要件

### 2.1 API エンドポイント

#### PUT /usage

- レポートファイルを受信
- 認証: USER_TOKEN ヘッダー必須

#### PUT /error

- エラーレポートファイルを受信
- 認証: USER_TOKEN ヘッダー必須

#### PUT /specimen

- 検体ファイルを受信
- クエリパラメータ: uri（ファイル URI）必須
- 認証: USER_TOKEN ヘッダー必須

#### GET /health

- ヘルスチェック
- 認証: 不要

### 2.2 認証

- リクエストヘッダに USER_TOKEN を渡す
- なければエラー
- 詳しい認証は後ほど追加実装するので、ひとまず USER_TOKEN=ユーザ名でよい

### 2.3 データフロー

#### キャッシュ

受け取ったファイルはローカルに保存してレスポンスを返す：

- `/var/lib/lightfile6-insights-gateway/`
  - `usage/` - timestamp.プロセス ID.ユーザ名で保存
  - `error/` - timestamp.プロセス ID.ユーザ名で保存
  - `specimen/` - timestamp.プロセス ID.uri(URL エンコード)で保存

#### S3 への転送

**usage と error:**

- 10 分ごと（設定で変更可）に既存のファイル名の古い順から合成して gz 圧縮
- S3 の設定で与えた usage 用、error 用のバケットにアップロード
- ファイル名: `YYYY/MM/DD/HH/YYYYMMDDHH.ホスト名.jsonl.gz`（UTC で計算）
- 処理の中断を想定した設計：
  - 対象ファイルを aggregation ディレクトリにまずは移動
  - aggregation ディレクトリにファイルがある場合はそれらを処理
  - gz 化したファイルは uploading ディレクトリに移動
  - uploading ディレクトリにあるファイルはまとめて処理
  - アップロードの完了を確認したファイルを削除

**specimen:**

- 待つことなく 1 個ずつ S3 の specimen 用バケットに送信
- 送信トラブルをリトライできるよう設計
- ファイル名: `ユーザ名/YYYY/MM/DD/uri.timestamp.推定拡張子`
- メタデータに URI を保存
- YYYY/MM/DD は UTC で計算

### 2.4 グレースフルシャットダウン

- Web は応答中のリクエストが完了してから応答を停止
- その後、未送信のファイルがあればそれらの処理（集約して送信）してからプロセスを終了

## 3. 設定仕様

### 3.1 設定項目

シンプルな設定ファイル（config.yml）で管理：

- キャッシュディレクトリ（デフォルト: `/var/lib/lightfile6-insights-gateway`）
- AWS の認証（必須）
- AWS のリージョン（デフォルト: ap-northeast-1）
- AWS のエンドポイント
- usage バケット（必須）
- usage プレフィックス
- error バケット（必須）
- error プレフィックス
- specimen バケット（必須）
- specimen プレフィックス
- usage 集約間隔（デフォルト: 10 分）
- error 集約間隔（デフォルト: 10 分）

### 3.2 プログラム引数

- `-p` ポート番号（必須）
- `-c` 設定ファイルパス（デフォルト: /etc/lightfile6/config.yml）

## 4. ログ仕様

STDERR にシンプルにレベル別に出力：

- HTTP リクエストの処理完了
- HTTP リクエストエラー
- usage 集約送信の完了
- usage 集約送信のエラー
- error の集約送信の完了
- error の集約送信のエラー
- specimen の送信完了
- specimen の送信エラー
- シャットダウン処理開始

## 5. テスト要件

### 5.1 単体テスト

- Web リクエスト
- モックを用いた usage, error, specimen の集約送信

### 5.2 結合テスト

- dockertest で MinIO を使う
- プログラムをビルドする
- 空きポートの利用でプロセス起動し Web 待受
- ユースケースのランスルー
  - Web リクエストした情報が MinIO に取り込まれるか
  - 大きなファイル（5MB など）の送信
  - グレースフルシャットダウン
    - Web リクエスト遅延モード（環境変数）を用意
    - 指定の ms だけ応答を遅らせる
    - その上でいくつかリクエストを送信
    - プロセス終了を指示し終了を待つ
    - リクエストに欠損がないこと
    - キャッシュディレクトリに残置ファイルがないこと
    - MinIO にファイルがアップロードされていること

## 6. CI/CD 要件

### 6.1 GitHub Actions

- linux ランナーの go1.23 のみでよい
- 任意のブランチで単体テストを実行
- develop、main ブランチで単体テストと結合テストを実行
- v タグで goreleaser によるビルドと公開

## 7. 技術スタック

### 7.1 使用技術

- **言語**: Go 1.23
- **Web フレームワーク**: Echo (github.com/labstack/echo/v4)
  - 高性能で軽量
  - 優れたミドルウェアサポート
  - コンテキスト管理が優秀
- **ロガー**: zerolog
- **設定管理**: viper
- **AWS SDK**: aws-sdk-go-v2
- **テスト**: testify, dockertest

### 7.2 プロジェクト構成

```
github.com/ideamans/lightfile6-insights-gateway
├── cmd/
│   └── gateway/
│       └── main.go                  # エントリーポイント
├── internal/
│   ├── api/
│   │   ├── handler.go              # HTTPハンドラー
│   │   ├── middleware.go           # 認証ミドルウェア
│   │   └── router.go               # ルーティング設定
│   ├── cache/
│   │   ├── manager.go              # キャッシュファイル管理
│   │   └── paths.go                # パス生成ロジック
│   ├── config/
│   │   ├── config.go               # 設定構造体
│   │   └── loader.go               # 設定ファイル読み込み
│   ├── s3/
│   │   ├── client.go               # S3クライアント
│   │   ├── uploader.go             # アップロード処理
│   │   └── aggregator.go           # ファイル集約処理
│   ├── worker/
│   │   ├── scheduler.go            # 定期実行スケジューラー
│   │   └── processor.go            # バッチ処理実装
│   └── shutdown/
│       └── graceful.go             # グレースフルシャットダウン
├── test/
│   ├── integration/                # 結合テスト
│   └── testdata/                   # テストデータ
├── .github/
│   └── workflows/
│       ├── test.yml                # テスト用ワークフロー
│       └── release.yml             # リリース用ワークフロー
├── Dockerfile
├── docker-compose.yml              # 開発環境用
├── .goreleaser.yml                 # GoReleaserの設定
├── go.mod
├── go.sum
└── README.md
```

## 8. 実装計画

### 8.1 フェーズ 1: 基本機能（1 週目）

- プロジェクトセットアップ
- 設定管理の実装
- HTTP ハンドラーの実装
- キャッシュマネージャーの実装
- 単体テストの作成

### 8.2 フェーズ 2: S3 連携（2 週目）

- S3 クライアントの実装
- ファイル集約・圧縮処理の実装
- Specimen の即時アップロード実装
- Usage/Error の定期アップロード実装
- リトライ機構の実装

### 8.3 フェーズ 3: 運用機能（3 週目）

- グレースフルシャットダウンの実装
- ログ出力の実装
- エラーハンドリングの強化
- 結合テストの作成

### 8.4 フェーズ 4: CI/CD（4 週目）

- GitHub Actions ワークフローの作成
- GoReleaser の設定
- Docker イメージのビルド設定
- ドキュメントの整備

## 9. 非機能要件

### 9.1 パフォーマンス

- 大容量ファイル（10MB 以上）の処理に対応
- 並行リクエストの効率的な処理
- メモリ効率的なストリーミング処理

### 9.2 信頼性

- データ欠損を防ぐための段階的なファイル処理
- エラー時のリトライ機構
- プロセス異常終了時の復旧機能

### 9.3 運用性

- ヘルスチェックエンドポイント
- 詳細なログ出力
- 設定の外部化

### 9.4 セキュリティ

- 認証トークンによるアクセス制御（将来的にはより強固な認証へ移行）
- 適切なファイルパーミッション設定
- 入力値の検証とサニタイゼーション

## 10. 今後の拡張予定

- OAuth2/OIDC 対応による認証強化
- データ変換・フィルタリング機能
- Web 管理画面
- 統計情報 API
- アラート通知機能

## 11. 成功基準

- すべてのテストが正常に動作すること
- データ欠損が発生しないこと
- グレースフルシャットダウンが正しく機能すること
- CI/CD パイプラインが自動化されていること
- ドキュメントが整備されていること
