import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/lecture_resource.dart';
import 'ai_assistant_page.dart';
import 'audio_player_page.dart';
import 'lecture_discussion_page.dart';
import 'lecture_practice_page.dart';
import 'resource_viewer_page.dart';
import 'slide_viewer_page.dart';
import 'video_player_page.dart';

/// 学习页（PRD v2 §7.4 B-4）：某讲的资源清单（视频 / 音频 / 讲义 / 课件 / 代码）。
///
/// 资源带短时签名 URL；本页**按类型分组**展示（视频 → 音频 → 讲义 → 课件 → 代码 → 其他），
/// 组内按标题排序并**按存储键去重**，避免同名/重复条目把清单搅乱。
class LearnPage extends StatefulWidget {
  const LearnPage(
      {super.key, required this.courseCode, required this.no, this.api});

  final String courseCode;
  final int no;
  final ApiClient? api;

  @override
  State<LearnPage> createState() => _LearnPageState();
}

class _LearnPageState extends State<LearnPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<List<LectureResource>> _future;

  /// 资源分组展示顺序（未列出的类型排最后）。
  static const List<String> _kindOrder = <String>[
    'video',
    'audio',
    'markdown',
    'slide',
    'code',
    'html',
    'frozen_output',
  ];

  @override
  void initState() {
    super.initState();
    _future = _loadResources();
  }

  Future<List<LectureResource>> _loadResources() async {
    final resources = await _api.lectureResources(widget.courseCode, widget.no);
    // 打开讲次只记「访问」，是否完成由学员显式标记（失败不影响阅读）。
    unawaited(_api
        .reportCourseProgress(widget.courseCode,
            lectureNo: widget.no, completed: false)
        .catchError((Object _) {}));
    return resources;
  }

  bool _completed = false;
  bool _completing = false;
  String? _completeError;

  Future<void> _markComplete() async {
    setState(() {
      _completing = true;
      _completeError = null;
    });
    try {
      await _api.reportCourseProgress(widget.courseCode,
          lectureNo: widget.no, completed: true);
      if (!mounted) return;
      setState(() {
        _completed = true;
        _completing = false;
      });
    } catch (_) {
      if (!mounted) return;
      setState(() {
        _completing = false;
        _completeError = '标记完成失败，请重试';
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text('第 ${widget.no} 讲'),
        actions: <Widget>[
          IconButton(
            tooltip: 'AI 助教',
            icon: const Icon(Icons.auto_awesome),
            onPressed: () => Navigator.of(context).push(MaterialPageRoute<void>(
                builder: (_) => AiAssistantPage(
                    courseCode: widget.courseCode,
                    lectureNo: widget.no,
                    api: _api))),
          ),
        ],
      ),
      body: FutureBuilder<List<LectureResource>>(
        future: _future,
        builder: (BuildContext context,
            AsyncSnapshot<List<LectureResource>> snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            final err = snapshot.error;
            if (err is DioException && err.response?.statusCode == 403) {
              return const Center(child: Text('请先购买 / 解锁本课程'));
            }
            return Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: <Widget>[
                  const Text('资源加载失败'),
                  TextButton(
                    onPressed: () => setState(() {
                      _future = _loadResources();
                    }),
                    child: const Text('重试'),
                  ),
                ],
              ),
            );
          }
          final rawResources = snapshot.data ?? const <LectureResource>[];
          final reviewing = rawResources.any((r) => r.contentReviewing);
          final resources = _grouped(rawResources);
          final children = <Widget>[
            if (reviewing)
              Card(
                color: Colors.orange.shade50,
                child: ListTile(
                  leading:
                      Icon(Icons.info_outline, color: Colors.orange.shade800),
                  title: const Text('课程内容核对中'),
                  subtitle: const Text('学习不受影响；新测评与发证暂停，待管理员处理完成。'),
                ),
              ),
            Card(
                child: ListTile(
              leading: Icon(
                  _completed ? Icons.check_circle : Icons.check_circle_outline),
              title: Text(_completed ? '本讲已完成' : '完成本讲'),
              subtitle: _completeError == null ? null : Text(_completeError!),
              trailing: _completing
                  ? const SizedBox(
                      width: 20,
                      height: 20,
                      child: CircularProgressIndicator(strokeWidth: 2))
                  : _completed
                      ? null
                      : const Icon(Icons.chevron_right),
              onTap: _completing || _completed ? null : _markComplete,
            )),
            Card(
                child: ListTile(
              leading: const Icon(Icons.quiz_outlined),
              title: const Text('本讲思考题'),
              subtitle: const Text('逐题作答，自动评分并给出参考答案'),
              onTap: () => Navigator.of(context).push(MaterialPageRoute<void>(
                  builder: (_) => LecturePracticePage(
                      courseCode: widget.courseCode,
                      no: widget.no,
                      api: _api))),
            )),
            Card(
                child: ListTile(
              leading: const Icon(Icons.forum_outlined),
              title: const Text('本讲讨论'),
              subtitle: const Text('公开讨论，按点赞量排序'),
              onTap: () => Navigator.of(context).push(MaterialPageRoute<void>(
                  builder: (_) => LectureDiscussionPage(
                      courseCode: widget.courseCode,
                      no: widget.no,
                      api: _api))),
            )),
          ];
          String? currentKind;
          for (final r in resources) {
            if (r.kind != currentKind) {
              currentKind = r.kind;
              children.add(_sectionHeader(context, r.kind));
            }
            children.add(_resourceTile(context, r));
          }
          return ListView(
            padding: const EdgeInsets.fromLTRB(12, 4, 12, 12),
            children: children,
          );
        },
      ),
    );
  }

  /// 去重（按 storage_key）后按「类型顺序 → 标题」稳定排序。
  List<LectureResource> _grouped(List<LectureResource> input) {
    final seen = <String>{};
    final deduped = <LectureResource>[];
    for (final r in input) {
      if (seen.add(r.storageKey)) deduped.add(r);
    }
    deduped.sort((a, b) {
      final rankCompare = _kindRank(a.kind).compareTo(_kindRank(b.kind));
      if (rankCompare != 0) return rankCompare;
      return a.title.compareTo(b.title);
    });
    return deduped;
  }

  int _kindRank(String kind) {
    final idx = _kindOrder.indexOf(kind);
    return idx >= 0 ? idx : _kindOrder.length;
  }

  Widget _sectionHeader(BuildContext context, String kind) {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.fromLTRB(4, 12, 4, 6),
      child: Row(
        children: <Widget>[
          Icon(_iconFor(kind), size: 18, color: theme.colorScheme.primary),
          const SizedBox(width: 6),
          Text(_labelFor(kind), style: theme.textTheme.titleSmall),
        ],
      ),
    );
  }

  Widget _resourceTile(BuildContext context, LectureResource r) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: Icon(_iconFor(r.kind)),
        title: Text(r.fileName),
        subtitle: Text(_labelFor(r.kind) +
            (r.sizeLabel.isEmpty ? '' : ' · ${r.sizeLabel}')),
        trailing: const Icon(Icons.chevron_right),
        onTap: () => _open(context, r),
      ),
    );
  }

  void _open(BuildContext context, LectureResource r) {
    // 视频：有 COS 预签名直链就端内播放；失败/过期时可在播放器内重取资源清单刷新 URL。
    if (r.kind == 'video' && r.url != null) {
      Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => VideoPlayerPage(
            title: r.title,
            url: _api.mediaUrl(r.url!),
            urlLoader: () => _refreshMediaUrl(r, 'video'),
          ),
        ),
      );
      return;
    }
    // 音频：有 COS 预签名直链就端内播放。
    if (r.kind == 'audio' && r.url != null) {
      Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => AudioPlayerPage(
            title: r.title,
            url: _api.mediaUrl(r.url!),
            urlLoader: () => _refreshMediaUrl(r, 'audio'),
          ),
        ),
      );
      return;
    }
    if (r.isTextKind ||
        r.isImageKind ||
        r.isPdfKind ||
        r.kind == 'frozen_output') {
      Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => ResourceViewerPage(
            resource: r,
            api: _api,
            courseCode: widget.courseCode,
            lectureNo: widget.no,
          ),
        ),
      );
      return;
    }
    // 课件：PPTX 走渲染器；PDF 走资源页提示/预览兜底（避免被 PPTX 渲染器误处理）。
    if (r.kind == 'slide' && r.isPdfKind) {
      Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => ResourceViewerPage(
            resource: r,
            api: _api,
            courseCode: widget.courseCode,
            lectureNo: widget.no,
          ),
        ),
      );
      return;
    }
    if (r.kind == 'slide') {
      Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => SlideViewerPage(
            resource: r,
            api: _api,
            courseCode: widget.courseCode,
            lectureNo: widget.no,
          ),
        ),
      );
      return;
    }
    // 媒体无直链时，仍尝试走资源阅读页（会给出明确错误）而不是静默无响应。
    if (r.kind == 'video' || r.kind == 'audio') {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('该媒体资源暂时无法播放，请稍后重试')),
      );
      return;
    }
    // 其余未知类型：尝试按文本/二进制提示打开，避免“点了没反应”。
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => ResourceViewerPage(
          resource: r,
          api: _api,
          courseCode: widget.courseCode,
          lectureNo: widget.no,
        ),
      ),
    );
  }

  Future<String> _refreshMediaUrl(LectureResource r, String kind) async {
    final fresh = await _api.refreshLectureResource(
        widget.courseCode, widget.no, r.storageKey);
    final url = fresh?.url;
    if (fresh == null || fresh.kind != kind || url == null || url.isEmpty) {
      throw StateError('未获取到可用的媒体地址，请返回重试');
    }
    return _api.mediaUrl(url);
  }

  IconData _iconFor(String kind) {
    switch (kind) {
      case 'video':
        return Icons.play_circle_outline;
      case 'audio':
        return Icons.audiotrack_outlined;
      case 'slide':
        return Icons.slideshow_outlined;
      case 'markdown':
        return Icons.description_outlined;
      case 'code':
        return Icons.code;
      case 'html':
        return Icons.web_outlined;
      case 'frozen_output':
        return Icons.insights_outlined;
      case 'image':
        return Icons.image_outlined;
      case 'pdf':
        return Icons.picture_as_pdf_outlined;
      default:
        return Icons.insert_drive_file_outlined;
    }
  }

  String _labelFor(String kind) {
    switch (kind) {
      case 'video':
        return '视频';
      case 'audio':
        return '音频';
      case 'slide':
        return '课件';
      case 'markdown':
        return '讲义';
      case 'code':
        return '代码';
      case 'html':
        return 'HTML 页';
      case 'frozen_output':
        return '输出/附件';
      case 'image':
        return '图片';
      case 'pdf':
        return 'PDF';
      default:
        return kind;
    }
  }
}
