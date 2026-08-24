# BENZHI_README

## 项目说明
- 项目：benzhi-project-a2fc9ca4-9db9-4fe5-9b16-1f6d27ed4a79
- 项目用途：已完整实现科研论文图像完整性审查工作台，覆盖稿件建档、确定性规则核验、审查判读、作者回应、复核终审、冻结归档、事件恢复与浏览器全流程。
- Go 工具链：`golang:1.23`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：image-integrity-review
- 项目概述：一个面向科研出版质量人员的论文图像完整性审查工作台，将稿件图像从审查建档、自动核验、人工判读、作者回应推进到终审归档，并保留可复核的状态轨迹与证据链。
- 核心工作流：责任编辑创建稿件审查案并登记图像与来源说明，系统执行完整性规则核验并生成风险项，审查员逐项判读后要求作者回应，作者提交说明或替换图证据，审查员复核并由责任编辑作出通过或退回决定，最终形成只读审查档案。
- 对外接口：由 Go 服务提供原生 HTML、CSS 和 JavaScript 的浏览器工作台，包含案件队列、图像清单、风险项判读、作者回应和终审档案页面；HTTP 服务支持 `-addr=127.0.0.1:<port>`，默认监听 `127.0.0.1:19081`，不得默认绑定 `8080`、`80`、`3000` 或 `0.0.0.0`，根目录 `README.md` 使用简体中文说明用途、标准构建、运行和测试方式。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -self-check -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-a2fc9ca4-9db9-4fe5-9b16-1f6d27ed4a79-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-a2fc9ca4-9db9-4fe5-9b16-1f6d27ed4a79-arm64 linux/arm64
docker run -it benzhi-project-a2fc9ca4-9db9-4fe5-9b16-1f6d27ed4a79-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -self-check -addr=127.0.0.1:19081`
