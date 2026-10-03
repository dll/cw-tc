import 'package:flutter/material.dart';

/// 课程广场（PRD v2 §7.2 B-1）。
///
/// M1 落地要点（此处先立"待接入"的显式提示，避免把空列表当成"没有课程"）：
///   - 课程内容来自上游 `课程总览.json`（只读同步，不写死）；
///   - **专业分类、价格、上架状态来自平台自有元数据**（PRD v2 §8.4 边界表）；
///   - 列表按 `status=已上架` 过滤。
class CourseSquarePage extends StatelessWidget {
  const CourseSquarePage({super.key});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: const Text('课程广场')),
      body: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Text('按专业筛选 / 主线辅线 / 价格展示', style: theme.textTheme.titleSmall),
            const SizedBox(height: 12),
            // 不编数：M0 尚未接入数据源，因此不显示 "共 0 门课" 之类会被误读的数字。
            const Card(
              child: Padding(
                padding: EdgeInsets.all(16),
                child: Text('课程列表尚未接入数据源（M1）'),
              ),
            ),
          ],
        ),
      ),
    );
  }
}
