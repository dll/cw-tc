import 'package:flutter/material.dart';

/// 通用占位页。
///
/// 用途：把 PRD v2 §8.1 的站点地图先固化在路由层，
/// 让"哪个路径该存在"在 M0 就确定下来，避免 M1 各人各自造路径。
class PlaceholderPage extends StatelessWidget {
  const PlaceholderPage({super.key, required this.title, required this.detail});

  final String title;
  final String detail;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: Text(title)),
      body: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: <Widget>[
            Text(detail, style: theme.textTheme.bodyLarge),
            const SizedBox(height: 8),
            Text('待实现', style: theme.textTheme.bodySmall),
          ],
        ),
      ),
    );
  }
}
