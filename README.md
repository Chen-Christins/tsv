# tsv

Go + Bubble Tea 实现的 tig 风格 SVN 终端浏览器，支持 Windows/Linux。

## 界面

启动后显示一个统一列表：第一行是虚拟的 `Not Committed Yet o Unstaged changes`，后面是从新到旧的提交历史（日期、时区、作者、版本号、提交说明）。工作副本干净时仍保留第一行和历史记录。

移动选择不会自动打开详情。按 Enter 打开右侧分屏：

- 第一行：整个工作副本的状态和本地 diff；未纳管文件只列出路径。
- 历史行：版本、作者、日期、完整提交说明、变更路径、对应版本的 diff。

从子目录启动会自动定位工作副本根目录，历史范围是该根目录对应的仓库路径。历史每次加载 100 条，接近列表底部时继续加载更早的记录；日志按 HEAD 查询，diff 使用所选版本的仓库 URL，不受本地 BASE 版本限制。

## 运行

### 安装为 tsv 命令（Windows）

在项目目录运行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\install.ps1
```

安装到 `%LOCALAPPDATA%\tsv\tsv.exe` 并加入当前用户 PATH，无需管理员权限。重新打开终端后，在工作副本目录执行 `tsv`，或执行 `tsv "C:\project"`。如果 IDE 内置终端仍找不到命令，重启 IDE 以刷新环境变量。更新程序后重新执行安装脚本即可。

Linux 可将构建产物安装到用户命令目录：

```sh
mkdir -p ~/.local/bin
install -m 755 tsv-linux-amd64 ~/.local/bin/tsv
```

确保 `~/.local/bin` 已加入 PATH 后即可运行 `tsv`。

运行环境需要 `svn` 在 PATH 中，以及至少 60 列 × 12 行的交互终端。

Windows：

```powershell
.\tsv-windows-amd64.exe "C:\project"
```

Linux：

```sh
chmod +x tsv-linux-amd64
./tsv-linux-amd64 /path/to/working-copy
```

不传路径时使用当前目录。历史和提交 diff 需要能访问 SVN 仓库；请先使用 SVN 客户端配置认证。命令非交互执行，超时 30 秒，失败会显示错误；diff 成功时产生的 stderr 警告也显示在详情中。

| 快捷键 | 行为 |
| --- | --- |
| ↑↓ / j、k | 移动列表或滚动详情 |
| Enter | 打开选中行详情 |
| Tab | 切换列表/详情焦点 |
| H | 定位最新项目提交 `[HEAD]` |
| B | 定位本地根目录 BASE 对应的项目提交，必要时自动加载历史 |
| V | 按需检查混合版本范围（扫描工作副本） |
| PgUp / PgDn | 翻页 |
| Home / End、g / G | 跳到已加载列表首行/末行 |
| r | 刷新工作副本与历史，关闭详情 |
| q / Esc | 关闭详情；主列表中退出 |
| Ctrl+C | 直接退出 |

SVN 没有 Git 暂存区，`Unstaged changes` 在此表示 SVN 未提交修改。界面不展示 Git 分支图或 Git 引用标签。当前版本只读；大型 diff 保存在内存中。

`[HEAD]` 标记当前项目路径的最新提交，并不代表全仓库最新版本号。`[ROOT BASE]` 从本地 `svn info` 的根目录最后变更版本取得，无需递归扫描或额外远程查询；状态栏保留实际根目录 BASE 版本号。按 V 才扫描版本化节点并显示 `mixed rN:rM`。版本范围不包含外部工作副本；稀疏工作副本只统计本地存在的节点。

启动先读取本地根目录位置，再异步读取不含变更路径的轻量历史。Enter 时按需读取完整路径和 diff；最多缓存 20 个已打开的提交详情，刷新时清空缓存。本地修改不缓存，以免显示旧结果。历史首次加载仍需要仓库网络请求，当前没有跨进程历史缓存。

## 开发与构建

需要 Go 1.25+。

```sh
go test ./...
go vet ./...
go run ./cmd/tsv /path/to/working-copy
```

测试包含本地 SVN 仓库集成测试，需要 `svn` 和 `svnadmin`，不访问网络。

PowerShell 交叉编译：

```powershell
$env:CGO_ENABLED="0"
$env:GOOS="windows"
$env:GOARCH="amd64"
go build -o tsv-windows-amd64.exe ./cmd/tsv
$env:GOOS="linux"
go build -o tsv-linux-amd64 ./cmd/tsv
```

目标环境无需 Go 运行时，但仍需 SVN 客户端及其依赖。Linux 终端交互需在目标环境实测。
