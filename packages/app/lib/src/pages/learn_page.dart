import 'package:dio/dio.dart';
import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../models/lecture_resource.dart';
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
    _future = _api.lectureResources(widget.courseCode, widget.no);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text('第 ${widget.no} 讲')),
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
                      _future =
                          _api.lectureResources(widget.courseCode, widget.no);
                    }),
                    child: const Text('重试'),
                  ),
                ],
              ),
            );
          }
          final resources =
              _grouped(snapshot.data ?? const <LectureResource>[]);
          final children = <Widget>[
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
    // 视频：有 COS 预签名直链就端内播放。
    if (r.kind == 'video' && r.url != null) {
      Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) =>
              VideoPlayerPage(title: r.title, url: _api.mediaUrl(r.url!)),
        ),
      );
      return;
    }
    // 音频：有 COS 预签名直链就端内播放。
    if (r.kind == 'audio' && r.url != null) {
      Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) =>
              AudioPlayerPage(title: r.title, url: _api.mediaUrl(r.url!)),
        ),
      );
      return;
    }
    if (r.isTextKind) {
      Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => ResourceViewerPage(resource: r, api: _api),
        ),
      );
      return;
    }
    // 课件（PPTX）端内渲染（pptxjs 经 WebView）。
    if (r.kind == 'slide') {
      Navigator.of(context).push(
        MaterialPageRoute<void>(
          builder: (_) => SlideViewerPage(resource: r, api: _api),
        ),
      );
      return;
    }
    // 其余未知类型兜底提示。
    ScaffoldMessenger.of(context).showSnackBar(
      const SnackBar(content: Text('该资源暂不支持端内播放')),
    );
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
        return '冻结输出';
      default:
        return kind;
    }
  }
}
