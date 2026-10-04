# tangboys 分支发布流程

每次发布新版本，同时更新 `CHANGELOG.md`、`Makefile` 的默认版本和
`changelogs/<版本标签>.md`。发布说明按原项目的格式写明更新内容、升级步骤、
验证结果、已知限制及完整差异，尤其记录本分支相对上游的改动。

版本标签采用 `v<主版本>.<次版本>.<修订号>-tangboys.<序号>`，例如
`v1.5.3-tangboys.4`。完成该提交的检查后推送提交及标签：

```bash
git push origin main
git tag v1.5.3-tangboys.4
git push origin v1.5.3-tangboys.4
```

`.github/workflows/release.yml` 自动构建六个平台包，执行 Windows UTF-8 回归测试，
生成 SHA-256 校验文件，并用同名 `changelogs` 文件发布 GitHub 预发布版。
Windows 包带已编译的 UTF-8 包装程序，安装时无需 Go；所有包带配置示例与 Codex 插件。
打包使用 `tools/package-release.ps1`，只选取公开的程序和安装文件。

后续大版本沿用此流程，使用该版本自己的标签和发布说明。工作流可从 Actions
页面手动运行并指定已存在的标签；仅在尚未发布该版本时使用，已发布版本不覆盖。
完成针对确切标签的完整测试和实际平台验收后，可在 Releases 页面编辑该版本，
移除预发布标记。
