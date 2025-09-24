## バリデーションの統一

CLI (cmd) と MCP サーバ (mcp) で重複していたユーザー名・チームスラグの検証を、共有パッケージ `validate` に集約しました。

- パッケージ: `ghub-desk/validate`
- 目的: 単一点管理による保守性向上と挙動の一貫性確保

### 提供 API
- 定数
  - `UserNamePattern`, `TeamSlugPattern`
  - `UserNameMin`, `UserNameMax`, `TeamSlugMin`, `TeamSlugMax`
- 関数
  - `ValidateUserName(string) error`
  - `ValidateTeamSlug(string) error`
  - `ParseTeamUserPair(string) (team, user string, err error)`
- エラー
  - `ErrInvalidUserName`, `ErrInvalidTeamSlug`, `ErrInvalidPair`

呼び出し側では、上記の関数を利用し、ユーザー向けメッセージ（日本語/英語）は各層で適切にラップしてください。

MCP の JSON Schema で `Pattern` を指定する場合は、`validate.TeamSlugPattern` を使用してコード/Schema の整合性を保ちます。

