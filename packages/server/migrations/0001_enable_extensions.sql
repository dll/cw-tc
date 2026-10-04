-- 0001_enable_extensions.sql
-- 第三课堂 · 迁移 0001：启用扩展（数据字典 §8 / §1）
--
-- 幂等：重跑安全。CI 与生产发布流水线都可能重复执行。

-- pgvector：知识库语义检索（PRD §7.5 H-2 / §11.2）。
-- 依赖它为 0007_kb 的 kb_chunk.embedding 列提供 vector 类型。
CREATE EXTENSION IF NOT EXISTS vector;

-- 说明：
--   · 不在这里建任何业务表 —— 业务表按数据字典 §8 的切分在 0002+ 逐批落地；
--   · 不创建任何角色/密码 —— 库与账号由部署环节创建（deploy/README.md）；
--   · 不写入任何种子数据 —— 种子在各自迁移里用 upsert，保证重跑结果一致。
