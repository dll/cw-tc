import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../api/api_client.dart';
import '../models/course.dart';
import '../models/lecture.dart';

/// 课程详情（PRD v2 §7.2 B-2）：基本信息 + 讲次列表。
///
/// 课程信息与讲次并行拉取；未解锁课程只看到试学讲次（`is_preview`），
/// 学习跳转由后续「学习页」承载（本页仅展示大纲与试学标记）。
class CourseDetailPage extends StatefulWidget {
  const CourseDetailPage({super.key, required this.courseCode, this.api});

  final String courseCode;
  final ApiClient? api;

  @override
  State<CourseDetailPage> createState() => _CourseDetailPageState();
}

class _CourseDetailPageState extends State<CourseDetailPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  bool _entitled = false;
  bool _buying = false;

  @override
  void initState() {
    super.initState();
    _checkEntitlement();
  }

  Future<void> _checkEntitlement() async {
    try {
      final codes = await _api.myEntitlementCourseCodes();
      if (!mounted) return;
      setState(() => _entitled = codes.contains(widget.courseCode));
    } catch (_) {
      // 未登录/查询失败按未购处理。
    }
  }

  Future<void> _buy(Course course) async {
    setState(() => _buying = true);
    try {
      final orderNo = await _api.createOrder(widget.courseCode);
      if (orderNo.isEmpty) throw Exception('下单失败');
      await _api.mockPayOrder(orderNo);
      if (!mounted) return;
      setState(() => _entitled = true);
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('购买成功，开始学习吧')),
      );
    } catch (_) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('购买失败，请稍后重试')),
      );
    } finally {
      if (mounted) setState(() => _buying = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(title: Text(widget.courseCode)),
      body: FutureBuilder<Course>(
        future: _api.courseDetail(widget.courseCode),
        builder: (BuildContext context, AsyncSnapshot<Course> snap) {
          if (snap.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snap.hasError || !snap.hasData) {
            return const Center(child: Text('课程加载失败'));
          }
          final course = snap.data!;
          return ListView(
            padding: const EdgeInsets.all(16),
            children: <Widget>[
              Text(course.name, style: theme.textTheme.headlineSmall),
              if (course.intro != null && course.intro!.isNotEmpty) ...<Widget>[
                const SizedBox(height: 8),
                Text(course.intro!, style: theme.textTheme.bodyMedium),
              ],
              const SizedBox(height: 12),
              Row(
                children: <Widget>[
                  Text(course.priceLabel, style: theme.textTheme.titleLarge?.copyWith(color: theme.colorScheme.primary)),
                  const Spacer(),
                  if (_entitled || course.priceCents <= 0) ...<Widget>[
                    OutlinedButton(
                      onPressed: () => context.push('/exam/${widget.courseCode}/L1'),
                      child: const Text('测评'),
                    ),
                    const SizedBox(width: 8),
                    FilledButton(
                      onPressed: () => context.push('/learn/${widget.courseCode}/1'),
                      child: const Text('开始学习'),
                    ),
                  ] else ...<Widget>[
                    FilledButton(
                      onPressed: _buying ? null : () => _buy(course),
                      child: Text(_buying ? '购买中…' : '购买'),
                    ),
                  ],
                ],
              ),
              const Divider(height: 32),
              Text('讲次大纲', style: theme.textTheme.titleMedium),
              const SizedBox(height: 8),
              FutureBuilder<List<Lecture>>(
                future: _api.lectures(widget.courseCode),
                builder: (BuildContext context, AsyncSnapshot<List<Lecture>> ls) {
                  if (ls.connectionState == ConnectionState.waiting) {
                    return const Padding(
                      padding: EdgeInsets.all(24),
                      child: Center(child: CircularProgressIndicator()),
                    );
                  }
                  final lectures = ls.data ?? const <Lecture>[];
                  if (lectures.isEmpty) {
                    return const Padding(
                      padding: EdgeInsets.all(24),
                      child: Center(child: Text('暂无讲次')),
                    );
                  }
                  return Column(
                    children: lectures.map((lecture) {
                      return ListTile(
                        dense: true,
                        leading: CircleAvatar(radius: 14, child: Text('${lecture.no}')),
                        title: Text(lecture.title),
                        trailing: lecture.isPreview ? const Chip(label: Text('试学')) : null,
                        onTap: () => context.push('/learn/${widget.courseCode}/${lecture.no}'),
                      );
                    }).toList(),
                  );
                },
              ),
            ],
          );
        },
      ),
    );
  }
}
