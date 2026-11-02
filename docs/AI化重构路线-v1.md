# AI 化重构路线 v1.0

> 决策：**渐进式重构**（不推倒重来）· **全链路 AI 化** · **保持 API / 数据库兼容**。
> 每个阶段独立可上线、可回滚；业务只依赖别名与稳定契约，不直接依赖厂商 SDK。

## 1. 现状与目标

已有能力：课程/学习/测评/证书/支付/讨论/离线/统计，以及 `/ai/ask` 的课程内问答。
目标：把 AI 从“一个问答接口”变成**贯穿学员、教师、管理员的基础设施**，同时满足
PRD §7.5 / §7.16 / §9.8 与 AGENTS 铁律 12/14。

## 2. 分层（从上到下）

```
业务能力  学员陪学 · 教师备课/批改 · 管理员运营与风控
   ↓
AI 服务   aiassist · aicontent · aigrading · aiinsight
   ↓
AI 网关   aigateway（别名路由 / 备案留痕 / 超时与错误归一）
   ↓
检索层   课程知识库（切片 / 向量 / 引用）· 冻结输出
   ↓
供应商   百炼 / DeepSeek / 方舟（OpenAI 兼容协议，可替换）
```

**硬约束**

- 业务只认 `assistant.fast` / `assistant.strong`；换模型 = 改配置 + 跑回归问答集，**不改业务代码**。
- 每次调用必须落 `model / model_version / filing_no`；未备案 / 缺备案号 **fail-closed**。
- 回答必须有课程内出处；无依据明说，越界拒答，测评期间关闭。
- 生成类资产必须带显式 + 隐式 AIGC 标识，且走双审（教师一审 → 管理员二审）。

## 3. 阶段计划

| 阶段 | 内容 | 交付物 | 验收 |
|---|---|---|---|
| **P0 网关** | 别名路由、备案留痕、OpenAI 兼容 Provider、超时/错误归一 | `internal/aigateway`；`/ai/ask` 改走网关 | 未备案拒绝；调用留痕齐全；换模型不改业务 |
| P1 知识库 | 课程切片、引用、SQLite 暴力检索 / PG pgvector | `internal/knowledge`；`/teacher/courses/{code}/kb/rebuild` 等 | 回答带引用率 100%；不含证书 / `_ai_draft` |
| P2 教师生成 | 建议聚类 → 草稿 → 自动体检 → 双审 → 原子插入 | `internal/aicontent`；教师/管理员审核队列 | 单审不可上架；引用缺失拒绝；章级原子插入 |
| P3 管理员 AI | 课程质量体检、评论/讨论聚类、异常与退款摘要 | `internal/aiinsight`；`/admin/insights` 增强 | 每条建议可溯源；证据缺项为 0 |
| P4 学员 AI | 学习路径、错题诊断、复习计划（基于已有成绩与进度） | `internal/ailearn`；`/me/learning-plan` | 不编造数据；可解释并可回到讲次 |
| P5 辅助批改 | 主观题采点建议（人是最终判定） | `internal/aigrading` | AI 只建议不判负；教师可改；留痕 |

## 4. 兼容策略

- 接口：**只增不删**；旧字段保留，新增字段可忽略。
- 数据库：迁移**只增列 / 增表**，不破坏既有数据；SQLite 与 PostgreSQL 双写迁移同步。
- 开关：AI 能力全部 **fail-closed**；未配置密钥 / 备案号时返回明确不可用，不静默降级。
- 可回滚：每阶段独立开关；关闭后原有非 AI 功能不受影响。

## 5. 质量门禁

```bash
go test ./internal/aigateway/... -run 'TestFilingTraceability|TestUnregisteredModelRejected'
go test ./internal/aicontent/... -run 'TestDoubleReview|TestGroundedGeneration|TestChapterAtomicInsert'
go run ./cmd/insightcheck --all          # 每条 AI 建议可溯源
go run ./cmd/aicheck --sample 50         # 回答带引用率 100%
```

## 6. 当前进度

- ✅ P0：`internal/aigateway` 已建立；`/ai/ask` 已从直连 DeepSeek 改为**别名 + 网关**。
- ✅ P5 起步：教师主观题 **AI 阅卷建议**（只建议不判负，留痕 model / filing_no）。`n- ✅ P1 起步：`knowledge_chunk` + 管理员重建 / 状态；`/ai/ask` 优先用知识库切片，未构建时回落文件扫描。`n- ✅ P4 起步：`/me/learning-plan` AI 学习路径（基于进度/成绩/知识库引用，不承诺通过）。`n- ✅ P2 起步：教师 AI 草稿 → 自动体检 → 教师一审 → 管理员二审 → 章级原子上架；双审与 AIGC 标识硬校验。`n- ✅ P3 起步：管理员 AI 运营报告（五段结构 + 每条建议 evidence_map 溯源，不可溯源即拒绝保存）。`n- ✅ P3 教师侧：AI 周报结论（采纳/驳回）+ 改版计划（依据/措施/目标指标必填，放弃须原因）+ 版本前后对比。`n- ✅ AI 增补效果对比：按版本发布时间切分前后 30 天统计；上架自动写 `course_revision`。`n- ✅ AIGC 导出闭环：发布写 Markdown 元数据（AIGC_label / provider / content_sha256），导出接口带隐式标识；未写标识拒绝导出。`n- ⏳ 后续：更多运营信号与效果回流。