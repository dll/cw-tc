import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';

import '../api/api_client.dart';
import '../models/lecture_resource.dart';
import 'html_media_unlock.dart';
import 'html_viewer_stub.dart' if (dart.library.html) 'html_viewer_web.dart';

/// 资源阅读页：文本类资源端内展示。
///
/// - markdown → 渲染 Markdown（讲义）
/// - html → iframe（Web）/ WebView（Android）渲染
/// - code → 纯文本（可复制）
///
/// 签名资源可能因停留过久失效；失败时会重取同一讲资源清单并用最新签名重试一次，
/// 避免“页面已打开但重试仍复用旧签名”导致持续失败。
class ResourceViewerPage extends StatefulWidget {
  const ResourceViewerPage({
    super.key,
    required this.resource,
    this.api,
    this.courseCode,
    this.lectureNo,
  });

  final LectureResource resource;
  final ApiClient? api;
  final String? courseCode;
  final int? lectureNo;

  @override
  State<ResourceViewerPage> createState() => _ResourceViewerPageState();
}

class _ResourceViewerPageState extends State<ResourceViewerPage> {
  late final ApiClient _api = widget.api ?? ApiClient();
  late Future<_LoadedText> _future;

  @override
  void initState() {
    super.initState();
    _future = _load();
  }

  Future<_LoadedText> _load() async {
    var current = widget.resource;
    final text = await _api.fetchResourceText(
      current,
      refresher: _canRefresh ? () => _refresh(current) : null,
    );
    // refresher 可能返回新条目；为保持 HTML 内联基址等元数据一致，成功后再刷新一次元数据。
    if (_canRefresh) {
      final fresh = await _refresh(current);
      if (fresh != null) current = fresh;
    }
    return _LoadedText(current, text);
  }

  bool get _canRefresh =>
      widget.courseCode != null && widget.lectureNo != null;

  Future<LectureResource?> _refresh(LectureResource current) =>
      _api.refreshLectureResource(
          widget.courseCode!, widget.lectureNo!, current.storageKey);

  String _errorMessage(Object error) {
    if (error is DioException) {
      final code = error.response?.statusCode ?? 0;
      if (code == 401) return '登录状态已失效，请重新登录后再试';
      if (code == 403) return '没有访问权限，或资源链接已失效，请重试';
      if (code == 404) return '资源不存在或已下架';
      if (error.type == DioExceptionType.connectionTimeout ||
          error.type == DioExceptionType.receiveTimeout ||
          error.type == DioExceptionType.sendTimeout) {
        return '网络超时，请检查网络后重试';
      }
      if (error.type == DioExceptionType.connectionError) {
        return '网络连接失败，请检查网络后重试';
      }
    }
    return '资源加载失败，请重试';
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: Text(widget.resource.fileName)),
      body: FutureBuilder<_LoadedText>(
        future: _future,
        builder: (BuildContext context, AsyncSnapshot<_LoadedText> snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }
          if (snapshot.hasError) {
            return Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: <Widget>[
                    Text(_errorMessage(snapshot.error!),
                        textAlign: TextAlign.center),
                    const SizedBox(height: 12),
                    FilledButton(
                      onPressed: () => setState(() => _future = _load()),
                      child: const Text('重试'),
                    ),
                  ],
                ),
              ),
            );
          }
          final loaded = snapshot.data;
          if (loaded == null) {
            return const Center(child: Text('资源加载失败，请重试'));
          }
          final content = loaded.text;
          final resource = loaded.resource;
          switch (resource.kind) {
            case 'markdown':
              return Markdown(data: content, selectable: true);
            case 'html':
              final html = resource.inlineBaseUrl == null
                  ? content
                  : htmlWithBase(
                      content,
                      _api.mediaUrl(resource.inlineBaseUrl!),
                    );
              return HtmlViewer(html: htmlWithMediaUnlock(html));
            default:
              return SingleChildScrollView(
                padding: const EdgeInsets.all(16),
                child: SelectableText(
                  content,
                  style: const TextStyle(fontSize: 15, height: 1.6),
                ),
              );
          }
        },
      ),
    );
  }
}

class _LoadedText {
  const _LoadedText(this.resource, this.text);
  final LectureResource resource;
  final String text;
}

/// 把受控的同讲资源地址放在原页面第一个 base 位置，供相对音频路径解析。
String htmlWithBase(String html, String baseUrl) {
  final base = '<base href="$baseUrl">';
  final head = RegExp(r'<head(?:\s[^>]*)?>', caseSensitive: false);
  final match = head.firstMatch(html);
  if (match == null) return '$base$html';
  return html.replaceRange(match.end, match.end, base);
}
