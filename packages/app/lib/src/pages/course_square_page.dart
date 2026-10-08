import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../models/course.dart';

/// 课程广场（PRD v2 §7.2 B-1）。
///
/// 接入 `GET /api/v1/courses`（公开只读，按 `status=已上架` 过滤）：
/// 加载中 / 拉取失败 / 空态 / 列表 四种状态如实呈现，不编造数据。
class CourseSquarePage extends StatefulWidget {
  const CourseSquarePage({super.key, this.api});

  final ApiClient? api;

  @override
  State<CourseSquarePage> createState() => _CourseSquarePageState();
}

class _CourseSquarePageState extends State<CourseSquarePage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<List<Course>> _future;

  @override
  void initState() {
    super.initState();
    _future = _api.courses();
  }

  Future<void> _reload() async {
    setState(() => _future = _api.courses());
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('课程广场')),
      body: FutureBuilder<List<Course>>(
        future: _future,
        builder: (BuildContext context, AsyncSnapshot<List<Course>> snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return _ErrorState(onRetry: _reload, message: '课程列表加载失败');
          }
          final courses = snapshot.data ?? const <Course>[];
          if (courses.isEmpty) {
            return _EmptyState(onRetry: _reload);
          }
          return ListView.separated(
            padding: const EdgeInsets.all(12),
            itemCount: courses.length,
            separatorBuilder: (_, __) => const SizedBox(height: 8),
            itemBuilder: (BuildContext context, int index) {
              final course = courses[index];
              return _CourseCard(course: course);
            },
          );
        },
      ),
    );
  }
}

class _CourseCard extends StatelessWidget {
  const _CourseCard({required this.course});

  final Course course;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: EdgeInsets.zero,
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => context.push('/courses/${course.courseCode}'),
        child: Padding(
          padding: const EdgeInsets.all(14),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: <Widget>[
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Text(course.name, style: theme.textTheme.titleMedium),
                    if (course.intro != null && course.intro!.isNotEmpty) ...<Widget>[
                      const SizedBox(height: 4),
                      Text(
                        course.intro!,
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                        style: theme.textTheme.bodySmall,
                      ),
                    ],
                  ],
                ),
              ),
              const SizedBox(width: 12),
              Text(
                course.priceLabel,
                style: theme.textTheme.titleMedium?.copyWith(color: theme.colorScheme.primary),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _EmptyState extends StatelessWidget {
  const _EmptyState({required this.onRetry});

  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(Icons.inbox_outlined, size: 56, color: theme.disabledColor),
            const SizedBox(height: 12),
            Text('暂无已上架课程', style: theme.textTheme.titleMedium),
            const SizedBox(height: 6),
            Text('课程上架后会出现在这里。', style: theme.textTheme.bodySmall),
            const SizedBox(height: 12),
            TextButton.icon(onPressed: onRetry, icon: const Icon(Icons.refresh), label: const Text('刷新')),
          ],
        ),
      ),
    );
  }
}

class _ErrorState extends StatelessWidget {
  const _ErrorState({required this.onRetry, required this.message});

  final VoidCallback onRetry;
  final String message;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: <Widget>[
            Icon(Icons.error_outline, size: 56, color: theme.colorScheme.error),
            const SizedBox(height: 12),
            Text(message, style: theme.textTheme.titleMedium),
            const SizedBox(height: 12),
            TextButton.icon(onPressed: onRetry, icon: const Icon(Icons.refresh), label: const Text('重试')),
          ],
        ),
      ),
    );
  }
}
