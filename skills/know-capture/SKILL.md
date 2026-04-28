---
name: know-capture
description: 从当前对话中自动提取有价值的知识并保存到知识库。适用于长对话结束时，或对话中产生了多条有价值信息时。触发词：提取知识、总结保存、capture、know capture、把这段对话的知识存一下、自动提取。
---
# 从对话自动提取知识

## 何时使用

- 长对话即将结束，对话中包含有价值的信息
- 用户说"把这段对话的知识存一下"
- 对话中产生了多条决策、规则、经验等
- debugging 过程解决了一个复杂问题

## 必须执行的步骤

### 1. 回顾对话并提取知识

浏览当前对话，按以下维度提取知识：

**主观知识**（个人经验/决策）：
- 技术选型理由（decision）
- 踩坑经验（pitfall）
- 调试过程中的关键发现（lesson）
- 个人偏好和工作习惯（preference）
- 可复用的操作流程——排查/部署/迁移的完整步骤（procedure）

**客观知识**（事实/规则）：
- 业务规则和流程（business）
- 架构设计和组件关系（architecture）
- 技术事实和配置（fact）
- 常见问答（faq）

### 2. 过滤低价值内容

**不要提取**：
- 标准 API 用法（文档已有的）
- 简单的代码生成（无特殊上下文）
- 闲聊内容
- 临时性信息（只对当前任务有用的）

### 3. 调用 MCP 工具保存

对每条提取的知识，调用 `mcp__personal-know__note_save`：

```
content: [知识内容，包含完整上下文和理由]
title: [简洁标题]
tags: [相关标签]
knowledge_type: [pitfall/decision/business/architecture/lesson/fact/faq/procedure]
source: conversation
```

如果对话内容很长（>2000字），可以改用 `mcp__personal-know__note_auto_capture`：

```
conversation: [对话的关键片段]
project_context: [当前项目上下文，如项目名、技术栈]
```

### 4. 向用户报告

提取完成后，向用户报告：
- 提取了几条知识
- 每条的标题和类型
- 提醒：新知识处于待审核状态，可以在 Web UI 审核

## 示例

对话中用户说：
> "这个 CORS 问题折腾了半天，最后发现是 nginx 和后端服务都设置了 CORS 头，导致浏览器收到双重 Access-Control-Allow-Origin 报错。解决方法是只在 nginx 层设置 CORS，后端服务去掉 CORS 中间件。"

提取为：
```
title: CORS 双重设置导致浏览器报错
content: 当 nginx 反向代理和后端服务同时设置了 CORS 头时，浏览器会收到双重 Access-Control-Allow-Origin 头，导致 CORS 校验失败。解决方法：只在一层设置 CORS，推荐在 nginx 层统一处理，后端服务去掉 CORS 中间件。排查耗时：约半天。
tags: CORS, nginx, 反向代理, 浏览器
knowledge_type: pitfall
```
