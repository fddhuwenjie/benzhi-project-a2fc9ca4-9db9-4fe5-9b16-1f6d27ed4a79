# 图像完整性审查工作台

本项目面向科研期刊图像审查员、责任编辑和论文作者，提供从稿件图像建档、草稿修订与预检、确定性完整性核验、人工批量判读、多轮作者回应、复核终审到冻结归档的完整工作流。服务同时提供响应式浏览器页面和 JSON HTTP API，所有变更均使用 `revision` 乐观并发控制，以 `request_id` 保证重复提交幂等，并写入可恢复的 JSONL 审计事件。

## 构建

项目仅依赖 Go 标准库，要求 Go 1.23 或兼容版本：

```bash
go build ./cmd/server
```

## 运行

默认监听高位回环地址 `127.0.0.1:19081`，持久化数据默认写入系统临时目录下的 `image-integrity-review`：

```bash
go run ./cmd/server
```

可通过参数设置回环监听地址和数据目录：

```bash
go run ./cmd/server -addr=127.0.0.1:19120 -data-dir=./review-data
```

也可设置 `PORT` 为端口号，服务会绑定 `127.0.0.1:<PORT>`。设置 `IMAGE_REVIEW_DATA` 可指定持久化目录。启动后访问 `http://127.0.0.1:19081/` 使用浏览器工作台。

## 自检与测试

有界自检会在指定地址实际启动 HTTP 服务，通过公开接口完成建档至归档的整条流程，随后关闭并退出：

```bash
go run ./cmd/server -self-check -addr=127.0.0.1:19081
```

运行全部自动化测试：

```bash
go test ./...
```

## 数据与审计

持久化目录包含 `events.jsonl`、`snapshots/` 和 `archives/`。写操作先追加并同步审计事件，再使用临时文件原子替换案件快照；事件载荷保留当次聚合结果，可在快照缺失时恢复。归档文件是只读 JSON 文档，包含稿件信息、规则结果、判读、全部回应轮次证据、终审决定和时间线，并可按案件 ID 或稿件编号校验摘要。

## 队列与新增接口

`GET /api/cases` 支持 `status`、`journal_section`、`assignee_id`、`q`、`severity`、`open_only`、`page` 和 `page_size` 查询参数。响应中的 `summary` 基于分页前的筛选全集，`pagination` 给出页码、容量、命中总数和总页数。

草稿状态可通过 `PUT /api/cases/{id}/figures` 整批修订图像清单并取得固定规则预检结果；判读中案件可通过 `POST /api/cases/{id}/verdicts/batch` 原子保存多个风险项。作者回应和复核请求携带 `round_number`，已关闭轮次只读并随最终 JSON 档案一并保存。
