# 七牛文件管理（Qiniu File Manager）

基于 [NSGo](https://nsgo.dev/zh/) 与 [qiniu/go-sdk](https://github.com/qiniu/go-sdk) 构建的七牛云对象存储（Kodo）桌面管理工具。

- 安装显示名：**七牛OSS**（通过 `locales/en.yaml` 的 `appName` 设置）
- 窗口标题：七牛文件管理
- 包名 / Bundle ID：`com.maguo.qiniuoss`
- 产物文件名：`qiniuoss`（`nsgo.yaml` 的 `name`）
- 支持平台：NSGo 可打包的全部平台（macOS / Windows / Linux / iOS / Android / Web / TUI 等）

---

## 功能特性

### 账户与空间
- 多账户管理；AccessKey / SecretKey 仅保存在本机（应用存储）。
- 从 qshell 配置一键导入账户。
- 存储空间（Bucket）列表与切换；自动获取空间绑定的访问域名。
- **S3 兼容密钥**：可选配置 S3 AccessKey/SecretKey，在没有绑定域名时也能下载/预览（端点由七牛 `uc` 自动发现，可手动覆盖）。

### 文件浏览
- 目录 / 文件列表，列：名称、大小、类型、修改时间；按类型显示**彩色图标**。
- 单击选中（高亮），双击预览 / 进入目录；`Cmd/Ctrl/Shift+单击` 多选。
- 行首复选框 + 表头全选，支持批量操作。
- 面包屑路径、返回上一级、按名称快速过滤。
- 分页加载（“加载更多”）。

### 文件操作
- 上传：工具栏按钮或**拖拽文件**到列表；底部**上传列表**实时显示每个文件的等待/进度/完成/失败。
- 下载：单文件或批量（多选后选择目录）。
- 新建文件夹、重命名（含目录）。
- 复制到 / 移动到：目标空间 + 目标前缀**手动输入**，或从**目录树**中选取（懒加载）。
- 删除（目录递归删除，二次确认）。
- 复制链接（私有签名直链或 S3 预签名）、在浏览器打开。
- 文件详情（Key/大小/MIME/Hash/存储类型/上传时间/EndUser）。

### 预览
- 按类型预览：图片、视频、文本/代码、PDF（提示打开）、其它类型提示。
- **小于 10 MiB 的文件先下载到本地再预览**；大文件走远程流式。
- 预览取链接优先级：**S3 预签名**（若已配置 S3 密钥）→ 绑定域名的私有直链。

### 高级搜索（对齐 qshell `listbucket2`）
- 前缀、后缀、MIME、**存储类型下拉多选**、大小区间、起止时间、是否包含目录等过滤条件。
- 可开始 / 停止扫描，实时显示已扫描与匹配数量。
- 结果支持多选与全部文件操作；**导出**为制表符或逗号分隔（可选表头、可读大小）。

### 其它
- **多语言**：中文、English、日本語、한국어、Deutsch、Français、Português、Русский、Polski、فارسی、العربية；**默认跟随系统**，可在标题栏语言菜单切换（不做 RTL）。
- **自适应布局**：窄窗口时侧边栏收为抽屉，类型/时间列自动隐藏，操作栏自动换行。
- **调试面板**：记录切换目录/高级搜索时调用的 API 请求 URL 与原始响应，便于排查。
- 文件类型图标按类别着色（图片/视频/音频/PDF/文档/表格/演示/压缩/字体/代码/数据库/证书等）。

---

## 存储类型（存储级别）速查

高级搜索的「存储类型」对应文件存储级别代码：

| 值 | 含义 |
|---|---|
| 0 | 标准存储 |
| 1 | 低频存储 |
| 2 | 归档存储 |
| 3 | 深度归档存储 |
| 4 | 归档直读 |

---

## 环境与构建

要求：Go ≥ 1.27.3，安装 NSGo CLI。

```shell
# 安装 NSGo CLI（如未安装）
go install github.com/nsgo-dev/nsgo/cmd/nsgo@latest

# 查看本机可用目标
nsgo doctor

# 开发运行（热重载）
nsgo dev

# 运行单元测试
go test ./...

# 打包（示例：macOS arm64）
nsgo build -target macos_arm64 -output dist
```

Windows / Linux / iOS / Android / Web / TUI 等目标同理，`-target` 传入对应名称。

---

## 使用说明

1. 打开应用 → 侧栏「账户」→ 填写名称、AccessKey、SecretKey，保存。
   - 也可点「从 qshell 导入」读取 `~/.qshell/account.json`。
2. 选择存储空间开始浏览。
3. **没有绑定域名时**：展开「账户」填入七牛控制台创建的 **S3 密钥**，即可下载/预览（链接为 S3 预签名，默认有效期 1 小时）。
4. 预览取链接优先使用 S3（若已配置），否则使用绑定域名的私有直链。

> 私有空间的临时链接依赖签名；S3 预签名链接有有效期，过期需重新生成。

### macOS 首次打开（去隔离）
本应用未做 Apple 公证（notarization），从 DMG 安装后首次打开可能提示“已损坏 / 无法验证开发者”。把应用拖入「应用程序」后，执行以下命令移除隔离属性即可：

```shell
# -d 后是扩展属性名（隔离属性固定为 com.apple.quarantine）
# 路径是应用 .app 的实际文件名；本应用由 name: qiniuoss 生成 qiniuoss.app
sudo xattr -r -d com.apple.quarantine /Applications/qiniuoss.app
```

> `com.maguo.qiniuoss` 是应用的 **Bundle ID**，不是扩展属性名，不能放在 `-d` 后面。若安装后的 `.app` 文件名为 `com.maguo.qiniuoss.app`，才把路径改为 `/Applications/com.maguo.qiniuoss.app`。


---

## 目录结构

```
.
├── app.go          # 入口、根布局、侧栏、标题栏、调试面板、账户编辑
├── views.go        # 浏览/搜索视图、文件列表、上传列表、响应式布局
├── dialogs.go      # 各类对话框（账户、上传、重命名、复制移动、预览、详情、域名、删除）
├── state.go        # 应用状态、选择/光标、上传队列、搜索扫描、目录树、S3 取链
├── model.go        # 数据模型、类型识别、过滤与导出、时间/大小解析
├── qiniu.go        # 七牛 SDK 封装（列桶/列文件/增删改/上传/下载）
├── s3.go           # S3 兼容端点发现 + AWS SigV4 预签名、管理 API 原始调用（调试）
├── i18n.go         # 多语言目录与翻译
├── nsgo.yaml       # 应用名 / id / 目标平台（id: com.maguo.qiniuoss）
└── assets/         # 应用图标（Icon.png / AppIcon.icns，七牛 logo）
```

---

## 技术栈

- Go 1.27
- [NSGo](https://nsgo.dev/zh/)：跨平台 UI（`core` / `ui` / `lucide` 图标）
- [qiniu/go-sdk/v7](https://github.com/qiniu/go-sdk)：七牛存储 API
- 自定义 AWS Signature V4 预签名，实现无域名时的 S3 兼容下载

---

## 备注

- 七牛已不再为新空间分配测试域名；无域名空间请使用 **S3 密钥**方案或自行绑定域名。
- 应用图标使用七牛 logo。
- 密钥仅保存在本机应用存储中，请勿提交到版本库。
