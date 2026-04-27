---
name: know-review
description: 审核待审知识，查看待审核队列并逐条审批。知识库采用类 Git 模型，只有审核通过的知识才会被搜索召回。触发词：审核知识、review、know review、待审核、通过知识、拒绝知识。
---
# 审核知识库中的待审知识

## 何时使用

- 用户说"审核一下知识"、"看看待审核的"
- 保存知识后提醒用户需要审核
- 定期维护知识库质量

## 知识审核模型

知识库采用**类 Git 分支模型**：
- 新保存的知识进入 `pending`（暂存区）
- 人工审核后变为 `approved`（主分支，可被搜索召回）或 `rejected`（拒绝）
- 只有 `approved` 状态的知识会被 MCP 搜索返回

## 必须执行的步骤

### 1. 查看待审队列

调用 `mcp__personal-know__note_review`：

```
action: list
limit: 10
```

### 2. 逐条展示待审知识

将待审知识展示给用户，包含：
- 标题、类型、来源
- 内容摘要（前200字）
- LLM 建议（如果有）
- 标签

### 3. 等待用户决定

对每条知识，用户可以：
- **通过** → 调用 `note_review` action=approve
- **拒绝** → 调用 `note_review` action=reject，附带原因
- **修改后通过** → 先调用 `note_update` 修改内容，再 approve
- **跳过** → 不处理，保持 pending

### 4. 执行审核操作

根据用户决定调用 `mcp__personal-know__note_review`：

通过：
```
action: approve
id: [知识 ID]
reason: [可选，通过原因]
```

拒绝：
```
action: reject
id: [知识 ID]
reason: [拒绝原因]
```

修改后通过：先更新内容
```
mcp__personal-know__note_update:
  id: [知识 ID]
  content: [修改后的内容]
  title: [修改后的标题]

然后:
mcp__personal-know__note_review:
  action: approve
  id: [知识 ID]
```

### 5. 报告审核结果

告诉用户：
- 审核了几条
- 通过了几条，拒绝了几条
- 还剩多少条待审核

## 审核标准建议

展示给用户参考：
1. **正确性**：内容是否准确？有没有过时的信息？
2. **完整性**：独立阅读能否理解？有没有缺少关键上下文？
3. **价值**：值得长期保存吗？还是只对当时有用？
4. **不重复**：是否和已有知识高度重复？
