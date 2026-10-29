import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../models/operation_slot.dart';

/// 首页（PRD v2 §8.1 `/`）：产品定位 + 运营位 Banner + 课程广场 / 证书验真入口。
class HomePage extends StatefulWidget {
  const HomePage({super.key, this.api});

  final ApiClient? api;

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  List<OperationSlot> _banners = const <OperationSlot>[];

  @override
  void initState() {
    super.initState();
    _loadBanners();
  }

  Future<void> _loadBanners() async {
    try {
      final items = await _api.operations('home_banner');
      if (!mounted) return;
      setState(() => _banners = items);
    } catch (_) {
      // 运营位失败不影响首页其他入口。
    }
  }

  void _open(OperationSlot slot) {
    if (slot.courseCode.isNotEmpty) {
      context.push('/courses/${slot.courseCode}');
      return;
    }
    if (slot.linkUrl.startsWith('/')) {
      context.push(slot.linkUrl);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('第三课堂')),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(24),
          children: <Widget>[
            if (_banners.isNotEmpty) ...<Widget>[
              for (final slot in _banners) _bannerCard(theme, slot),
              const SizedBox(height: 16),
            ],
            Text('以 AI 为引擎的在线自主学习与能力认证平台',
                style: theme.textTheme.titleMedium),
            const SizedBox(height: 8),
            Text(
              '畅网智能科技 · 手机号注册 → 实名认证 → AI 陪学 → 分级测评 → 企业能力认证',
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
    );
  }

  Widget _bannerCard(ThemeData theme, OperationSlot slot) {
    final hasLink = slot.courseCode.isNotEmpty || slot.linkUrl.startsWith('/');
    final image = slot.imageUrl.isEmpty
        ? null
        : Image.network(
            _api.mediaUrl(slot.imageUrl),
            height: 140,
            width: double.infinity,
            fit: BoxFit.cover,
            errorBuilder: (_, __, ___) => const SizedBox.shrink(),
          );
    return Card(
      clipBehavior: Clip.antiAlias,
      margin: const EdgeInsets.only(bottom: 12),
      child: InkWell(
        onTap: hasLink ? () => _open(slot) : null,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: <Widget>[
            if (image != null) image,
            Padding(
              padding: const EdgeInsets.all(12),
              child: Text(
                slot.title.isEmpty ? '推荐内容' : slot.title,
                style: theme.textTheme.titleSmall,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
