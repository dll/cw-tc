import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:qr_flutter/qr_flutter.dart';

import '../api/api_client.dart';
import '../models/course.dart';
import '../models/course_comment.dart';
import '../models/lecture.dart';
import 'lecture_discussion_page.dart';

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
  bool _freeGranted = false;
  bool _buying = false;
  bool _favorited = false;
  Future<CourseCommentPage>? _comments;

  @override
  void initState() {
    super.initState();
    _checkEntitlement();
    _loadFavorite();
    _comments = _api.courseComments(widget.courseCode);
  }

  Future<void> _loadFavorite() async {
    try {
      final value = await _api.favoriteStatus(widget.courseCode);
      if (mounted) setState(() => _favorited = value);
    } catch (_) {}
  }

  Future<void> _toggleFavorite() async {
    final next = !_favorited;
    setState(() => _favorited = next);
    try {
      await _api.setFavorite(widget.courseCode, next);
    } catch (_) {
      if (!mounted) return;
      setState(() => _favorited = !next);
      ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('收藏操作失败，请稍后重试')));
    }
  }

  Future<void> _checkEntitlement() async {
    try {
      final sources = await _api.myEntitlementSources();
      if (!mounted) return;
      final source = sources[widget.courseCode];
      setState(() {
        _entitled = source != null;
        // 管理员授权免费（grant）或免费权益（free）→ 界面显示「免费」。
        _freeGranted = source == 'grant' || source == 'free';
      });
    } catch (_) {
      // 未登录/查询失败按未购处理。
    }
  }

  Future<void> _buy(Course course) async {
    setState(() => _buying = true);
    try {
      final orderNo = await _api.createOrder(widget.courseCode);
      if (orderNo.isEmpty) throw Exception('下单失败');
      final qrCode = await _api.alipayPrecreate(orderNo);
      if (qrCode.isEmpty) throw Exception('支付宝二维码获取失败');
      if (!mounted) return;
      await showDialog<void>(context: context, builder: (dialogContext) {
        return AlertDialog(
          title: const Text('支付宝扫码支付'),
          content: Column(mainAxisSize: MainAxisSize.min, children: <Widget>[
            QrImageView(data: qrCode, size: 240),
            const SizedBox(height: 8),
            const Text('请使用支付宝扫码付款。付款后点击“我已支付”查询订单。'),
          ]),
          actions: <Widget>[
            TextButton(onPressed: () => Navigator.of(dialogContext).pop(), child: const Text('稍后支付')),
            FilledButton(onPressed: () async {
              final status = await _api.orderStatus(orderNo);
              if (!dialogContext.mounted) return;
              if (status == 'paid') { Navigator.of(dialogContext).pop(); return; }
              ScaffoldMessenger.of(dialogContext).showSnackBar(const SnackBar(content: Text('尚未收到支付确认，请稍后再查')));
            }, child: const Text('我已支付')),
          ],
        );
      });
      final status = await _api.orderStatus(orderNo);
      if (status != 'paid') return;
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

  Widget _commentsSection(ThemeData theme) {
    return FutureBuilder<CourseCommentPage>(
      future: _comments,
      builder: (BuildContext context, AsyncSnapshot<CourseCommentPage> snap) {
        if (snap.connectionState == ConnectionState.waiting) {
          return const Padding(
            padding: EdgeInsets.all(16),
            child: Center(child: CircularProgressIndicator()),
          );
        }
        final page = snap.data ??
            const CourseCommentPage(
                items: <CourseComment>[], count: 0, average: null, threshold: 10);
        return Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: <Widget>[
            Row(
              children: <Widget>[
                Text('学员评价', style: theme.textTheme.titleMedium),
                const SizedBox(width: 8),
                if (page.average != null)
                  Text('${page.average!.toStringAsFixed(1)} 分',
                      style: theme.textTheme.titleSmall
                          ?.copyWith(color: theme.colorScheme.primary)),
                const Spacer(),
                TextButton(
                  onPressed: _entitled ? _writeComment : null,
                  child: Text(_entitled ? '写评价' : '解锁后可评价'),
                ),
              ],
            ),
            if (page.average == null)
              Text('评价人数不足 ${page.threshold} 条，暂不展示均分',
                  style: theme.textTheme.bodySmall),
            if (page.items.isEmpty)
              Padding(
                padding: const EdgeInsets.symmetric(vertical: 12),
                child: Text('暂无评价', style: theme.textTheme.bodySmall),
              ),
            for (final item in page.items.take(5))
              Padding(
                padding: const EdgeInsets.only(bottom: 10),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: <Widget>[
                    Row(
                      children: <Widget>[
                        Text(item.nickname, style: theme.textTheme.bodyMedium),
                        const SizedBox(width: 8),
                        Text('${item.rating} 星',
                            style: theme.textTheme.bodySmall
                                ?.copyWith(color: Colors.orange.shade800)),
                        const Spacer(),
                        Text('已学 ${item.completedLectures}/${item.totalLectures} 讲',
                            style: theme.textTheme.bodySmall),
                      ],
                    ),
                    if (item.body != null && item.body!.isNotEmpty)
                      Text(item.body!, style: theme.textTheme.bodyMedium),
                  ],
                ),
              ),
            if (page.count > 5)
              Text('共 ${page.count} 条评价', style: theme.textTheme.bodySmall),
          ],
        );
      },
    );
  }

  Future<void> _writeComment() async {
    var rating = 5;
    final body = TextEditingController();
    final ok = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => StatefulBuilder(
        builder: (context, setDialogState) => AlertDialog(
          title: const Text('写课程评价'),
          content: Column(
            mainAxisSize: MainAxisSize.min,
            children: <Widget>[
              Wrap(
                children: <Widget>[
                  for (var i = 1; i <= 5; i++)
                    IconButton(
                      onPressed: () => setDialogState(() => rating = i),
                      icon: Icon(i <= rating ? Icons.star : Icons.star_border,
                          color: Colors.orange),
                    ),
                ],
              ),
              TextField(
                controller: body,
                maxLines: 4,
                maxLength: 500,
                decoration: const InputDecoration(
                  labelText: '评价内容（选填，20-500 字）',
                  border: OutlineInputBorder(),
                ),
              ),
            ],
          ),
          actions: <Widget>[
            TextButton(
                onPressed: () => Navigator.of(dialogContext).pop(false),
                child: const Text('取消')),
            FilledButton(
                onPressed: () => Navigator.of(dialogContext).pop(true),
                child: const Text('提交')),
          ],
        ),
      ),
    );
    final text = body.text.trim();
    body.dispose();
    if (ok != true) return;
    try {
      await _api.submitCourseComment(widget.courseCode,
          rating: rating, body: text.isEmpty ? null : text);
      if (!mounted) return;
      setState(() => _comments = _api.courseComments(widget.courseCode));
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('评价已提交')));
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('评价失败：$e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.courseCode),
        actions: <Widget>[
          IconButton(
            tooltip: _favorited ? '取消收藏' : '收藏',
            onPressed: _toggleFavorite,
            icon: Icon(_favorited ? Icons.favorite : Icons.favorite_border),
          ),
        ],
      ),
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
                  Text(_freeGranted ? '免费' : course.priceLabel,
                      style: theme.textTheme.titleLarge
                          ?.copyWith(color: theme.colorScheme.primary)),
                  const Spacer(),
                  if (_entitled || course.priceCents <= 0) ...<Widget>[
                    OutlinedButton(
                      onPressed: () =>
                          context.push('/exam/${widget.courseCode}/L1'),
                      child: const Text('测评'),
                    ),
                    const SizedBox(width: 8),
                    FilledButton(
                      onPressed: () =>
                          context.push('/learn/${widget.courseCode}/1'),
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
                builder:
                    (BuildContext context, AsyncSnapshot<List<Lecture>> ls) {
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
                        leading: CircleAvatar(
                            radius: 14, child: Text('${lecture.no}')),
                        title: Text(lecture.title),
                        trailing: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: <Widget>[
                              if (lecture.isPreview)
                                const Chip(label: Text('试学')),
                              IconButton(
                                tooltip: '本讲讨论',
                                icon: const Icon(Icons.forum_outlined),
                                onPressed: () => Navigator.of(context).push(
                                    MaterialPageRoute<void>(
                                        builder: (_) => LectureDiscussionPage(
                                            courseCode: widget.courseCode,
                                            no: lecture.no,
                                            api: _api))),
                              ),
                            ]),
                        onTap: () => context
                            .push('/learn/${widget.courseCode}/${lecture.no}'),
                      );
                    }).toList(),
                  );
                },
              ),
              const Divider(height: 32),
              _commentsSection(theme),
            ],
          );
        },
      ),
    );
  }
}
