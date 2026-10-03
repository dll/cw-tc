import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

/// 首页（PRD v2 §8.1 `/`）。
///
/// M0 为骨架：展示产品定位与到课程广场的入口。
/// M1 起补：按专业找课、验真入口、价格展示。
class HomePage extends StatelessWidget {
  const HomePage({super.key});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('第三课堂')),
      body: Center(
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 560),
          child: Padding(
            padding: const EdgeInsets.all(24),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: <Widget>[
                Text('以 AI 为引擎的自主学习与能力认证平台',
                    style: theme.textTheme.titleMedium),
                const SizedBox(height: 8),
                Text(
                  '畅网智能科技 · 手机号注册 · 付费解锁 · AI 陪学 · 分级测评 · 企业发证',
                  style: theme.textTheme.bodySmall,
                ),
                const SizedBox(height: 32),
                FilledButton(
                  onPressed: () => context.go('/courses'),
                  child: const Text('进入课程广场'),
                ),
                const SizedBox(height: 12),
                OutlinedButton(
                  onPressed: () => context.go('/verify'),
                  child: const Text('证书验真'),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
